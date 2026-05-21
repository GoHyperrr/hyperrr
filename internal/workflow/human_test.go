package workflow

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
)

func TestHumanIntervention(t *testing.T) {
	bus := eventbus.NewInMemBus()
	runner := NewRunner(bus)

	t.Run("Resume Workflow Success", func(t *testing.T) {
		wf := &Workflow{
			Name: "human",
			Steps: []Step{
				{
					ID:   "s1",
					Uses: "unstable",
					Escalation: &Escalation{
						Strategy: "wait_human",
					},
				},
			},
		}

		attempts := 0
		runner.RegisterTask("unstable", func(ctx context.Context, input any) (any, error) {
			attempts++
			if attempts == 1 {
				return nil, errors.New("need help")
			}
			return "ok", nil
		})

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		workflowID := "wf_human_1"
		errChan := make(chan error, 1)
		go func() {
			errChan <- runner.Execute(ctx, workflowID, wf, nil)
		}()

		// Wait a bit for it to enter WAITING_HUMAN
		time.Sleep(100 * time.Millisecond)

		// Resume it
		err := runner.ResumeWorkflow(workflowID, "retry")
		if err != nil {
			t.Fatalf("failed to resume: %v", err)
		}

		err = <-errChan
		if err != nil {
			t.Fatalf("workflow failed after resume: %v", err)
		}

		if attempts != 2 {
			t.Errorf("expected 2 attempts, got %d", attempts)
		}
	})

	t.Run("Cancel Workflow Success", func(t *testing.T) {
		wf := &Workflow{
			Steps: []Step{
				{ID: "s1", Uses: "fail", Escalation: &Escalation{Strategy: "wait_human"}},
			},
		}
		runner.RegisterTask("fail", func(ctx context.Context, i any) (any, error) { return nil, errors.New("e") })

		workflowID := "wf_human_2"
		errChan := make(chan error, 1)
		go func() {
			errChan <- runner.Execute(context.Background(), workflowID, wf, nil)
		}()

		time.Sleep(50 * time.Millisecond)
		runner.ResumeWorkflow(workflowID, "cancel")

		err := <-errChan
		if err == nil || err.Error() != "workflow wf_human_2 failed at step s1: cancelled by operator" {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("Resume Non-existent Workflow", func(t *testing.T) {
		err := runner.ResumeWorkflow("ghost", "retry")
		if err == nil {
			t.Fatal("expected error for non-existent workflow, got nil")
		}
	})
}
