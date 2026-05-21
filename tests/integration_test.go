package tests

import (
	"context"
	"testing"

	domain "github.com/GoHyperrr/hyperrr/internal/context"
	"github.com/GoHyperrr/hyperrr/internal/context/graph"
	"github.com/GoHyperrr/hyperrr/internal/workflow"
	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
)

func TestFullIntegration(t *testing.T) {
	ctx := context.Background()
	bus := eventbus.NewInMemBus()
	
	// Start Projector
	projector := domain.NewProjector(bus)
	projector.Start(ctx)

	// Setup Runner
	runner := workflow.NewRunner(bus)
	runner.RegisterTask("step1", func(ctx context.Context, input any) (any, error) {
		return "done", nil
	})

	// Run Workflow
	wf := &workflow.Workflow{
		Name: "integration-wf",
		Steps: []workflow.Step{
			{ID: "s1", Uses: "step1"},
		},
	}
	
	workflowID := "int-123"
	err := runner.Execute(ctx, workflowID, wf, nil)
	if err != nil {
		t.Fatalf("workflow failed: %v", err)
	}

	// Verify Projection
	lineage, err := projector.GetLineage(workflowID)
	if err != nil {
		t.Fatalf("lineage not found: %v", err)
	}
	if lineage.State != "COMPLETED" {
		t.Errorf("expected COMPLETED, got %s", lineage.State)
	}

	// Verify GraphQL (via Resolver)
	resolver := &graph.Resolver{Projector: projector}
	gqlLineage, err := resolver.Query().GetWorkflowLineage(ctx, workflowID)
	if err != nil {
		t.Fatalf("gql query failed: %v", err)
	}
	if gqlLineage.Name != "integration-wf" {
		t.Errorf("expected name integration-wf, got %s", gqlLineage.Name)
	}
}
