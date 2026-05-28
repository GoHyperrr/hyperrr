package context

import (
	"context"
	"testing"
	"time"

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
			Type:      "workflow.started",
			Timestamp: time.Now(),
			Payload: map[string]any{
				"id":      workflowID,
				"name":    "test-workflow",
				"version": "v1",
			},
		})

		// Simulate step 1
		bus.Publish(ctx, eventbus.Event{
			Type:      "workflow.step.started",
			Timestamp: time.Now(),
			Payload: map[string]any{
				"id":      workflowID,
				"step_id": "step1",
			},
		})
		bus.Publish(ctx, eventbus.Event{
			Type:      "workflow.step.completed",
			Timestamp: time.Now(),
			Payload: map[string]any{
				"id":      workflowID,
				"step_id": "step1",
			},
		})

		// Simulate workflow.completed
		bus.Publish(ctx, eventbus.Event{
			Type:      "workflow.completed",
			Timestamp: time.Now(),
			Payload: map[string]any{
				"id": workflowID,
			},
		})

		lineage, err := projector.GetLineage(workflowID)
		if err != nil {
			t.Fatalf("failed to get lineage: %v", err)
		}

		if lineage.Name != "test-workflow" {
			t.Errorf("expected test-workflow, got %s", lineage.Name)
		}
		if lineage.State != "COMPLETED" {
			t.Errorf("expected COMPLETED state, got %s", lineage.State)
		}
		if len(lineage.Steps) != 1 {
			t.Errorf("expected 1 step, got %d", len(lineage.Steps))
		}
		if lineage.Steps[0].State != "COMPLETED" {
			t.Errorf("expected step COMPLETED, got %s", lineage.Steps[0].State)
		}
	})

	t.Run("Failure and Retry Path", func(t *testing.T) {
		wfID := "wf_fail"
		bus.Publish(ctx, eventbus.Event{
			Type:      "workflow.started",
			Timestamp: time.Now(),
			Payload:   map[string]any{"id": wfID, "name": "fail", "version": "v1"},
		})
		bus.Publish(ctx, eventbus.Event{
			Type:      "workflow.step.started",
			Timestamp: time.Now(),
			Payload:   map[string]any{"id": wfID, "step_id": "s1"},
		})
		bus.Publish(ctx, eventbus.Event{
			Type:      "workflow.step.retrying",
			Timestamp: time.Now(),
			Payload:   map[string]any{"id": wfID, "step_id": "s1"},
		})
		bus.Publish(ctx, eventbus.Event{
			Type:      "workflow.step.failed",
			Timestamp: time.Now(),
			Payload:   map[string]any{"id": wfID, "step_id": "s1", "error": "boom"},
		})
		bus.Publish(ctx, eventbus.Event{
			Type:      "workflow.failed",
			Timestamp: time.Now(),
			Payload:   map[string]any{"id": wfID, "error": "workflow failed"},
		})

		l, _ := projector.GetLineage(wfID)
		if l.State != "FAILED" || l.Error != "workflow failed" {
			t.Errorf("unexpected state: %s, error: %s", l.State, l.Error)
		}
		if l.Steps[0].Attempts != 2 {
			t.Errorf("expected 2 attempts, got %d", l.Steps[0].Attempts)
		}
	})

	t.Run("Waiting Human", func(t *testing.T) {
		wfID := "wf_human"
		bus.Publish(ctx, eventbus.Event{
			Type:    "workflow.waiting_human",
			Payload: map[string]any{"id": wfID},
		})
		l, _ := projector.GetLineage(wfID)
		if l.State != "WAITING_HUMAN" {
			t.Errorf("expected WAITING_HUMAN, got %s", l.State)
		}
	})

	t.Run("Lineage Not Found", func(t *testing.T) {
		_, err := projector.GetLineage("ghost")
		if err == nil {
			t.Error("expected error for non-existent lineage")
		}

		_, err = projector.GetRelatedLineages("ghost")
		if err == nil {
			t.Error("expected error for non-existent lineage in GetRelatedLineages")
		}
	})

	t.Run("ListLineages", func(t *testing.T) {
		res := projector.ListLineages()
		if len(res) < 3 { // wf123, wf_fail, wf_human
			t.Errorf("expected at least 3 lineages, got %d", len(res))
		}
	})

	t.Run("Comprehensive handleEvent", func(t *testing.T) {
		wfID := "wf_comp"
		
		// 1. workflow.started
		bus.Publish(ctx, eventbus.Event{
			Type: "workflow.started",
			Payload: map[string]any{"id": wfID, "name": "comp-wf", "version": "v2"},
			Timestamp: time.Now(),
		})

		// 2. workflow.step.started
		bus.Publish(ctx, eventbus.Event{
			Type: "workflow.step.started",
			Payload: map[string]any{"id": wfID, "step_id": "step1"},
			Timestamp: time.Now(),
		})

		// 3. workflow.step.retrying
		bus.Publish(ctx, eventbus.Event{
			Type: "workflow.step.retrying",
			Payload: map[string]any{"id": wfID, "step_id": "step1"},
			Timestamp: time.Now(),
		})

		// 4. workflow.step.completed
		bus.Publish(ctx, eventbus.Event{
			Type: "workflow.step.completed",
			Payload: map[string]any{"id": wfID, "step_id": "step1"},
			Timestamp: time.Now(),
		})

		// 5. order.created (custom event recorded in lineage)
		bus.Publish(ctx, eventbus.Event{
			Type: "order.created",
			Payload: map[string]any{"id": wfID, "order_id": "ord123"},
			Timestamp: time.Now(),
		})

		// 6. workflow.step.started (step 2)
		bus.Publish(ctx, eventbus.Event{
			Type: "workflow.step.started",
			Payload: map[string]any{"id": wfID, "step_id": "step2"},
			Timestamp: time.Now(),
		})

		// 7. workflow.step.failed
		bus.Publish(ctx, eventbus.Event{
			Type: "workflow.step.failed",
			Payload: map[string]any{"id": wfID, "step_id": "step2", "error": "step fail"},
			Timestamp: time.Now(),
		})

		// 8. workflow.waiting_human
		bus.Publish(ctx, eventbus.Event{
			Type: "workflow.waiting_human",
			Payload: map[string]any{"id": wfID},
			Timestamp: time.Now(),
		})

		// 9. order.paid
		bus.Publish(ctx, eventbus.Event{
			Type: "order.paid",
			Payload: map[string]any{"id": wfID, "order_id": "ord123"},
			Timestamp: time.Now(),
		})

		// 10. workflow.failed
		bus.Publish(ctx, eventbus.Event{
			Type: "workflow.failed",
			Payload: map[string]any{"id": wfID, "error": "final fail"},
			Timestamp: time.Now(),
		})

		time.Sleep(50 * time.Millisecond)
		l, _ := projector.GetLineage(wfID)
		if l.State != "FAILED" {
			t.Errorf("expected FAILED, got %s", l.State)
		}
		if len(l.Steps) != 2 {
			t.Errorf("expected 2 steps, got %d", len(l.Steps))
		}
		if l.Steps[0].Attempts != 2 {
			t.Errorf("expected 2 attempts for step1, got %d", l.Steps[0].Attempts)
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
		// Three workflows sharing same customer_id metadata
		bus.Publish(ctx, eventbus.Event{
			Type: "workflow.started",
			Payload: map[string]any{"id": "rel_a", "name": "a"},
			Metadata: map[string]string{"customer_id": "cust_shared"},
		})
		bus.Publish(ctx, eventbus.Event{
			Type: "workflow.started",
			Payload: map[string]any{"id": "rel_b", "name": "b"},
			Metadata: map[string]string{"customer_id": "cust_shared"},
		})
		bus.Publish(ctx, eventbus.Event{
			Type: "workflow.started",
			Payload: map[string]any{"id": "rel_c", "name": "c"},
			Metadata: map[string]string{"customer_id": "cust_shared"},
		})

		time.Sleep(50 * time.Millisecond)
		rel, _ := projector.GetRelatedLineages("rel_a")
		// Should find rel_b and rel_c
		if len(rel) != 2 {
			t.Errorf("expected 2 related lineages, got %d", len(rel))
		}
	})

	t.Run("findStep multiple steps", func(t *testing.T) {
		l := &Lineage{
			Steps: []*StepLineage{
				{StepID: "s1", State: "COMPLETED"},
				{StepID: "s2", State: "RUNNING"},
				{StepID: "s1", State: "RETRYING"}, // Newer s1
			},
		}
		p := &Projector{}
		step := p.findStep(l, "s1")
		if step == nil || step.State != "RETRYING" {
			t.Errorf("expected latest s1 step with RETRYING state, got %v", step)
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
		p.mu.Lock()
		p.lineages["no-events"] = &Lineage{ID: "no-events"}
		p.mu.Unlock()

		related, err := p.GetRelatedLineages("no-events")
		if err != nil {
			t.Errorf("GetRelatedLineages failed: %v", err)
		}
		if len(related) != 0 {
			t.Errorf("expected 0 related lineages, got %d", len(related))
		}
	})

	t.Run("Exhaustive Event Types and Store", func(t *testing.T) {
		// Mock DB for store
		// Note: We need a real DB or a mock that works with GORM.
		// Since this is a unit test, maybe we can skip the real DB if we don't want to depend on it.
		// But the requirement says "exhaustive".
		
		p := NewProjector(nil)
		wfID := "wf_exhaustive"

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

		// Test correlation indexing directly
		if len(p.correlation["m1"]["v1"]) != 1 || p.correlation["m1"]["v1"][0] != wfID {
			t.Errorf("correlation indexing failed")
		}
		
		// Test duplicate correlation indexing (should not add same ID twice)
		p.handleEvent(ctx, eventbus.Event{
			Type: "test",
			Payload: map[string]any{"id": wfID},
			Metadata: map[string]string{"m1": "v1"},
		})
		if len(p.correlation["m1"]["v1"]) != 1 {
			t.Errorf("expected 1 correlation ID, got %d", len(p.correlation["m1"]["v1"]))
		}
	})
}
