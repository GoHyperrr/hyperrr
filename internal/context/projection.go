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
	mu          sync.RWMutex
	lineages    map[string]*Lineage
	correlation map[string]map[string][]string // key -> value -> []workflowID
	bus         eventbus.EventBus
	store       *LineageStore
}

// NewProjector creates a new Projector.
func NewProjector(bus eventbus.EventBus) *Projector {
	return &Projector{
		lineages:    make(map[string]*Lineage),
		correlation: make(map[string]map[string][]string),
		bus:         bus,
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
		err := p.bus.Subscribe(ctx, t, p.handleEvent)
		if err != nil {
			return fmt.Errorf("failed to subscribe to %s: %w", t, err)
		}
	}

	return nil
}

func (p *Projector) handleEvent(ctx context.Context, event eventbus.Event) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	rawPayload, ok := event.Payload.(map[string]any)
	if !ok {
		return nil
	}

	id := utils.GetString(rawPayload, "id")
	if id == "" {
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

	// Index correlation metadata
	for key, val := range event.Metadata {
		if p.correlation[key] == nil {
			p.correlation[key] = make(map[string][]string)
		}
		
		existsInCorrelation := false
		for _, existingID := range p.correlation[key][val] {
			if existingID == id {
				existsInCorrelation = true
				break
			}
		}
		if !existsInCorrelation {
			p.correlation[key][val] = append(p.correlation[key][val], id)
		}
	}

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

	if p.store != nil {
		p.store.Save(ctx, lineage)
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
func (p *Projector) GetRelatedLineages(id string) ([]*Lineage, error) {
	p.mu.RLock()
	lineage, ok := p.lineages[id]
	p.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("lineage not found: %s", id)
	}

	relatedIDs := make(map[string]bool)
	p.mu.RLock()
	defer p.mu.RUnlock()

	for _, event := range lineage.Events {
		for key, val := range event.Metadata {
			if bucket, ok := p.correlation[key]; ok {
				if ids, ok := bucket[val]; ok {
					for _, relatedID := range ids {
						if relatedID != id {
							relatedIDs[relatedID] = true
						}
					}
				}
			}
		}
	}

	res := make([]*Lineage, 0, len(relatedIDs))
	for relatedID := range relatedIDs {
		res = append(res, p.lineages[relatedID])
	}

	return res, nil
}
