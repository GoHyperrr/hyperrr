package ctxengine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/GoHyperrr/hyperrr/pkg/workflow"
	"github.com/GoHyperrr/hyperrr/pkg/db"
	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
	"github.com/GoHyperrr/hyperrr/pkg/logger"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
	"github.com/GoHyperrr/hyperrr/pkg/utils"
	"gorm.io/gorm"
)

// Lineage represents the execution history of a workflow.
type Lineage struct {
	ID        string           `json:"id"`
	Name      string           `json:"name"`
	Version   string           `json:"version"`
	State     string           `json:"state"`
	StartedAt time.Time        `json:"started_at"`
	EndedAt   *time.Time       `json:"ended_at,omitempty"`
	Steps     []*StepLineage   `json:"steps"`
	Events    []eventbus.Event `json:"events"`
	Error     string           `json:"error,omitempty"`
}

func (l *Lineage) GetID() string         { return l.ID }
func (l *Lineage) GetName() string       { return l.Name }
func (l *Lineage) GetState() string      { return l.State }
func (l *Lineage) GetError() string      { return l.Error }
func (l *Lineage) GetStartedAt() time.Time { return l.StartedAt }
func (l *Lineage) GetEndedAt() *time.Time  { return l.EndedAt }

// StepLineage represents a single step in the lineage.

