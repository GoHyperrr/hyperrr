package workflow

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
)

func TestRunner_ResumeExecution(t *testing.T) {
	ctx := context.Background()
	bus := eventbus.NewInMemBus()
	store := NewInMemStore()

	// Use an injected store
	runner := NewRunner(bus, store)

	var step1Count, step2Count int

	runner.RegisterTask("step1_handler", func(ctx context.Context, input any) (any, error) {
		step1Count++
		return map[string]any{"step1_res": "ok"}, nil
	})

	runner.RegisterTask("step2_handler", func(ctx context.Context, input any) (any, error) {
		step2Count++
		return map[string]any{"step2_res": "done"}, nil
	})

	wf := &Workflow{
		Name:    "resume_test_wf",
		Version: "v1",
		Steps: []Step{
			{ID: "step1", Uses: "step1_handler"},
			{ID: "step2", Uses: "step2_handler", DependsOn: []string{"step1"}},
		},
	}

	execID := "resume_test_exec_1"
	
	// Simulate an interrupted workflow where step1 completed successfully but step2 never started
	
	// Setup mock state in store
	input := map[string]any{"user_id": "u1"}
	inputBytes, _ := json.Marshal(input)
	store.SaveInput(ctx, execID, inputBytes)
	
	store.SaveState(ctx, execID, "step1", StateCompleted)
	
	step1Res := map[string]any{"step1_res": "ok"}
	step1ResBytes, _ := json.Marshal(step1Res)
	store.SaveStepOutput(ctx, execID, "step1", step1ResBytes)
	
	// Now attempt to resume
	res, err := runner.ResumeExecution(ctx, execID, wf)
	if err != nil {
		t.Fatalf("ResumeExecution failed: %v", err)
	}
	
	// Validate results
	if step1Count != 0 {
		t.Errorf("step1 should not have been executed, ran %d times", step1Count)
	}
	
	if step2Count != 1 {
		t.Errorf("step2 should have been executed exactly once, ran %d times", step2Count)
	}
	
	if res["step1"] == nil {
		t.Errorf("step1 results missing from final output")
	} else {
		resMap := res["step1"].(map[string]any)
		if resMap["step1_res"] != "ok" {
			t.Errorf("expected step1_res=ok, got %v", resMap["step1_res"])
		}
	}
	
	if res["step2"] == nil {
		t.Errorf("step2 results missing from final output")
	}
}

func TestRunner_ResumeExecution_NoStore(t *testing.T) {
	runner := &Runner{handlers: make(map[string]TaskHandler)}
	_, err := runner.ResumeExecution(context.Background(), "id", &Workflow{})
	if err == nil {
		t.Error("expected error when resuming without StateStore")
	}
}
