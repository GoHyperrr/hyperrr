package workflow

import (
	"context"
	"errors"
	"testing"

	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
)

func TestSagas(t *testing.T) {
	bus := eventbus.NewInMemBus()
	runner := NewRunner(bus, nil, nil)

	t.Run("Full Rollback", func(t *testing.T) {
		s1Done := false
		s1Compensated := false
		s2Done := false

		runner.RegisterTask("step1", func(ctx context.Context, input any) (any, error) {
			s1Done = true
			return "res1", nil
		})
		runner.RegisterTask("comp1", func(ctx context.Context, input any) (any, error) {
			s1Compensated = true
			return nil, nil
		})
		runner.RegisterTask("step2", func(ctx context.Context, input any) (any, error) {
			s2Done = true
			return nil, errors.New("fail at step 2")
		})

		wf := &Workflow{
			Name: "saga-wf",
			Steps: []Step{
				{
					ID:   "s1",
					Uses: "step1",
					Saga: &Saga{Uses: "comp1"},
				},
				{
					ID:         "s2",
					Uses:       "step2",
					DependsOn:  []string{"s1"},
				},
			},
		}

		_, err := runner.Execute(context.Background(), "saga-123", wf, nil)
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

		runner.RegisterTask("create_order", func(ctx context.Context, input any) (any, error) {
			return "order_1", nil
		})
		runner.RegisterTask("cancel_order", func(ctx context.Context, input any) (any, error) {
			orderCancelled = true
			return nil, nil
		})
		runner.RegisterTask("reserve_inv", func(ctx context.Context, input any) (any, error) {
			return "inv_1", nil
		})
		runner.RegisterTask("release_inv", func(ctx context.Context, input any) (any, error) {
			invReleased = true
			return nil, nil
		})
		runner.RegisterTask("payment", func(ctx context.Context, input any) (any, error) {
			return nil, errors.New("insufficient funds")
		})

		wf := &Workflow{
			Steps: []Step{
				{ID: "order", Uses: "create_order", Saga: &Saga{Uses: "cancel_order"}},
				{ID: "inv", Uses: "reserve_inv", Saga: &Saga{Uses: "release_inv"}, DependsOn: []string{"order"}},
				{ID: "pay", Uses: "payment", DependsOn: []string{"inv"}},
			},
		}

		_, err := runner.Execute(context.Background(), "saga-nested", wf, nil)
		if err == nil {
			t.Fatal("expected failure")
		}

		if !orderCancelled || !invReleased {
			t.Error("all steps should have been compensated")
		}
	})
}
