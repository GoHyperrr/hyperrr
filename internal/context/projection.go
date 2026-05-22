package context

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
)

// Lineage represents the execution history of a workflow.
type Lineage struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Version   string         `json:"version"`
	State     string         `json:"state"`
	StartedAt time.Time      `json:"started_at"`
	EndedAt   *time.Time     `json:"ended_at,omitempty"`
	Steps     []*StepLineage `json:"steps"`
	Events    []eventbus.Event `json:"events"`
	Error     string         `json:"error,omitempty"`
}

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
		"workflow.started",
		"workflow.step.started",
		"workflow.step.completed",
		"workflow.step.failed",
		"workflow.step.retrying",
		"workflow.step.fallback",
		"workflow.waiting_human",
		"workflow.completed",
		"workflow.failed",
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

	payload, ok := event.Payload.(map[string]any)
	if !ok {
		return nil
	}

	id, ok := payload["id"].(string)
	if !ok {
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
	case "workflow.started":
		payload := event.Payload.(map[string]any)
		name, _ := payload["name"].(string)
		version, _ := payload["version"].(string)
		lineage.Name = name
		lineage.Version = version
		lineage.State = "RUNNING"
		lineage.StartedAt = event.Timestamp

	case "workflow.step.started":
		stepID := event.Payload.(map[string]any)["step_id"].(string)
		lineage.Steps = append(lineage.Steps, &StepLineage{
			StepID:    stepID,
			State:     "RUNNING",
			StartedAt: event.Timestamp,
			Attempts:  1,
		})

	case "workflow.step.completed":
		stepID := event.Payload.(map[string]any)["step_id"].(string)
		if step := p.findStep(lineage, stepID); step != nil {
			step.State = "COMPLETED"
			step.EndedAt = &event.Timestamp
		}

	case "workflow.step.failed":
		payload := event.Payload.(map[string]any)
		stepID := payload["step_id"].(string)
		errMsg := payload["error"].(string)
		if step := p.findStep(lineage, stepID); step != nil {
			step.State = "FAILED"
			step.Error = errMsg
			step.EndedAt = &event.Timestamp
		}

	case "workflow.step.retrying":
		stepID := event.Payload.(map[string]any)["step_id"].(string)
		if step := p.findStep(lineage, stepID); step != nil {
			step.State = "RETRYING"
			step.Attempts++
		}

	case "workflow.waiting_human":
		lineage.State = "WAITING_HUMAN"

	case "workflow.completed":
		lineage.State = "COMPLETED"
		lineage.EndedAt = &event.Timestamp

	case "workflow.failed":
		payload := event.Payload.(map[string]any)
		lineage.State = "FAILED"
		lineage.Error = payload["error"].(string)
		lineage.EndedAt = &event.Timestamp
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

// ListLineages returns all lineages.
func (p *Projector) ListLineages() []*Lineage {
	p.mu.RLock()
	defer p.mu.RUnlock()

	res := make([]*Lineage, 0, len(p.lineages))
	for _, l := range p.lineages {
		res = append(res, l)
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
