package workflow

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
)

func TestFailureOrchestration(t *testing.T) {
	bus := eventbus.NewInMemBus()
	runner := NewRunner(bus)

	t.Run("Retry Success", func(t *testing.T) {
		wf := &Workflow{
			Name: "retry",
			Steps: []Step{
				{
					ID:   "s1",
					Uses: "unstable",
					Retry: &Retry{
						MaxAttempts: 3,
						Backoff:     "constant",
						Interval:    10 * time.Millisecond,
					},
				},
			},
		}

		attempts := 0
		runner.RegisterTask("unstable", func(ctx context.Context, input any) (any, error) {
			attempts++
			if attempts < 2 {
				return nil, errors.New("temporary failure")
			}
			return "ok", nil
		})

		err := runner.Execute(context.Background(), "test-id", wf, nil)
		if err != nil {
			t.Fatalf("expected success after retry, got %v", err)
		}
		if attempts != 2 {
			t.Errorf("expected 2 attempts, got %d", attempts)
		}
	})

	t.Run("Retry Exhausted", func(t *testing.T) {
		wf := &Workflow{
			Name: "retry_fail",
			Steps: []Step{
				{
					ID:   "s1",
					Uses: "always_fail",
					Retry: &Retry{
						MaxAttempts: 2,
						Interval:    10 * time.Millisecond,
					},
				},
			},
		}

		runner.RegisterTask("always_fail", func(ctx context.Context, input any) (any, error) {
			return nil, errors.New("permanent failure")
		})

		err := runner.Execute(context.Background(), "test-id", wf, nil)
		if err == nil {
			t.Fatal("expected error after exhausted retries, got nil")
		}
	})

	t.Run("Fallback Execution", func(t *testing.T) {
		wf := &Workflow{
			Name: "fallback",
			Steps: []Step{
				{
					ID:   "s1",
					Uses: "fail",
					Fallback: &Fallback{
						Step: "recovery",
					},
				},
			},
		}

		runner.RegisterTask("fail", func(ctx context.Context, input any) (any, error) {
			return nil, errors.New("fail")
		})
		runner.RegisterTask("recovery", func(ctx context.Context, input any) (any, error) {
			return "recovered", nil
		})

		err := runner.Execute(context.Background(), "test-id", wf, nil)
		if err != nil {
			t.Fatalf("expected success via fallback, got %v", err)
		}
	})
	
	t.Run("Constant Backoff", func(t *testing.T) {
		wf := &Workflow{
			Steps: []Step{
				{
					ID: "s1", Uses: "f",
					Retry: &Retry{MaxAttempts: 2, Backoff: "constant", Interval: 10 * time.Millisecond},
				},
			},
		}
		runner.RegisterTask("f", func(ctx context.Context, i any) (any, error) { return nil, errors.New("e") })
		start := time.Now()
		runner.Execute(context.Background(), "test-id", wf, nil)
		if time.Since(start) < 10*time.Millisecond {
			t.Error("expected backoff delay")
		}
	})

	t.Run("Fallback Handler Missing", func(t *testing.T) {
		wf := &Workflow{
			Steps: []Step{
				{ID: "s1", Uses: "fail", Fallback: &Fallback{Step: "missing"}},
			},
		}
		runner.RegisterTask("fail", func(ctx context.Context, i any) (any, error) { return nil, errors.New("e") })
		err := runner.Execute(context.Background(), "test-id", wf, nil)
		if err == nil {
			t.Fatal("expected error from main step failure after fallback missing")
		}
	})

	t.Run("ApplyBackoff Edge Cases", func(t *testing.T) {
		r := NewRunner(nil)
		// Should not panic
		r.applyBackoff(nil, 1)
		r.applyBackoff(&Retry{Interval: 0}, 1)
	})

	t.Run("Exponential Backoff", func(t *testing.T) {
		wf := &Workflow{
			Steps: []Step{
				{
					ID: "exp", Uses: "f",
					Retry: &Retry{MaxAttempts: 2, Backoff: "exponential", Interval: 10 * time.Millisecond},
				},
			},
		}
		runner.RegisterTask("f", func(ctx context.Context, i any) (any, error) { return nil, errors.New("e") })
		start := time.Now()
		runner.Execute(context.Background(), "test-id", wf, nil)
		if time.Since(start) < 10*time.Millisecond {
			t.Error("expected backoff delay")
		}
	})
}
