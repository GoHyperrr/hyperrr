package workflow

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
	"github.com/GoHyperrr/hyperrr/pkg/locking"
	"github.com/GoHyperrr/mdk"
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
	lock := locking.NewInMemLocker()
	runner := NewRunner(bus, nil, lock)
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

	// 1. Context GetStateStore and GetEventBus with no runner
	if GetStateStore(ctx) != nil {
		t.Error("expected nil state store when no runner in context")
	}
	if GetEventBus(ctx) != nil {
		t.Error("expected nil event bus when no runner in context")
	}

	// 2. Lock context error paths (missing runner)
	_, err := AcquireLock(ctx, "mylock", 1*time.Second, 1*time.Second)
	if err == nil {
		t.Error("expected error for AcquireLock with missing runner")
	}
	err = ReleaseLock(ctx, "mylock")
	if err == nil {
		t.Error("expected error for ReleaseLock with missing runner")
	}

	// 3. Lock context error paths (missing locker)
	runnerNoLocker := NewRunner(bus, nil, nil)
	runnerNoLocker.locker = nil
	wCtxNoLocker := WithRunner(ctx, runnerNoLocker, id)
	_, err = AcquireLock(wCtxNoLocker, "mylock", 1*time.Second, 1*time.Second)
	if err == nil {
		t.Error("expected error for AcquireLock with missing locker")
	}
	err = ReleaseLock(wCtxNoLocker, "mylock")
	if err == nil {
		t.Error("expected error for ReleaseLock with missing locker")
	}

	// 4. Lock Context success paths
	ok, err := AcquireLock(wCtx, "mylock", 100*time.Millisecond, 100*time.Millisecond)
	if err != nil || !ok {
		t.Errorf("AcquireLock failed: err=%v, ok=%v", err, ok)
	}
	err = ReleaseLock(wCtx, "mylock")
	if err != nil {
		t.Errorf("ReleaseLock failed: %v", err)
	}
}

func TestWorkflow_IdempotentEmit(t *testing.T) {
	bus := eventbus.NewInMemBus()
	store := NewInMemStore()
	runner := NewRunner(bus, store, nil)
	ctx := context.Background()
	
	id := "idempotent-exec"
	wCtx := WithRunner(ctx, runner, id)
	
	// Mock a handler that emits an event
	count := 0
	_, _ = bus.Subscribe("test", "idempotent", func(ctx context.Context, event mdk.Event) error {
		count++
		return nil
	})
	
	// First emit
	Emit(wCtx, "test.idempotent", map[string]any{"data": 1})
	if count != 1 {
		t.Errorf("expected 1 emission, got %d", count)
	}
	
	// Second emit (same type)
	Emit(wCtx, "test.idempotent", map[string]any{"data": 1})
	if count != 1 {
		t.Errorf("expected still 1 emission (idempotency), got %d", count)
	}
}

func TestRunner_Execute_Checkpointing(t *testing.T) {
	bus := eventbus.NewInMemBus()
	store := NewInMemStore()
	runner := NewRunner(bus, store, nil)
	ctx := context.Background()
	
	_ = runner.RegisterHandler("t1", func(sCtx mdk.StepContext) mdk.StepResult {
		return mdk.StepResult{Output: map[string]any{"res": "ok"}}
	})
	
	wf := &Workflow{
		ID:   "check-wf",
		Name: "check-wf",
		Steps: []Step{{ID: "s1", Uses: "t1"}},
	}
	
	id := "exec-check"
	_, _ = runner.ExecuteSyncWorkflow(ctx, id, wf, map[string]any{"start": true})
	
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
	if string(out) != `{"res":"ok"}` {
		t.Errorf("expected `{\"res\":\"ok\"}`, got %s", string(out))
	}
}

func TestInMemStore_ExtraPaths(t *testing.T) {
	ctx := context.Background()
	
	t.Run("Expired state and input", func(t *testing.T) {
		store := NewInMemStore()
		execID := "expire-exec"
		_ = store.InitializeExecution(ctx, execID, []byte("inp"))
		_ = store.SaveState(ctx, execID, "s1", StateRunning)
		
		// Set TTL to past time or very short TTL
		_ = store.SetTTL(ctx, execID, -1*time.Second)
		
		_, err := store.GetState(ctx, execID)
		if err == nil {
			t.Error("expected error for expired state")
		}
		
		_, err = store.GetInput(ctx, execID)
		if err == nil {
			t.Error("expected error for expired input")
		}
	})

	t.Run("ListExecutions", func(t *testing.T) {
		store := NewInMemStore()
		_ = store.InitializeExecution(ctx, "e1", []byte("inp"))
		_ = store.InitializeExecution(ctx, "e2", []byte("inp"))
		_ = store.SaveState(ctx, "e1", "", StateRunning)
		_ = store.SaveState(ctx, "e2", "", StateCompleted)

		running, _ := store.ListExecutions(ctx, StateRunning)
		if len(running) != 1 || running[0] != "e1" {
			t.Errorf("expected [e1], got %v", running)
		}
	})

	t.Run("Close and cleanup routine", func(t *testing.T) {
		// Set customizable cleanup interval to 10ms for this test
		oldInterval := inMemStoreCleanupInterval
		inMemStoreCleanupInterval = 10 * time.Millisecond
		defer func() { inMemStoreCleanupInterval = oldInterval }()

		store := NewInMemStore()
		_ = store.InitializeExecution(ctx, "e-clean", []byte("inp"))
		_ = store.SetTTL(ctx, "e-clean", 5 * time.Millisecond)

		// Wait for cleanup routine to tick and delete expired records
		time.Sleep(30 * time.Millisecond)

		// Check if clean-up succeeded (data should be gone)
		store.mu.Lock()
		_, exists := store.inputs["e-clean"]
		store.mu.Unlock()
		if exists {
			t.Error("expected e-clean input to be deleted by cleanup routine")
		}

		err := store.Close()
		if err != nil {
			t.Errorf("Close failed: %v", err)
		}
	})
}
