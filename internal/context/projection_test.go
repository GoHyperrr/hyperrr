package context

import (
	"context"
	"testing"
	"time"

	"github.com/GoHyperrr/hyperrr/internal/workflow"
	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
)

func TestProjector(t *testing.T) {
	bus := eventbus.NewInMemBus()
	projector := NewProjector(bus)
	
	ctx := context.Background()

	err := projector.Start(ctx)
	if err != nil {
		t.Fatalf("failed to start projector: %v", err)
	}

	workflowID := "wf123"

	t.Run("Full Success Path", func(t *testing.T) {
		// Simulate workflow.started
		bus.Publish(ctx, eventbus.Event{
			Type:      workflow.EventWorkflowStarted,
			Timestamp: time.Now(),
			Payload: map[string]any{
				"id":      workflowID,
				"name":    "test-workflow",
				"version": "v1",
			},
		})

		// Simulate step 1
		bus.Publish(ctx, eventbus.Event{
			Type:      workflow.EventStepStarted,
			Timestamp: time.Now(),
			Payload: map[string]any{
				"id":      workflowID,
				"step_id": "step1",
			},
		})
		bus.Publish(ctx, eventbus.Event{
			Type:      workflow.EventStepCompleted,
			Timestamp: time.Now(),
			Payload: map[string]any{
				"id":      workflowID,
				"step_id": "step1",
			},
		})

		// Simulate workflow.completed
		bus.Publish(ctx, eventbus.Event{
			Type:      workflow.EventWorkflowCompleted,
			Timestamp: time.Now(),
			Payload: map[string]any{
				"id": workflowID,
			},
		})

		time.Sleep(50 * time.Millisecond) // Wait for async projection
		lineage, err := projector.GetLineage(workflowID)
		if err != nil {
			t.Fatalf("failed to get lineage: %v", err)
		}

		if lineage.Name != "test-workflow" {
			t.Errorf("expected test-workflow, got %s", lineage.Name)
		}
		if lineage.State != workflow.StateCompleted {
			t.Errorf("expected %s state, got %s", workflow.StateCompleted, lineage.State)
		}
		if len(lineage.Steps) != 1 {
			t.Errorf("expected 1 step, got %d", len(lineage.Steps))
		}
		if lineage.Steps[0].State != workflow.StateCompleted {
			t.Errorf("expected step %s, got %s", workflow.StateCompleted, lineage.Steps[0].State)
		}
	})

	t.Run("Failure and Retry Path", func(t *testing.T) {
		wfID := "wf_fail"
		bus.Publish(ctx, eventbus.Event{
			Type:      workflow.EventWorkflowStarted,
			Timestamp: time.Now(),
			Payload:   map[string]any{"id": wfID, "name": "fail", "version": "v1"},
		})
		bus.Publish(ctx, eventbus.Event{
			Type:      workflow.EventStepStarted,
			Timestamp: time.Now(),
			Payload:   map[string]any{"id": wfID, "step_id": "s1"},
		})
		bus.Publish(ctx, eventbus.Event{
			Type:      workflow.EventStepRetrying,
			Timestamp: time.Now(),
			Payload:   map[string]any{"id": wfID, "step_id": "s1"},
		})
		bus.Publish(ctx, eventbus.Event{
			Type:      workflow.EventStepFailed,
			Timestamp: time.Now(),
			Payload:   map[string]any{"id": wfID, "step_id": "s1", "error": "boom"},
		})
		bus.Publish(ctx, eventbus.Event{
			Type:      workflow.EventWorkflowFailed,
			Timestamp: time.Now(),
			Payload:   map[string]any{"id": wfID, "error": "workflow failed"},
		})

		time.Sleep(50 * time.Millisecond)
		l, _ := projector.GetLineage(wfID)
		if l.State != workflow.StateFailed || l.Error != "workflow failed" {
			t.Errorf("unexpected state: %s, error: %s", l.State, l.Error)
		}
		if l.Steps[0].Attempts != 2 {
			t.Errorf("expected 2 attempts, got %d", l.Steps[0].Attempts)
		}
	})

	t.Run("Waiting Human", func(t *testing.T) {
		wfID := "wf_human"
		bus.Publish(ctx, eventbus.Event{
			Type:    workflow.EventWaitingHuman,
			Payload: map[string]any{"id": wfID},
		})
		time.Sleep(50 * time.Millisecond)
		l, _ := projector.GetLineage(wfID)
		if l.State != workflow.StateWaitingHuman {
			t.Errorf("expected %s, got %s", workflow.StateWaitingHuman, l.State)
		}
	})

	t.Run("Lineage Not Found", func(t *testing.T) {
		_, err := projector.GetLineage("ghost")
		if err == nil {
			t.Error("expected error for non-existent lineage")
		}

		_, err = projector.GetRelatedLineages(ctx, "ghost")
		if err == nil {
			t.Error("expected error for non-existent lineage in GetRelatedLineages")
		}
	})

	t.Run("ListLineages", func(t *testing.T) {
		res := projector.ListLineages()
		if len(res) < 3 {
			t.Errorf("expected at least 3 lineages, got %d", len(res))
		}
	})

	t.Run("Comprehensive handleEvent", func(t *testing.T) {
		wfID := "wf_comp"
		
		bus.Publish(ctx, eventbus.Event{
			Type: workflow.EventWorkflowStarted,
			Payload: map[string]any{"id": wfID, "name": "comp-wf", "version": "v2"},
			Timestamp: time.Now(),
		})
		bus.Publish(ctx, eventbus.Event{
			Type: workflow.EventStepStarted,
			Payload: map[string]any{"id": wfID, "step_id": "step1"},
			Timestamp: time.Now(),
		})
		bus.Publish(ctx, eventbus.Event{
			Type: workflow.EventStepRetrying,
			Payload: map[string]any{"id": wfID, "step_id": "step1"},
			Timestamp: time.Now(),
		})
		bus.Publish(ctx, eventbus.Event{
			Type: workflow.EventStepCompleted,
			Payload: map[string]any{"id": wfID, "step_id": "step1"},
			Timestamp: time.Now(),
		})
		bus.Publish(ctx, eventbus.Event{
			Type: "order.created",
			Payload: map[string]any{"id": wfID, "order_id": "ord123"},
			Timestamp: time.Now(),
		})
		bus.Publish(ctx, eventbus.Event{
			Type: workflow.EventStepStarted,
			Payload: map[string]any{"id": wfID, "step_id": "step2"},
			Timestamp: time.Now(),
		})
		bus.Publish(ctx, eventbus.Event{
			Type: workflow.EventStepFailed,
			Payload: map[string]any{"id": wfID, "step_id": "step2", "error": "step fail"},
			Timestamp: time.Now(),
		})
		bus.Publish(ctx, eventbus.Event{
			Type: workflow.EventWaitingHuman,
			Payload: map[string]any{"id": wfID},
			Timestamp: time.Now(),
		})
		bus.Publish(ctx, eventbus.Event{
			Type: "order.paid",
			Payload: map[string]any{"id": wfID, "order_id": "ord123"},
			Timestamp: time.Now(),
		})
		bus.Publish(ctx, eventbus.Event{
			Type: workflow.EventWorkflowFailed,
			Payload: map[string]any{"id": wfID, "error": "final fail"},
			Timestamp: time.Now(),
		})

		time.Sleep(100 * time.Millisecond)
		l, _ := projector.GetLineage(wfID)
		if l.State != workflow.StateFailed {
			t.Errorf("expected FAILED, got %s", l.State)
		}
		
		foundCreated := false
		foundPaid := false
		for _, e := range l.Events {
			if e.Type == "order.created" { foundCreated = true }
			if e.Type == "order.paid" { foundPaid = true }
		}
		if !foundCreated || !foundPaid {
			t.Error("missing order events in lineage")
		}
	})

	t.Run("QueryLineages", func(t *testing.T) {
		res := projector.QueryLineages(func(ld registry.LineageData) bool {
			return ld.GetName() == "comp-wf"
		})
		if len(res) != 1 {
			t.Errorf("expected 1 lineage for comp-wf, got %d", len(res))
		}
	})

	t.Run("Complex RelatedLineages", func(t *testing.T) {
		bus.Publish(ctx, eventbus.Event{
			Type: workflow.EventWorkflowStarted,
			Payload: map[string]any{"id": "rel_a", "name": "a"},
			Metadata: map[string]string{"customer_id": "cust_shared"},
		})
		bus.Publish(ctx, eventbus.Event{
			Type: workflow.EventWorkflowStarted,
			Payload: map[string]any{"id": "rel_b", "name": "b"},
			Metadata: map[string]string{"customer_id": "cust_shared"},
		})
		bus.Publish(ctx, eventbus.Event{
			Type: workflow.EventWorkflowStarted,
			Payload: map[string]any{"id": "rel_c", "name": "c"},
			Metadata: map[string]string{"customer_id": "cust_shared"},
		})

		time.Sleep(100 * time.Millisecond)
		rel, err := projector.GetRelatedLineages(ctx, "rel_a")
		if err != nil { t.Fatalf("failed: %v", err) }
		if len(rel) != 2 {
			t.Errorf("expected 2 related lineages, got %d", len(rel))
		}
	})

	t.Run("findStep multiple steps", func(t *testing.T) {
		l := &Lineage{
			Steps: []*StepLineage{
				{StepID: "s1", State: workflow.StateCompleted},
				{StepID: "s2", State: workflow.StateRunning},
				{StepID: "s1", State: workflow.StateRetrying}, // Newer s1
			},
		}
		p := &Projector{}
		step := p.findStep(l, "s1")
		if step == nil || step.State != workflow.StateRetrying {
			t.Errorf("expected latest s1 step with %s state, got %v", workflow.StateRetrying, step)
		}
	})

	t.Run("Start with nil bus", func(t *testing.T) {
		p := &Projector{}
		err := p.Start(context.Background())
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("Coverage Edge Cases", func(t *testing.T) {
		p := NewProjector(nil)

		// 1. handleEvent with payload that is NOT map[string]any
		err := p.handleEvent(ctx, eventbus.Event{
			Type:    "test",
			Payload: "not-a-map",
		})
		if err != nil {
			t.Errorf("expected no error for invalid payload type, got %v", err)
		}

		// 2. handleEvent with empty id
		err = p.handleEvent(ctx, eventbus.Event{
			Type:    "test",
			Payload: map[string]any{"not-id": "val"},
		})
		if err != nil {
			t.Errorf("expected no error for missing id, got %v", err)
		}

		// 3. GetRelatedLineages with no events/correlation
		projector.mu.Lock()
		projector.lineages["no-events"] = &Lineage{ID: "no-events"}
		projector.mu.Unlock()

		related, err := projector.GetRelatedLineages(ctx, "no-events")
		if err != nil {
			t.Errorf("GetRelatedLineages failed: %v", err)
		}
		if len(related) != 0 {
			t.Errorf("expected 0 related lineages, got %d", len(related))
		}
	})

	t.Run("Exhaustive Event Types", func(t *testing.T) {
		p := NewProjector(nil)
		wfID := "wf_exhaustive"

		eventTypes := []string{
			workflow.EventWorkflowStarted,
			workflow.EventStepStarted,
			workflow.EventStepCompleted,
			workflow.EventStepFailed,
			workflow.EventStepRetrying,
			workflow.EventWaitingHuman,
			workflow.EventWorkflowCompleted,
			workflow.EventWorkflowFailed,
			"order.created",
			"order.paid",
		}

		for _, et := range eventTypes {
			payload := map[string]any{"id": wfID, "step_id": "s1", "name": "test", "version": "v1", "error": "some error"}
			err := p.handleEvent(ctx, eventbus.Event{
				Type:      et,
				Payload:   payload,
				Timestamp: time.Now(),
				Metadata:  map[string]string{"m1": "v1"},
			})
			if err != nil {
				t.Errorf("failed to handle event %s: %v", et, err)
			}
		}

		l, _ := p.GetLineage(wfID)
		if len(l.Events) != len(eventTypes) {
			t.Errorf("expected %d events, got %d", len(eventTypes), len(l.Events))
		}
	})
}
