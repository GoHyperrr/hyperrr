package workflow

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
)

func TestStateStore_Common(t *testing.T) {
	ctx := context.Background()
	
	stores := map[string]StateStore{
		"InMem": NewInMemStore(),
	}

	for name, store := range stores {
		t.Run(name, func(t *testing.T) {
			execID := "exec-" + name
			
			// Test Input
			input := map[string]any{"key": "val"}
			inputBytes, _ := json.Marshal(input)
			if err := store.SaveInput(ctx, execID, inputBytes); err != nil {
				t.Errorf("SaveInput failed: %v", err)
			}
			
			gotInput, err := store.GetInput(ctx, execID)
			if err != nil {
				t.Errorf("GetInput failed: %v", err)
			}
			if !reflect.DeepEqual(gotInput, inputBytes) {
				t.Error("input mismatch")
			}
			
			// Test State
			if err := store.SaveState(ctx, execID, "s1", StateRunning); err != nil {
				t.Errorf("SaveState failed: %v", err)
			}
			states, _ := store.GetState(ctx, execID)
			if states["s1"] != StateRunning {
				t.Errorf("expected RUNNING, got %s", states["s1"])
			}
			
			// Test Output
			out := map[string]any{"res": 123}
			outBytes, _ := json.Marshal(out)
			if err := store.SaveStepOutput(ctx, execID, "s1", outBytes); err != nil {
				t.Errorf("SaveStepOutput failed: %v", err)
			}
			gotOut, err := store.GetStepOutput(ctx, execID, "s1")
			if err != nil {
				t.Errorf("GetStepOutput failed: %v", err)
			}
			if !reflect.DeepEqual(gotOut, outBytes) {
				t.Error("output mismatch")
			}
			
			// Test Missing
			if _, err := store.GetState(ctx, "ghost"); err == nil {
				t.Error("expected error for missing state")
			}
			if _, err := store.GetInput(ctx, "ghost"); err == nil {
				t.Error("expected error for missing input")
			}
			if _, err := store.GetStepOutput(ctx, execID, "ghost"); err == nil {
				t.Error("expected error for missing step output")
			}
		})
	}
}

func TestWorkflowContext(t *testing.T) {
	bus := eventbus.NewInMemBus()
	runner := NewRunner(bus, nil)
	ctx := context.Background()
	
	id := "ctx-test"
	wCtx := WithRunner(ctx, runner, id)
	
	if GetWorkflowID(wCtx) != id {
		t.Errorf("expected id %s, got %s", id, GetWorkflowID(wCtx))
	}
	
	if GetStateStore(wCtx) == nil {
		t.Error("missing state store in context")
	}
	
	if GetEventBus(wCtx) != bus {
		t.Error("event bus mismatch in context")
	}
	
	// Test Emit
	payload := map[string]any{"foo": "bar"}
	if err := Emit(wCtx, "test.evt", payload); err != nil {
		t.Errorf("Emit failed: %v", err)
	}
	if payload["_workflow_id"] != id {
		t.Error("workflow id not injected into payload")
	}
	
	// Test Emit Error
	if err := Emit(ctx, "fail", nil); err == nil {
		t.Error("expected error for missing runner in context")
	}
}

func TestRunner_Execute_Checkpointing(t *testing.T) {
	bus := eventbus.NewInMemBus()
	store := NewInMemStore()
	runner := NewRunner(bus, store)
	ctx := context.Background()
	
	runner.RegisterTask("t1", func(ctx context.Context, input any) (any, error) {
		return "ok", nil
	})
	
	wf := &Workflow{
		Name: "check-wf",
		Steps: []Step{{ID: "s1", Uses: "t1"}},
	}
	
	id := "exec-check"
	runner.Execute(ctx, id, wf, map[string]any{"start": true})
	
	// Verify store has input
	inp, _ := store.GetInput(ctx, id)
	if len(inp) == 0 { t.Error("input not checkpointed") }
	
	// Verify store has state
	states, _ := store.GetState(ctx, id)
	if states["s1"] != StateCompleted {
		t.Errorf("expected COMPLETED, got %s", states["s1"])
	}
	
	// Verify store has output
	out, _ := store.GetStepOutput(ctx, id, "s1")
	if string(out) != `"ok"` {
		t.Errorf("expected \"ok\", got %s", string(out))
	}
}