type StepLineage struct {
	ID        string     `json:"id"`
	StepID    string     `json:"step_id"`
	State     string     `json:"state"`
	StartedAt time.Time  `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
	Attempts  int        `json:"attempts"`
	Error     string     `json:"error,omitempty"`
}

// LineageModel GORM model for persisting lineages to database.
type LineageModel struct {
	ID        string    `gorm:"primaryKey"`
	Name      string
	Version   string
	State     string
	StartedAt time.Time
	EndedAt   *time.Time
	Error     string
	Steps     string    `gorm:"type:text"` // Serialized JSON of []*StepLineage
	Events    string    `gorm:"type:text"` // Serialized JSON of []eventbus.Event
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (LineageModel) TableName() string {
	return "workflow_lineages"
}

// Projector listens to events and maintains execution lineage.
type Projector struct {
	mu       sync.RWMutex
	lineages map[string]*Lineage
	bus      eventbus.EventBus
	db       *db.DB
}

// SetDB sets the database connection for the projector.
func (p *Projector) SetDB(database *db.DB) {
	p.db = database
}

// NewProjector creates a new Projector.
func NewProjector(bus eventbus.EventBus) *Projector {
	return &Projector{
		lineages: make(map[string]*Lineage),
		bus:      bus,
	}
}

// Start begins listening for workflow events.
func (p *Projector) Start(ctx context.Context) error {
	if p.bus == nil {
		return nil
	}
	eventTypes := []string{
		workflow.EventWorkflowStarted,
		workflow.EventStepStarted,
		workflow.EventStepCompleted,
		workflow.EventStepFailed,
		workflow.EventStepRetrying,
		workflow.EventStepFallback,
		workflow.EventWaitingHuman,
		workflow.EventWorkflowCompleted,
		workflow.EventWorkflowFailed,
		"order.created",
		"order.paid",
	}

	for _, t := range eventTypes {
		_, err := p.bus.Subscribe(ctx, t, p.handleEvent)
		if err != nil {
			return fmt.Errorf("failed to subscribe to %s: %w", t, err)
		}
	}

	return nil
}

func (p *Projector) handleEvent(ctx context.Context, event eventbus.Event) error {
	p.mu.Lock()

	rawPayload, ok := event.Payload.(map[string]any)
	if !ok {
		p.mu.Unlock()
		return nil
	}

	id := utils.GetString(rawPayload, "id")
	if id == "" {
		p.mu.Unlock()
		return nil
	}

	lineage, exists := p.lineages[id]
	if !exists {
		lineage = &Lineage{
			ID:    id,
			Steps: make([]*StepLineage, 0),
		}
		p.lineages[id] = lineage
	}

	lineage.Events = append(lineage.Events, event)

	switch event.Type {
	case workflow.EventWorkflowStarted:
		lineage.Name = utils.GetString(rawPayload, "name")
		lineage.Version = utils.GetString(rawPayload, "version")
		lineage.State = workflow.StateRunning
		if lineage.StartedAt.IsZero() {
			lineage.StartedAt = event.Timestamp
		}

	case workflow.EventStepStarted:
		stepID := utils.GetString(rawPayload, "step_id")
		if p.findStep(lineage, stepID) == nil {
			lineage.Steps = append(lineage.Steps, &StepLineage{
				StepID:    stepID,
				State:     workflow.StateRunning,
				StartedAt: event.Timestamp,
				Attempts:  1,
			})
		}

	case workflow.EventStepCompleted:
		stepID := utils.GetString(rawPayload, "step_id")
		if step := p.findStep(lineage, stepID); step != nil {
			step.State = workflow.StateCompleted
			step.EndedAt = &event.Timestamp
		}

	case workflow.EventStepFailed:
		stepID := utils.GetString(rawPayload, "step_id")
		errMsg := utils.GetString(rawPayload, "error")
		if step := p.findStep(lineage, stepID); step != nil {
			step.State = workflow.StateFailed
			step.Error = errMsg
			step.EndedAt = &event.Timestamp
		}

	case workflow.EventStepRetrying:
		stepID := utils.GetString(rawPayload, "step_id")
		if step := p.findStep(lineage, stepID); step != nil {
			step.State = workflow.StateRetrying
			step.Attempts++
		}

	case workflow.EventWaitingHuman:
		lineage.State = workflow.StateWaitingHuman

	case workflow.EventWorkflowCompleted:
		lineage.State = workflow.StateCompleted
		lineage.EndedAt = &event.Timestamp

	case workflow.EventWorkflowFailed:
		lineage.State = workflow.StateFailed
		lineage.Error = utils.GetString(rawPayload, "error")
		lineage.EndedAt = &event.Timestamp
	}

	// Deep copy for storage to avoid holding lock during DB I/O and prevent data races
	saveLineage := &Lineage{
		ID:        lineage.ID,
		Name:      lineage.Name,
		Version:   lineage.Version,
		State:     lineage.State,
		StartedAt: lineage.StartedAt,
		EndedAt:   lineage.EndedAt,
		Error:     lineage.Error,
	}

	saveLineage.Steps = make([]*StepLineage, len(lineage.Steps))
	for i, s := range lineage.Steps {
		stepCopy := *s
		saveLineage.Steps[i] = &stepCopy
	}

	saveLineage.Events = make([]eventbus.Event, len(lineage.Events))
	copy(saveLineage.Events, lineage.Events)
	
	p.mu.Unlock()

	// Only save to DB if it is a terminal/pause state (Completed, Failed, Waiting Human)
	if saveLineage.State == workflow.StateCompleted || saveLineage.State == workflow.StateFailed || saveLineage.State == workflow.StateWaitingHuman {
		p.saveToDB(ctx, saveLineage)
	}

	return nil
}

func (p *Projector) saveToDB(ctx context.Context, l *Lineage) {
	if p.db == nil {
		return
	}
	stepsJSON, err := json.Marshal(l.Steps)
	if err != nil {
		logger.Error("failed to marshal lineage steps", "error", err)
		return
	}
	eventsJSON, err := json.Marshal(l.Events)
	if err != nil {
		logger.Error("failed to marshal lineage events", "error", err)
		return
	}

	model := &LineageModel{
		ID:        l.ID,
		Name:      l.Name,
		Version:   l.Version,
		State:     l.State,
		StartedAt: l.StartedAt,
		EndedAt:   l.EndedAt,
		Error:     l.Error,
		Steps:     string(stepsJSON),
		Events:    string(eventsJSON),
	}

	if err := p.db.WithContext(ctx).Save(model).Error; err != nil {
		logger.Error("failed to save lineage to database", "id", l.ID, "error", err)
	}
}

func (p *Projector) findStep(l *Lineage, stepID string) *StepLineage {
	for i := len(l.Steps) - 1; i >= 0; i-- {
		if l.Steps[i].StepID == stepID {
			return l.Steps[i]
		}
	}
	return nil
}

// GetLineage returns the lineage for a workflow ID.
func (p *Projector) GetLineage(id string) (*Lineage, error) {
	p.mu.RLock()
	lineage, ok := p.lineages[id]
	p.mu.RUnlock()

	if ok {
		return lineage, nil
	}

	// Fallback to database
	if p.db != nil {
		var model LineageModel
		err := p.db.First(&model, "id = ?", id).Error
		if err == nil {
			var steps []*StepLineage
			_ = json.Unmarshal([]byte(model.Steps), &steps)

			var events []eventbus.Event
			_ = json.Unmarshal([]byte(model.Events), &events)

			l := &Lineage{
				ID:        model.ID,
				Name:      model.Name,
				Version:   model.Version,
				State:     model.State,
				StartedAt: model.StartedAt,
				EndedAt:   model.EndedAt,
				Error:     model.Error,
				Steps:     steps,
				Events:    events,
			}

			// Cache in-memory
			p.mu.Lock()
			p.lineages[id] = l
			p.mu.Unlock()

			return l, nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("failed to query database for lineage %s: %w", id, err)
		}
	}

	return nil, fmt.Errorf("lineage not found for workflow: %s", id)
}

// ListLineages returns all lineages as registry.LineageData.
func (p *Projector) ListLineages() []registry.LineageData {
	p.mu.RLock()
	inMem := make(map[string]*Lineage)
	for k, v := range p.lineages {
		inMem[k] = v
	}
	p.mu.RUnlock()

	if p.db != nil {
		var models []LineageModel
		if err := p.db.Find(&models).Error; err != nil {
			logger.Error("failed to query lineages from database", "error", err)
		} else {
			for _, model := range models {
				if _, exists := inMem[model.ID]; !exists {
					var steps []*StepLineage
					_ = json.Unmarshal([]byte(model.Steps), &steps)
					var events []eventbus.Event
					_ = json.Unmarshal([]byte(model.Events), &events)
					inMem[model.ID] = &Lineage{
						ID:        model.ID,
						Name:      model.Name,
						Version:   model.Version,
						State:     model.State,
						StartedAt: model.StartedAt,
						EndedAt:   model.EndedAt,
						Error:     model.Error,
						Steps:     steps,
						Events:    events,
					}
				}
			}
		}
	}

	res := make([]registry.LineageData, 0, len(inMem))
	for _, l := range inMem {
		res = append(res, l)
	}
	return res
}

// QueryLineages returns lineages that match the given filter.
func (p *Projector) QueryLineages(filter func(registry.LineageData) bool) []registry.LineageData {
	all := p.ListLineages()
	var res []registry.LineageData
	for _, l := range all {
		if filter(l) {
			res = append(res, l)
		}
	}
	return res
}

// GetRelatedLineages returns all lineages that share metadata with the given workflow ID.
func (p *Projector) GetRelatedLineages(ctx context.Context, id string) ([]*Lineage, error) {
	lineage, err := p.GetLineage(id)
	if err != nil {
		return nil, err
	}

	relatedIDsMap := make(map[string]bool)
	allLineages := p.ListLineages()
	allLineagesMap := make(map[string]*Lineage)
	for _, data := range allLineages {
		if l, ok := data.(*Lineage); ok {
			allLineagesMap[l.ID] = l
		}
	}

	for _, other := range allLineagesMap {
		if other.ID == id {
			continue
		}
		
		for _, event := range lineage.Events {
			for key, val := range event.Metadata {
				for _, otherEvent := range other.Events {
					if otherVal, ok := otherEvent.Metadata[key]; ok && otherVal == val {
						relatedIDsMap[other.ID] = true
					}
				}
			}
		}
	}

	res := make([]*Lineage, 0, len(relatedIDsMap))
	for relatedID := range relatedIDsMap {
		res = append(res, allLineagesMap[relatedID])
	}

	return res, nil
}
