package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
)

func TestRunner_ResumeExecution(t *testing.T) {
	ctx := context.Background()
	bus := eventbus.NewInMemBus()
	store := NewInMemStore()

	// Use an injected store
	runner := NewRunner(bus, store, nil)

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
		runner.RegisterTask("ok_task", func(ctx context.Context, input any) (any, error) {
			return "ok", nil
		})
		runner.RegisterTask("fail_task", func(ctx context.Context, input any) (any, error) {
			return nil, errors.New("step failed")
		})
		runner.RegisterTask("comp_task", func(ctx context.Context, input any) (any, error) {
			compCalled = true
			return nil, nil
		})

		execID := "resume-fail"
		inputBytes, _ := json.Marshal(map[string]any{"x": 1})
		store.SaveInput(ctx, execID, inputBytes)
		store.InitializeExecution(ctx, execID, inputBytes)

		// s1 already completed
		store.SaveState(ctx, execID, "s1", StateCompleted)
		s1Out, _ := json.Marshal("ok")
		store.SaveStepOutput(ctx, execID, "s1", s1Out)

		wf := &Workflow{
			Steps: []Step{
				{ID: "s1", Uses: "ok_task", Saga: &Saga{Uses: "comp_task"}},
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
		runner.RegisterTask("rerun_task", func(ctx context.Context, input any) (any, error) {
			callCount++
			return "rerun_result", nil
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
		if res["s1"] != "rerun_result" {
			t.Errorf("expected rerun_result, got %v", res["s1"])
		}
	})

	t.Run("Context cancellation during resume", func(t *testing.T) {
		store := NewInMemStore()
		runner := NewRunner(bus, store, nil)

		runner.RegisterTask("slow_task", func(ctx context.Context, input any) (any, error) {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(5 * time.Second):
				return "done", nil
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
		runner.RegisterTask("t1", func(ctx context.Context, input any) (any, error) { return nil, nil })

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

	t.Run("Resume completed step with empty output bytes", func(t *testing.T) {
		store := NewInMemStore()
		runner := NewRunner(bus, store, nil)

		runner.RegisterTask("next_task", func(ctx context.Context, input any) (any, error) {
			return "ok", nil
		})

		execID := "resume-empty-output"
		inputBytes, _ := json.Marshal(map[string]any{"x": 1})
		store.SaveInput(ctx, execID, inputBytes)
		store.InitializeExecution(ctx, execID, inputBytes)
		store.SaveState(ctx, execID, "s1", StateCompleted)
		store.SaveStepOutput(ctx, execID, "s1", []byte{}) // empty output

		wf := &Workflow{
			Steps: []Step{
				{ID: "s1", Uses: "next_task"},
				{ID: "s2", Uses: "next_task", DependsOn: []string{"s1"}},
			},
		}

		res, err := runner.ResumeExecution(ctx, execID, wf)
		if err != nil {
			t.Fatalf("ResumeExecution failed: %v", err)
		}
		if res["s2"] != "ok" {
			t.Errorf("expected ok, got %v", res["s2"])
		}
	})
}

func TestRunner_Execute_ContextCancellation(t *testing.T) {
	bus := eventbus.NewInMemBus()
	runner := NewRunner(bus, nil, nil)

	runner.RegisterTask("slow", func(ctx context.Context, input any) (any, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(5 * time.Second):
			return "done", nil
		}
	})

	wf := &Workflow{
		Steps: []Step{{ID: "s1", Uses: "slow"}},
	}

	ctx, cancel := context.WithCancel(context.Background())
	errChan := make(chan error, 1)
	go func() {
		_, err := runner.Execute(ctx, "ctx-cancel-exec", wf, nil)
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

func TestRunner_ResumeWorkflow_NotWaiting(t *testing.T) {
	bus := eventbus.NewInMemBus()
	runner := NewRunner(bus, nil, nil)

	err := runner.ResumeWorkflow("non-existent-wf", "retry")
	if err == nil {
		t.Error("expected error for non-waiting workflow")
	}
}

func TestRunner_ApplyBackoff_NilPolicy(t *testing.T) {
	r := &Runner{}
	// Should not panic
	r.applyBackoff(nil, 1)
}

func TestRunner_Compensate_NonCriticalFailure(t *testing.T) {
	bus := eventbus.NewInMemBus()
	runner := NewRunner(bus, nil, nil)

	runner.RegisterTask("comp_non_crit", func(ctx context.Context, input any) (any, error) {
		return nil, errors.New("comp failed non-critical")
	})

	// Should not panic, just log
	runner.compensate(context.Background(), "test-id", []Step{
		{ID: "s1", Uses: "t1", Saga: &Saga{Uses: "comp_non_crit", IsCritical: false}},
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
