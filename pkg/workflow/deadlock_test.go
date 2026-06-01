package workflow

import (
	"context"
	"testing"

	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
)

func TestDeadlock(t *testing.T) {
	bus := eventbus.NewInMemBus()
	runner := NewRunner(bus, nil, nil)

	t.Run("Deadlock", func(t *testing.T) {
		wf := &Workflow{
			Name: "deadlock",
			Steps: []Step{
				{ID: "s1", Uses: "task", DependsOn: []string{"s2"}},
				{ID: "s2", Uses: "task", DependsOn: []string{"s1"}},
			},
		}
		
		runner.RegisterTask("task", func(ctx context.Context, input any) (any, error) { return nil, nil })

		_, err := runner.Execute(context.Background(), "dl1", wf, nil)
		if err == nil {
			t.Fatal("expected deadlock error")
		}
	})
}
