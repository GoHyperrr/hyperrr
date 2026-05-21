package graph

import (
	"context"
	"testing"
	"time"

	domain "github.com/GoHyperrr/hyperrr/internal/context"
	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
)

func TestGraphResolvers(t *testing.T) {
	bus := eventbus.NewInMemBus()
	projector := domain.NewProjector(bus)
	ctx := context.Background()

	err := projector.Start(ctx)
	if err != nil {
		t.Fatalf("failed to start projector: %v", err)
	}

	resolver := &Resolver{Projector: projector}

	// Setup some data
	workflowID := "wf_graph"
	bus.Publish(ctx, eventbus.Event{
		Type:      "workflow.started",
		Timestamp: time.Now(),
		Payload: map[string]any{
			"id":      workflowID,
			"name":    "graph-workflow",
			"version": "v1",
		},
		Metadata: map[string]string{"order_id": "ord123"},
	})
	
	relatedID := "wf_related"
	bus.Publish(ctx, eventbus.Event{
		Type:      "workflow.started",
		Timestamp: time.Now(),
		Payload:   map[string]any{"id": relatedID, "name": "rel", "version": "v1"},
		Metadata:  map[string]string{"order_id": "ord123"},
	})

	t.Run("GetWorkflowLineage", func(t *testing.T) {
		res, err := resolver.Query().GetWorkflowLineage(ctx, workflowID)
		if err != nil {
			t.Fatalf("failed to get lineage: %v", err)
		}
		if res.ID != workflowID {
			t.Errorf("expected ID %s, got %s", workflowID, res.ID)
		}
	})

	t.Run("ListLineages", func(t *testing.T) {
		res, err := resolver.Query().ListLineages(ctx)
		if err != nil {
			t.Fatalf("failed to list lineages: %v", err)
		}
		if len(res) < 2 {
			t.Errorf("expected at least 2 lineages, got %d", len(res))
		}
	})

	t.Run("RelatedLineages", func(t *testing.T) {
		lineage, _ := resolver.Query().GetWorkflowLineage(ctx, workflowID)
		related, err := resolver.WorkflowLineage().RelatedLineages(ctx, lineage)
		if err != nil {
			t.Fatalf("failed to get related lineages: %v", err)
		}
		if len(related) != 1 || related[0].ID != relatedID {
			t.Errorf("expected related workflow %s, got %d related", relatedID, len(related))
		}
	})
	
	t.Run("GetWorkflowLineage with Error", func(t *testing.T) {
		failID := "wf_err"
		bus.Publish(ctx, eventbus.Event{
			Type:    "workflow.started",
			Payload: map[string]any{"id": failID, "name": "fail", "version": "v1"},
		})
		bus.Publish(ctx, eventbus.Event{
			Type:    "workflow.step.started",
			Payload: map[string]any{"id": failID, "step_id": "s1"},
		})
		bus.Publish(ctx, eventbus.Event{
			Type:    "workflow.step.failed",
			Payload: map[string]any{"id": failID, "step_id": "s1", "error": "step failure"},
		})
		bus.Publish(ctx, eventbus.Event{
			Type:    "workflow.failed",
			Payload: map[string]any{"id": failID, "error": "fatal failure"},
		})
		
		res, _ := resolver.Query().GetWorkflowLineage(ctx, failID)
		if res.Error == nil || *res.Error != "fatal failure" {
			t.Errorf("expected error message, got %v", res.Error)
		}
		if len(res.Steps) == 0 || res.Steps[0].Error == nil || *res.Steps[0].Error != "step failure" {
			t.Errorf("expected step error message, got %v", res.Steps)
		}
	})

	t.Run("Events", func(t *testing.T) {
		lineage, _ := resolver.Query().GetWorkflowLineage(ctx, workflowID)
		// Check if resolver has the method
		_, err := resolver.WorkflowLineage().Events(ctx, lineage)
		if err != nil {
			t.Errorf("Events resolver failed: %v", err)
		}
	})
}
