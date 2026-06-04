package workflow

import (
	"context"
	"errors"
	"testing"

	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
	"github.com/GoHyperrr/mdk"
)

func TestFailurePolicies(t *testing.T) {
	bus := eventbus.NewInMemBus()
	runner := NewRunner(bus, nil, nil)

	t.Run("Retry Success", func(t *testing.T) {
		attempts := 0
		_ = runner.RegisterHandler("flaky", func(sCtx mdk.StepContext) mdk.StepResult {
			attempts++
			if attempts < 2 {
				return mdk.StepResult{Err: errors.New("fail")}
			}
			return mdk.StepResult{Output: map[string]any{"res": "ok"}}
		})

		wf := mdk.Workflow{
			ID:   "f1",
			Name: "f1",
			Steps: []mdk.Step{
				{
					ID:         "s1",
					Uses:       "flaky",
					MaxRetries: 2,
				},
			},
		}

		res, err := runner.ExecuteSyncWorkflow(context.Background(), "exec_f1", &wf, nil)
		if err != nil {
			t.Fatalf("workflow failed: %v", err)
		}
		
		s1Res, ok := res["s1"].(map[string]any)
		if !ok || s1Res["res"] != "ok" {
			t.Errorf("expected ok, got %v", res["s1"])
		}
	})

	t.Run("Retry Exhausted", func(t *testing.T) {
		_ = runner.RegisterHandler("always-fail", func(sCtx mdk.StepContext) mdk.StepResult {
			return mdk.StepResult{Err: errors.New("permanent-fail")}
		})

		wf := mdk.Workflow{
			ID:   "f2",
			Name: "f2",
			Steps: []mdk.Step{
				{
					ID:         "s1",
					Uses:       "always-fail",
					MaxRetries: 1,
				},
			},
		}

		_, err := runner.ExecuteSyncWorkflow(context.Background(), "exec_f2", &wf, nil)
		if err == nil {
			t.Fatal("expected error after retries")
		}
	})
}
