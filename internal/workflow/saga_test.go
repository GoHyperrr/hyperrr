package workflow

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
)

func TestSaga(t *testing.T) {
	bus := eventbus.NewInMemBus()
	runner := NewRunner(bus)

	t.Run("Full Compensation", func(t *testing.T) {
		wf := &Workflow{
			Name: "saga_test",
			Steps: []Step{
				{
					ID:   "step1",
					Uses: "action1",
					Saga: &Saga{Uses: "undo1"},
				},
				{
					ID:   "step2",
					Uses: "action2",
					Saga: &Saga{Uses: "undo2"},
				},
				{
					ID:   "step3",
					Uses: "fail",
				},
			},
		}

		var compensated []string
		var mu sync.Mutex

		runner.RegisterTask("action1", func(ctx context.Context, input any) (any, error) { return "ok", nil })
		runner.RegisterTask("action2", func(ctx context.Context, input any) (any, error) { return "ok", nil })
		runner.RegisterTask("fail", func(ctx context.Context, input any) (any, error) { return nil, errors.New("fail") })

		runner.RegisterTask("undo1", func(ctx context.Context, input any) (any, error) {
			mu.Lock()
			compensated = append(compensated, "undo1")
			mu.Unlock()
			return nil, nil
		})
		runner.RegisterTask("undo2", func(ctx context.Context, input any) (any, error) {
			mu.Lock()
			compensated = append(compensated, "undo2")
			mu.Unlock()
			return nil, nil
		})

		err := runner.Execute(context.Background(), "test-id", wf, nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		if len(compensated) != 2 {
			t.Errorf("expected 2 compensation steps, got %d", len(compensated))
		}
		if compensated[0] != "undo2" || compensated[1] != "undo1" {
			t.Errorf("expected reverse order [undo2, undo1], got %v", compensated)
		}
	})

	t.Run("Partial Compensation", func(t *testing.T) {
		wf := &Workflow{
			Name: "saga_partial",
			Steps: []Step{
				{
					ID:   "step1",
					Uses: "action1",
					Saga: &Saga{Uses: "undo1"},
				},
				{
					ID:   "step2",
					Uses: "fail",
				},
			},
		}

		undoCalled := false
		runner.RegisterTask("action1", func(ctx context.Context, input any) (any, error) { return "ok", nil })
		runner.RegisterTask("fail", func(ctx context.Context, input any) (any, error) { return nil, errors.New("fail") })
		runner.RegisterTask("undo1", func(ctx context.Context, input any) (any, error) {
			undoCalled = true
			return nil, nil
		})

		runner.Execute(context.Background(), "test-id", wf, nil)
		if !undoCalled {
			t.Error("expected undo1 to be called")
		}
	})
	
	t.Run("Saga Handler Error", func(t *testing.T) {
		wf := &Workflow{
			Steps: []Step{
				{ID: "s1", Uses: "a", Saga: &Saga{Uses: "u"}},
				{ID: "s2", Uses: "fail"},
			},
		}
		runner.RegisterTask("a", func(ctx context.Context, i any) (any, error) { return nil, nil })
		runner.RegisterTask("fail", func(ctx context.Context, i any) (any, error) { return nil, errors.New("e") })
		runner.RegisterTask("u", func(ctx context.Context, i any) (any, error) { return nil, errors.New("saga fail") })

		// Should just log and continue
		err := runner.Execute(context.Background(), "test-id", wf, nil)
		if err == nil {
			t.Fatal("expected error from main flow")
		}
	})

	t.Run("Compensate Edge Cases", func(t *testing.T) {
		r := NewRunner(nil)
		// No history
		r.compensate(context.Background(), "test-id", nil, nil)
		
		// Missing saga handler
		history := []Step{{ID: "s1", Saga: &Saga{Uses: "ghost"}}}
		r.compensate(context.Background(), "test-id", history, nil)
	})
}
