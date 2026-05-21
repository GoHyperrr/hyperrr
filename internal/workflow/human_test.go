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

	t.Run("Resume Success", func(t *testing.T) {
		workflowID := "wf-h1"
		attempts := 0
		
		runner.RegisterTask("wait-task", func(ctx context.Context, input any) (any, error) {
			attempts++
			if attempts == 1 {
				return nil, errors.New("need help")
			}
			return "fixed", nil
		})

		wf := &Workflow{
			Steps: []Step{
				{
					ID: "s1",
					Uses: "wait-task",
					Escalation: &Escalation{Strategy: "wait_human"},
				},
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		errChan := make(chan error)
		resChan := make(chan map[string]any)

		go func() {
			res, err := runner.Execute(ctx, workflowID, wf, nil)
			if err != nil {
				errChan <- err
			} else {
				resChan <- res
			}
		}()

		// Wait for it to be waiting
		time.Sleep(50 * time.Millisecond)
		err := runner.ResumeWorkflow(workflowID, "retry")
		if err != nil {
			t.Fatalf("failed to resume: %v", err)
		}

		select {
		case err := <-errChan:
			t.Fatalf("workflow failed: %v", err)
		case res := <-resChan:
			if res["s1"] != "fixed" {
				t.Errorf("expected fixed, got %v", res["s1"])
			}
		case <-ctx.Done():
			t.Fatal("timeout waiting for workflow")
		}
	})

	t.Run("Cancel Workflow", func(t *testing.T) {
		workflowID := "wf-h2"
		runner.RegisterTask("fail-task", func(ctx context.Context, input any) (any, error) {
			return nil, errors.New("help")
		})

		wf := &Workflow{
			Steps: []Step{
				{
					ID: "s1",
					Uses: "fail-task",
					Escalation: &Escalation{Strategy: "wait_human"},
				},
			},
		}

		go func() {
			time.Sleep(50 * time.Millisecond)
			runner.ResumeWorkflow(workflowID, "cancel")
		}()

		_, err := runner.Execute(context.Background(), workflowID, wf, nil)
		if err == nil || err.Error() != "workflow wf-h2 failed at step s1: cancelled by operator" {
			t.Errorf("expected cancellation error, got %v", err)
		}
	})
}
