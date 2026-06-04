package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
	"github.com/GoHyperrr/mdk"
)

func TestRunner_ResumeExecution(t *testing.T) {
	ctx := context.Background()
	bus := eventbus.NewInMemBus()
	store := NewInMemStore()

	runner := NewRunner(bus, store, nil)

	var step1Count, step2Count int

	_ = runner.RegisterHandler("step1_handler", func(sCtx mdk.StepContext) mdk.StepResult {
		step1Count++
		return mdk.StepResult{Output: map[string]any{"step1_res": "ok"}}
	})

	_ = runner.RegisterHandler("step2_handler", func(sCtx mdk.StepContext) mdk.StepResult {
		step2Count++
		return mdk.StepResult{Output: map[string]any{"step2_res": "done"}}
	})

	wf := &Workflow{
		ID:   "resume_test_wf",
		Name: "resume_test_wf",
		Steps: []Step{
			{ID: "step1", Uses: "step1_handler"},
			{ID: "step2", Uses: "step2_handler", DependsOn: []string{"step1"}},
		},
	}

	execID := "resume_test_exec_1"
	
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
	
	if step1Count != 0 {
		t.Errorf("step1 should not have been executed, ran %d times", step1Count)
	}
	
	if step2Count != 1 {
		t.Errorf("step2 should have been executed exactly once, ran %d times", step2Count)
	}
	
	s1Res, ok := res["step1"].(map[string]any)
	if !ok || s1Res["step1_res"] != "ok" {
		t.Errorf("expected step1_res=ok, got %v", res["step1"])
	}
}

func TestRunner_ResumeExecution_NoStore(t *testing.T) {
	runner := &Runner{}
	_, err := runner.ResumeExecution(context.Background(), "id", &Workflow{})
	if err == nil {
		t.Error("expected error when resuming without StateStore")
	}
}

