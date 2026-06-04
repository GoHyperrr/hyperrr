package workflow

import (
	"context"
	"testing"

	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
	"github.com/GoHyperrr/mdk"
)

func TestDeadlock(t *testing.T) {
	bus := eventbus.NewInMemBus()
	runner := NewRunner(bus, nil, nil)

	t.Run("Deadlock", func(t *testing.T) {
		wf := mdk.Workflow{
			ID:   "deadlock",
			Name: "deadlock",
			Steps: []mdk.Step{
				{ID: "s1", Uses: "task", DependsOn: []string{"s2"}},
				{ID: "s2", Uses: "task", DependsOn: []string{"s1"}},
			},
		}
		
		_ = runner.Register(wf)
		_ = runner.RegisterHandler("task", func(ctx mdk.StepContext) mdk.StepResult { return mdk.StepResult{} })

		_, err := runner.ExecuteSync(context.Background(), "dl1", "deadlock", nil)
		if err == nil {
			t.Fatal("expected deadlock error")
		}
	})
}
