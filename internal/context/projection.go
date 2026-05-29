package context

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/GoHyperrr/hyperrr/internal/workflow"
	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
	"github.com/GoHyperrr/hyperrr/pkg/utils"
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

// Projector listens to events and maintains execution lineage.
type Projector struct {
	mu       sync.RWMutex
	lineages map[string]*Lineage
	bus      eventbus.EventBus
	store    *LineageStore
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

	if p.store != nil {
		p.store.Save(ctx, saveLineage)
		// Save correlation index to database
		for key, val := range event.Metadata {
			p.store.SaveCorrelation(ctx, key, val, id)
		}
	}

	return nil
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
	defer p.mu.RUnlock()

	lineage, ok := p.lineages[id]
	if !ok {
		return nil, fmt.Errorf("lineage not found for workflow: %s", id)
	}
	return lineage, nil
}

// ListLineages returns all lineages as registry.LineageData.
func (p *Projector) ListLineages() []registry.LineageData {
	p.mu.RLock()
	defer p.mu.RUnlock()

	res := make([]registry.LineageData, 0, len(p.lineages))
	for _, l := range p.lineages {
		res = append(res, l)
	}
	return res
}

// QueryLineages returns lineages that match the given filter.
func (p *Projector) QueryLineages(filter func(registry.LineageData) bool) []registry.LineageData {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var res []registry.LineageData
	for _, l := range p.lineages {
		if filter(l) {
			res = append(res, l)
		}
	}
	return res
}

// GetRelatedLineages returns all lineages that share metadata with the given workflow ID.
func (p *Projector) GetRelatedLineages(ctx context.Context, id string) ([]*Lineage, error) {
	p.mu.RLock()
	lineage, ok := p.lineages[id]
	p.mu.RUnlock()

	if !ok {
		// Try to load from store
		if p.store != nil {
			lineage, _ = p.store.Get(ctx, id)
		}
	}

	if lineage == nil {
		return nil, fmt.Errorf("lineage not found for workflow: %s", id)
	}

	relatedIDsMap := make(map[string]bool)

	// 1. Deduplicate metadata to avoid redundant DB queries
	uniqueMetadata := make(map[string]map[string]bool)
	for _, event := range lineage.Events {
		for key, val := range event.Metadata {
			if uniqueMetadata[key] == nil {
				uniqueMetadata[key] = make(map[string]bool)
			}
			uniqueMetadata[key][val] = true
		}
	}

	// 2. Query database index for relationships
	if p.store != nil {
		for key, values := range uniqueMetadata {
			for val := range values {
				dbIDs, err := p.store.ListRelatedIDs(ctx, key, val)
				if err == nil {
					for _, dbID := range dbIDs {
						if dbID != id {
							relatedIDsMap[dbID] = true
						}
					}
				}
			}
		}
	}

	res := make([]*Lineage, 0, len(relatedIDsMap))
	for relatedID := range relatedIDsMap {
		l, err := p.GetLineage(relatedID)
		if err == nil {
			res = append(res, l)
		} else if p.store != nil {
			// Fallback to store if not in memory
			l, err = p.store.Get(ctx, relatedID)
			if err == nil {
				res = append(res, l)
			}
		}
	}

	return res, nil
}
