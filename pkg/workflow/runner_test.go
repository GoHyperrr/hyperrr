package workflow

import (
	"context"
	"testing"

	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
	"github.com/GoHyperrr/mdk"
)

func TestRunner(t *testing.T) {
	bus := eventbus.NewInMemBus()
	runner := NewRunner(bus, nil, nil)

	_ = runner.RegisterHandler("step1", func(sCtx mdk.StepContext) mdk.StepResult {
		return mdk.StepResult{Output: map[string]any{"res": "res1"}}
	})

	_ = runner.RegisterHandler("step2", func(sCtx mdk.StepContext) mdk.StepResult {
		prev := sCtx.Input["s1"].(map[string]any)["res"].(string)
		return mdk.StepResult{Output: map[string]any{"res": prev + "_res2"}}
	})

	t.Run("Sequential Execution", func(t *testing.T) {
		wf := &Workflow{
			ID:   "test-wf",
			Name: "test-wf",
			Steps: []Step{
				{ID: "s1", Uses: "step1"},
				{ID: "s2", Uses: "step2", DependsOn: []string{"s1"}},
			},
		}

		res, err := runner.ExecuteSyncWorkflow(context.Background(), "wf1", wf, nil)
		if err != nil {
			t.Fatalf("workflow failed: %v", err)
		}

		s1Res, ok1 := res["s1"].(map[string]any)
		s2Res, ok2 := res["s2"].(map[string]any)
		if !ok1 || s1Res["res"] != "res1" {
			t.Errorf("expected res1, got %v", res["s1"])
		}
		if !ok2 || s2Res["res"] != "res1_res2" {
			t.Errorf("expected res1_res2, got %v", res["s2"])
		}
	})

	t.Run("Event Emission", func(t *testing.T) {
		events := make(chan mdk.Event, 10)
		unsub, _ := bus.Subscribe("workflow", "completed", func(ctx context.Context, ev mdk.Event) error {
			events <- ev
			return nil
		})
		defer unsub()

		wf := &Workflow{
			ID:   "wf2",
			Name: "wf2",
			Steps: []Step{{ID: "s1", Uses: "step1"}},
		}

		_, err := runner.ExecuteSyncWorkflow(context.Background(), "wf2", wf, nil)
		if err != nil {
			t.Fatal(err)
		}

		select {
		case ev := <-events:
			if ev.Payload["id"] != "wf2" {
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

	t.Run("GetStateStore", func(t *testing.T) {
		if runner.GetStateStore() == nil {
			t.Error("expected state store to not be nil")
		}
	})
}
