package context

import (
	"context"
	"testing"
	"time"

	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
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
	})

	t.Run("ListLineages", func(t *testing.T) {
		res := projector.ListLineages()
		if len(res) < 3 { // wf123, wf_fail, wf_human
			t.Errorf("expected at least 3 lineages, got %d", len(res))
		}
	})

	t.Run("RelatedLineages", func(t *testing.T) {
		bus.Publish(ctx, eventbus.Event{
			Type: "workflow.started",
			Payload: map[string]any{"id": "wf_rel_1", "name": "n", "version": "v"},
			Metadata: map[string]string{"key": "val"},
		})
		bus.Publish(ctx, eventbus.Event{
			Type: "workflow.started",
			Payload: map[string]any{"id": "wf_rel_2", "name": "n", "version": "v"},
			Metadata: map[string]string{"key": "val"},
		})
		
		rel, _ := projector.GetRelatedLineages("wf_rel_1")
		if len(rel) != 1 || rel[0].ID != "wf_rel_2" {
			t.Errorf("expected 1 related lineage wf_rel_2, got %d", len(rel))
		}
	})

	t.Run("RelatedLineages Not Found", func(t *testing.T) {
		_, err := projector.GetRelatedLineages("ghost")
		if err == nil {
			t.Error("expected error for non-existent lineage")
		}
	})

	t.Run("Invalid Payload", func(t *testing.T) {
		bus.Publish(ctx, eventbus.Event{
			Type: "workflow.started",
			Payload: "not a map",
		})
		// Should just ignore and not crash
	})

	t.Run("Payload Missing ID", func(t *testing.T) {
		bus.Publish(ctx, eventbus.Event{
			Type: "workflow.started",
			Payload: map[string]any{"name": "missing_id"},
		})
		// Should just ignore and not crash
	})

	t.Run("findStep Not Found", func(t *testing.T) {
		l := &Lineage{Steps: []*StepLineage{{StepID: "s1"}}}
		p := &Projector{}
		if p.findStep(l, "ghost") != nil {
			t.Error("expected nil for non-existent step")
		}
	})
}
