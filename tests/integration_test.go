package tests

import (
	"context"
	"reflect"
	"testing"
	"unsafe"

	"github.com/GoHyperrr/hyperrr/api/graph"
	domain "github.com/GoHyperrr/hyperrr/pkg/ctxengine"
	"github.com/GoHyperrr/hyperrr/pkg/workflow"
	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
	"github.com/GoHyperrr/mdk"
)

func TestFullIntegration(t *testing.T) {
	ctx := context.Background()
	bus := eventbus.NewInMemBus()
	
	// Start Projector
	projector := domain.NewProjector(bus)
	projector.Start(ctx)

	// Setup Runner
	runner := workflow.NewRunner(bus, nil, nil)
	_ = runner.RegisterHandler("step1", func(sCtx mdk.StepContext) mdk.StepResult {
		return mdk.StepResult{Output: map[string]any{"result": "done"}}
	})

	// Run Workflow
	wf := &workflow.Workflow{
		ID:   "integration-wf",
		Name: "integration-wf",
		Steps: []workflow.Step{
			{ID: "s1", Uses: "step1"},
		},
	}
	
	workflowID := "int-123"
	_, err := runner.ExecuteSyncWorkflow(ctx, workflowID, wf, nil)
	if err != nil {
		t.Fatalf("workflow failed: %v", err)
	}

	// Verify Projection
	lineage, err := projector.GetLineage(workflowID)
	if err != nil {
		t.Fatalf("lineage not found: %v", err)
	}
	if lineage.State != workflow.StateCompleted {
		t.Errorf("expected %s, got %s", workflow.StateCompleted, lineage.State)
	}

	// Verify GraphQL (via Resolver)
	ctxMod := domain.NewModule()
	// Inject projector into the unexported projector field of ctxMod using reflection
	v := reflect.ValueOf(ctxMod).Elem()
	f := v.FieldByName("projector")
	ptr := unsafe.Pointer(f.UnsafeAddr())
	*(**domain.Projector)(ptr) = projector

	resolver := &graph.Resolver{
		Projector:       projector,
		CtxEngineModule: ctxMod,
	}
	gqlLineage, err := resolver.Query().GetWorkflowLineage(ctx, workflowID)
	if err != nil {
		t.Fatalf("gql query failed: %v", err)
	}
	if gqlLineage.Name != "integration-wf" {
		t.Errorf("expected name integration-wf, got %s", gqlLineage.Name)
	}
}
