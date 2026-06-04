package workflow

import (
	"context"
	"errors"
	"testing"

	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
	"github.com/GoHyperrr/mdk"
)

func TestSagas(t *testing.T) {
	bus := eventbus.NewInMemBus()
	runner := NewRunner(bus, nil, nil)

	t.Run("Full Rollback", func(t *testing.T) {
		s1Done := false
		s1Compensated := false
		s2Done := false

		_ = runner.RegisterHandler("step1", func(sCtx mdk.StepContext) mdk.StepResult {
			s1Done = true
			return mdk.StepResult{Output: map[string]any{"res": "res1"}}
		})
		_ = runner.RegisterHandler("comp1", func(sCtx mdk.StepContext) mdk.StepResult {
			s1Compensated = true
			return mdk.StepResult{}
		})
		_ = runner.RegisterHandler("step2", func(sCtx mdk.StepContext) mdk.StepResult {
			s2Done = true
			return mdk.StepResult{Err: errors.New("fail at step 2")}
		})

		wf := mdk.Workflow{
			ID:   "saga-wf",
			Name: "saga-wf",
			Steps: []mdk.Step{
				{
					ID:   "s1",
					Uses: "step1",
					Saga: &mdk.Saga{Uses: "comp1"},
				},
				{
					ID:        "s2",
					Uses:      "step2",
					DependsOn: []string{"s1"},
				},
			},
		}

		_, err := runner.ExecuteSyncWorkflow(context.Background(), "saga-123", &wf, nil)
		if err == nil {
			t.Fatal("expected workflow failure")
		}

		if !s1Done {
			t.Error("step 1 should have run")
		}
		if !s2Done {
			t.Error("step 2 should have run and failed")
		}
		if !s1Compensated {
			t.Error("step 1 should have been compensated")
		}
	})

	t.Run("Nested Compensation", func(t *testing.T) {
		orderCancelled := false
		invReleased := false

		_ = runner.RegisterHandler("create_order", func(sCtx mdk.StepContext) mdk.StepResult {
			return mdk.StepResult{Output: map[string]any{"res": "order_1"}}
		})
		_ = runner.RegisterHandler("cancel_order", func(sCtx mdk.StepContext) mdk.StepResult {
			orderCancelled = true
			return mdk.StepResult{}
		})
		_ = runner.RegisterHandler("reserve_inv", func(sCtx mdk.StepContext) mdk.StepResult {
			return mdk.StepResult{Output: map[string]any{"res": "inv_1"}}
		})
		_ = runner.RegisterHandler("release_inv", func(sCtx mdk.StepContext) mdk.StepResult {
			invReleased = true
			return mdk.StepResult{}
		})
		_ = runner.RegisterHandler("payment", func(sCtx mdk.StepContext) mdk.StepResult {
			return mdk.StepResult{Err: errors.New("insufficient funds")}
		})

		wf := mdk.Workflow{
			ID:   "saga-nested",
			Name: "saga-nested",
			Steps: []mdk.Step{
				{ID: "order", Uses: "create_order", Saga: &mdk.Saga{Uses: "cancel_order"}},
				{ID: "inv", Uses: "reserve_inv", Saga: &mdk.Saga{Uses: "release_inv"}, DependsOn: []string{"order"}},
				{ID: "pay", Uses: "payment", DependsOn: []string{"inv"}},
			},
		}

		_, err := runner.ExecuteSyncWorkflow(context.Background(), "saga-nested-exec", &wf, nil)
		if err == nil {
			t.Fatal("expected failure")
		}

		if !orderCancelled || !invReleased {
			t.Error("all steps should have been compensated")
		}
	})
}