func TestRunner_ResumeExecution_ErrorPaths(t *testing.T) {
	ctx := context.Background()
	bus := eventbus.NewInMemBus()

	t.Run("GetInput failure", func(t *testing.T) {
		store := NewInMemStore()
		runner := NewRunner(bus, store, nil)
		// Don't save any input, so GetInput will fail
		_, err := runner.ResumeExecution(ctx, "missing-input", &Workflow{})
		if err == nil {
			t.Error("expected error for missing input")
		}
	})

	t.Run("Invalid JSON input", func(t *testing.T) {
		store := NewInMemStore()
		runner := NewRunner(bus, store, nil)
		store.SaveInput(ctx, "bad-input", []byte("not-json"))
		_, err := runner.ResumeExecution(ctx, "bad-input", &Workflow{})
		if err == nil {
			t.Error("expected error for invalid JSON input")
		}
	})

	t.Run("GetState failure", func(t *testing.T) {
		store := NewInMemStore()
		runner := NewRunner(bus, store, nil)
		inputBytes, _ := json.Marshal(map[string]any{"x": 1})
		store.SaveInput(ctx, "no-state", inputBytes)
		// Don't initialize execution state, so GetState will fail
		_, err := runner.ResumeExecution(ctx, "no-state", &Workflow{
			Steps: []Step{{ID: "s1", Uses: "t1"}},
		})
		if err == nil {
			t.Error("expected error for missing state")
		}
	})

	t.Run("GetStepOutput failure for completed step", func(t *testing.T) {
		store := NewInMemStore()
		runner := NewRunner(bus, store, nil)
		inputBytes, _ := json.Marshal(map[string]any{"x": 1})
		store.SaveInput(ctx, "bad-output", inputBytes)
		store.InitializeExecution(ctx, "bad-output", inputBytes)
		store.SaveState(ctx, "bad-output", "s1", StateCompleted)
		// Don't save step output, so GetStepOutput will fail
		_, err := runner.ResumeExecution(ctx, "bad-output", &Workflow{
			Steps: []Step{{ID: "s1", Uses: "t1"}},
		})
		if err == nil {
			t.Error("expected error for missing step output")
		}
	})

	t.Run("Invalid JSON step output", func(t *testing.T) {
		store := NewInMemStore()
		runner := NewRunner(bus, store, nil)
		inputBytes, _ := json.Marshal(map[string]any{"x": 1})
		store.SaveInput(ctx, "bad-step-output", inputBytes)
		store.InitializeExecution(ctx, "bad-step-output", inputBytes)
		store.SaveState(ctx, "bad-step-output", "s1", StateCompleted)
		store.SaveStepOutput(ctx, "bad-step-output", "s1", []byte("not-json"))
		_, err := runner.ResumeExecution(ctx, "bad-step-output", &Workflow{
			Steps: []Step{{ID: "s1", Uses: "t1"}},
		})
		if err == nil {
			t.Error("expected error for invalid JSON step output")
		}
	})

	t.Run("Step failure during resume triggers compensation", func(t *testing.T) {
		store := NewInMemStore()
		runner := NewRunner(bus, store, nil)

		compCalled := false
		_ = runner.RegisterHandler("ok_task", func(sCtx mdk.StepContext) mdk.StepResult {
			return mdk.StepResult{Output: map[string]any{"res": "ok"}}
		})
		_ = runner.RegisterHandler("fail_task", func(sCtx mdk.StepContext) mdk.StepResult {
			return mdk.StepResult{Err: errors.New("step failed")}
		})
		_ = runner.RegisterHandler("comp_task", func(sCtx mdk.StepContext) mdk.StepResult {
			compCalled = true
			return mdk.StepResult{}
		})

		execID := "resume-fail"
		inputBytes, _ := json.Marshal(map[string]any{"x": 1})
		store.SaveInput(ctx, execID, inputBytes)
		store.InitializeExecution(ctx, execID, inputBytes)

		// s1 already completed
		store.SaveState(ctx, execID, "s1", StateCompleted)
		s1Out, _ := json.Marshal(map[string]any{"res": "ok"})
		store.SaveStepOutput(ctx, execID, "s1", s1Out)

		wf := &Workflow{
			Steps: []Step{
				{ID: "s1", Uses: "ok_task", Saga: &mdk.Saga{Uses: "comp_task"}},
				{ID: "s2", Uses: "fail_task", DependsOn: []string{"s1"}},
			},
		}

		_, err := runner.ResumeExecution(ctx, execID, wf)
		if err == nil {
			t.Error("expected error from failed step during resume")
		}
		if !compCalled {
			t.Error("expected compensation to be called")
		}
	})

	t.Run("Resume with RUNNING state step re-executes it", func(t *testing.T) {
		store := NewInMemStore()
		runner := NewRunner(bus, store, nil)

		callCount := 0
		_ = runner.RegisterHandler("rerun_task", func(sCtx mdk.StepContext) mdk.StepResult {
			callCount++
			return mdk.StepResult{Output: map[string]any{"res": "rerun_result"}}
		})

		execID := "resume-running"
		inputBytes, _ := json.Marshal(map[string]any{"x": 1})
		store.SaveInput(ctx, execID, inputBytes)
		store.InitializeExecution(ctx, execID, inputBytes)
		store.SaveState(ctx, execID, "s1", StateRunning) // Interrupted mid-execution

		wf := &Workflow{
			Steps: []Step{{ID: "s1", Uses: "rerun_task"}},
		}

		res, err := runner.ResumeExecution(ctx, execID, wf)
		if err != nil {
			t.Fatalf("ResumeExecution failed: %v", err)
		}
		if callCount != 1 {
			t.Errorf("expected step to be re-executed once, got %d", callCount)
		}
		s1Res, ok := res["s1"].(map[string]any)
		if !ok || s1Res["res"] != "rerun_result" {
			t.Errorf("expected rerun_result, got %v", res["s1"])
		}
	})

	t.Run("Context cancellation during resume", func(t *testing.T) {
		store := NewInMemStore()
		runner := NewRunner(bus, store, nil)

		_ = runner.RegisterHandler("slow_task", func(sCtx mdk.StepContext) mdk.StepResult {
			select {
			case <-sCtx.Ctx.Done():
				return mdk.StepResult{Err: sCtx.Ctx.Err()}
			case <-time.After(5 * time.Second):
				return mdk.StepResult{Output: map[string]any{"res": "done"}}
			}
		})

		execID := "resume-cancel"
		inputBytes, _ := json.Marshal(map[string]any{"x": 1})
		store.SaveInput(ctx, execID, inputBytes)
		store.InitializeExecution(ctx, execID, inputBytes)

		wf := &Workflow{
			Steps: []Step{{ID: "s1", Uses: "slow_task"}},
		}

		cCtx, cancel := context.WithCancel(ctx)
		errChan := make(chan error, 1)
		go func() {
			_, err := runner.ResumeExecution(cCtx, execID, wf)
			errChan <- err
		}()

		time.Sleep(20 * time.Millisecond)
		cancel()

		select {
		case err := <-errChan:
			if err != context.Canceled {
				t.Errorf("expected context.Canceled, got %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Error("timeout waiting for resume to return after cancel")
		}
	})

	t.Run("Resume deadlock detection", func(t *testing.T) {
		store := NewInMemStore()
		runner := NewRunner(bus, store, nil)
		_ = runner.RegisterHandler("t1", func(sCtx mdk.StepContext) mdk.StepResult { return mdk.StepResult{} })

		execID := "resume-deadlock"
		inputBytes, _ := json.Marshal(map[string]any{"x": 1})
		store.SaveInput(ctx, execID, inputBytes)
		store.InitializeExecution(ctx, execID, inputBytes)

		wf := &Workflow{
			Steps: []Step{
				{ID: "s1", Uses: "t1", DependsOn: []string{"s2"}},
				{ID: "s2", Uses: "t1", DependsOn: []string{"s1"}},
			},
		}

		_, err := runner.ResumeExecution(ctx, execID, wf)
		if err == nil {
			t.Error("expected deadlock error during resume")
		}
	})
}

func TestRunner_ExecuteSyncWorkflow_ContextCancellation(t *testing.T) {
	bus := eventbus.NewInMemBus()
	runner := NewRunner(bus, nil, nil)

	_ = runner.RegisterHandler("slow", func(sCtx mdk.StepContext) mdk.StepResult {
		select {
		case <-sCtx.Ctx.Done():
			return mdk.StepResult{Err: sCtx.Ctx.Err()}
		case <-time.After(5 * time.Second):
			return mdk.StepResult{Output: map[string]any{"res": "done"}}
		}
	})

	wf := &Workflow{
		Steps: []Step{{ID: "s1", Uses: "slow"}},
	}

	ctx, cancel := context.WithCancel(context.Background())
	errChan := make(chan error, 1)
	go func() {
		_, err := runner.ExecuteSyncWorkflow(ctx, "ctx-cancel-exec", wf, nil)
		errChan <- err
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-errChan:
		if err != context.Canceled {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("timeout waiting for execute to return after cancel")
	}
}

func TestRunner_Compensate_NonCriticalFailure(t *testing.T) {
	bus := eventbus.NewInMemBus()
	runner := NewRunner(bus, nil, nil)

	_ = runner.RegisterHandler("comp_non_crit", func(sCtx mdk.StepContext) mdk.StepResult {
		return mdk.StepResult{Err: errors.New("comp failed non-critical")}
	})

	// Should not panic, just log
	runner.compensate(context.Background(), "test-id", []Step{
		{ID: "s1", Uses: "t1", Saga: &mdk.Saga{Uses: "comp_non_crit", IsCritical: false}},
	}, map[string]any{})
}

func TestRunner_Compensate_NoSaga(t *testing.T) {
	bus := eventbus.NewInMemBus()
	runner := NewRunner(bus, nil, nil)

	// Step with no saga should be skipped
	runner.compensate(context.Background(), "test-id", []Step{
		{ID: "s1", Uses: "t1"},
	}, map[string]any{})
}

func TestRunner_Compensate_EmptyHistory(t *testing.T) {
	bus := eventbus.NewInMemBus()
	runner := NewRunner(bus, nil, nil)

	// Empty history - should return immediately
	runner.compensate(context.Background(), "test-id", nil, map[string]any{})
}
