package workflow

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
)

func TestRunner(t *testing.T) {
	bus := eventbus.NewInMemBus()
	runner := NewRunner(bus)

	t.Run("Execute Success", func(t *testing.T) {
		wf := &Workflow{
			Name: "test",
			Steps: []Step{
				{ID: "step1", Uses: "task1"},
				{ID: "step2", Uses: "task2", DependsOn: []string{"step1"}},
			},
		}

		var order []string
		var mu sync.Mutex

		runner.RegisterTask("task1", func(ctx context.Context, input any) (any, error) {
			mu.Lock()
			order = append(order, "step1")
			mu.Unlock()
			return "res1", nil
		})
		runner.RegisterTask("task2", func(ctx context.Context, input any) (any, error) {
			mu.Lock()
			order = append(order, "step2")
			mu.Unlock()
			return "res2", nil
		})

		err := runner.Execute(context.Background(), "test-id", wf, nil)
		if err != nil {
			t.Fatalf("expected success, got %v", err)
		}

		if len(order) != 2 || order[0] != "step1" || order[1] != "step2" {
			t.Errorf("unexpected execution order: %v", order)
		}
	})

	t.Run("Step Failure", func(t *testing.T) {
		wf := &Workflow{
			Name: "fail",
			Steps: []Step{
				{ID: "s1", Uses: "fail_task"},
			},
		}

		runner.RegisterTask("fail_task", func(ctx context.Context, input any) (any, error) {
			return nil, errors.New("boom")
		})

		err := runner.Execute(context.Background(), "test-id", wf, nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("Missing Task Handler", func(t *testing.T) {
		wf := &Workflow{
			Name: "missing",
			Steps: []Step{
				{ID: "s1", Uses: "ghost"},
			},
		}

		err := runner.Execute(context.Background(), "test-id", wf, nil)
		if err == nil {
			t.Fatal("expected error for missing handler, got nil")
		}
	})

	t.Run("Emit Failure", func(t *testing.T) {
		failBus := &failEventBus{}
		r := NewRunner(failBus)
		// emit failure should just log, not crash
		r.emit(context.Background(), "test", nil)
	})

	t.Run("Event Emission", func(t *testing.T) {
		received := make(chan string, 10)
		bus := eventbus.NewInMemBus()
		bus.Subscribe(context.Background(), "workflow.started", func(ctx context.Context, e eventbus.Event) error {
			received <- e.Type
			return nil
		})

		r := NewRunner(bus)
		wf := &Workflow{Name: "ev", Steps: []Step{{ID: "s1", Uses: "t"}}}
		r.RegisterTask("t", func(ctx context.Context, i any) (any, error) { return nil, nil })
		
		r.Execute(context.Background(), "test-id", wf, nil)

		select {
		case type_ := <-received:
			if type_ != "workflow.started" {
				t.Errorf("expected workflow.started, got %s", type_)
			}
		case <-time.After(100 * time.Millisecond):
			t.Error("timed out waiting for event")
		}
	})
}

type failEventBus struct {
	eventbus.EventBus
}

func (f *failEventBus) Publish(ctx context.Context, e eventbus.Event) error {
	return errors.New("publish failed")
}
