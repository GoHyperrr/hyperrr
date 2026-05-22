package workflow

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
)

func TestFailurePolicies(t *testing.T) {
	bus := eventbus.NewInMemBus()
	runner := NewRunner(bus)

	t.Run("Retry Success", func(t *testing.T) {
		attempts := 0
		runner.RegisterTask("flaky", func(ctx context.Context, input any) (any, error) {
			attempts++
			if attempts < 2 {
				return nil, errors.New("fail")
			}
			return "ok", nil
		})

		wf := &Workflow{
			Steps: []Step{
				{
					ID: "s1",
					Uses: "flaky",
					Retry: &Retry{MaxAttempts: 3, Interval: 10 * time.Millisecond},
				},
			},
		}

		res, err := runner.Execute(context.Background(), "f1", wf, nil)
		if err != nil {
			t.Fatalf("workflow failed: %v", err)
		}
		if res["s1"] != "ok" {
			t.Errorf("expected ok, got %v", res["s1"])
		}
	})

	t.Run("Retry Exhausted", func(t *testing.T) {
		runner.RegisterTask("always-fail", func(ctx context.Context, input any) (any, error) {
			return nil, errors.New("permanent-fail")
		})

		wf := &Workflow{
			Steps: []Step{
				{
					ID: "s1",
					Uses: "always-fail",
					Retry: &Retry{MaxAttempts: 2, Interval: 10 * time.Millisecond},
				},
			},
		}

		_, err := runner.Execute(context.Background(), "f2", wf, nil)
		if err == nil {
			t.Fatal("expected error after retries")
		}
	})

	t.Run("Fallback", func(t *testing.T) {
		runner.RegisterTask("fallback-task", func(ctx context.Context, input any) (any, error) {
			return "fallback-res", nil
		})

		wf := &Workflow{
			Steps: []Step{
				{
					ID: "s1",
					Uses: "always-fail",
					Fallback: &Fallback{Step: "fallback-task"},
				},
			},
		}

		res, err := runner.Execute(context.Background(), "f3", wf, nil)
		if err != nil {
			t.Fatalf("workflow failed: %v", err)
		}
		if res["s1"] != "fallback-res" {
			t.Errorf("expected fallback-res, got %v", res["s1"])
		}
	})

	t.Run("Exponential Backoff", func(t *testing.T) {
		attempts := 0
		runner.RegisterTask("backoff", func(ctx context.Context, input any) (any, error) {
			attempts++
			if attempts < 3 {
				return nil, errors.New("retry me")
			}
			return "ok", nil
		})

		wf := &Workflow{
			Steps: []Step{
				{
					ID: "s1",
					Uses: "backoff",
					Retry: &Retry{MaxAttempts: 5, Interval: 5 * time.Millisecond},
				},
			},
		}

		start := time.Now()
		_, err := runner.Execute(context.Background(), "backoff1", wf, nil)
		duration := time.Since(start)
		
		if err != nil {
			t.Fatalf("workflow failed: %v", err)
		}
		// Expecting at least (5 + 10) ms of sleep if backoff is 2x
		if duration < 10*time.Millisecond {
			t.Errorf("expected backoff delay, got %v", duration)
		}
	})
}

func TestSagaCompensate(t *testing.T) {
	bus := eventbus.NewInMemBus()
	runner := NewRunner(bus)

	t.Run("Saga Compensation", func(t *testing.T) {
		compCalled := false
		runner.RegisterTask("step1", func(ctx context.Context, input any) (any, error) { return "ok", nil })
		runner.RegisterTask("comp1", func(ctx context.Context, input any) (any, error) {
			compCalled = true
			return nil, nil
		})
		runner.RegisterTask("fail", func(ctx context.Context, input any) (any, error) { return nil, errors.New("fail") })

		wf := &Workflow{
			Steps: []Step{
				{ID: "s1", Uses: "step1", Saga: &Saga{Uses: "comp1"}},
				{ID: "s2", Uses: "fail", DependsOn: []string{"s1"}},
			},
		}

		_, err := runner.Execute(context.Background(), "saga1", wf, nil)
		if err == nil {
			t.Fatal("expected workflow failure")
		}
		if !compCalled {
			t.Error("compensation not called")
		}
	})
}
