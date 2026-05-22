package workflow

import (
	"context"
	"testing"

	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
)

func TestRunner(t *testing.T) {
	bus := eventbus.NewInMemBus()
	runner := NewRunner(bus)

	runner.RegisterTask("step1", func(ctx context.Context, input any) (any, error) {
		return "res1", nil
	})

	runner.RegisterTask("step2", func(ctx context.Context, input any) (any, error) {
		data := input.(map[string]any)
		prev := data["s1"].(string)
		return prev + "_res2", nil
	})

	t.Run("Sequential Execution", func(t *testing.T) {
		wf := &Workflow{
			Name: "test-wf",
			Steps: []Step{
				{ID: "s1", Uses: "step1"},
				{ID: "s2", Uses: "step2", DependsOn: []string{"s1"}},
			},
		}

		res, err := runner.Execute(context.Background(), "wf1", wf, nil)
		if err != nil {
			t.Fatalf("workflow failed: %v", err)
		}

		if res["s1"] != "res1" {
			t.Errorf("expected res1, got %v", res["s1"])
		}
		if res["s2"] != "res1_res2" {
			t.Errorf("expected res1_res2, got %v", res["s2"])
		}
	})

	t.Run("Event Emission", func(t *testing.T) {
		events := make(chan eventbus.Event, 10)
		bus.Subscribe(context.Background(), "workflow.completed", func(ctx context.Context, ev eventbus.Event) error {
			events <- ev
			return nil
		})

		wf := &Workflow{
			Steps: []Step{{ID: "s1", Uses: "step1"}},
		}

		_, err := runner.Execute(context.Background(), "wf2", wf, nil)
		if err != nil {
			t.Fatal(err)
		}

		select {
		case ev := <-events:
			payload := ev.Payload.(map[string]any)
			if payload["id"] != "wf2" {
				t.Error("wrong event payload")
			}
		default:
			t.Error("event not emitted")
		}
	})

	t.Run("Emit with nil bus", func(t *testing.T) {
		r := &Runner{}
		r.emit(context.Background(), "test", nil) // Should not panic
	})
}
