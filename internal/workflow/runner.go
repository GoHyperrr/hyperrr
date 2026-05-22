package workflow

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
	"github.com/GoHyperrr/hyperrr/pkg/logger"
)

// TaskHandler is a function that executes a workflow step.
type TaskHandler func(ctx context.Context, input any) (any, error)

// Runner executes workflow instances.
type Runner struct {
	bus      eventbus.EventBus
	handlers map[string]TaskHandler
	mu       sync.RWMutex
	waiting  map[string]chan string // workflowID -> signal channel
}

// NewRunner creates a new Runner.
func NewRunner(bus eventbus.EventBus) *Runner {
	return &Runner{
		bus:      bus,
		handlers: make(map[string]TaskHandler),
		waiting:  make(map[string]chan string),
	}
}

// RegisterTask registers a handler for a task type.
func (r *Runner) RegisterTask(name string, handler TaskHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[name] = handler
}

// Execute runs a workflow end-to-end and returns the results map.
func (r *Runner) Execute(ctx context.Context, id string, wf *Workflow, input any) (map[string]any, error) {
	logger.Info("starting workflow", "name", wf.Name, "id", id)
	r.emit(ctx, "workflow.started", map[string]any{
		"id":      id,
		"name":    wf.Name,
		"version": wf.Version,
	})

	completed := make(map[string]bool)
	results := make(map[string]any)
	results["input"] = input
	
	var history []Step

	for len(completed) < len(wf.Steps) {
		madeProgress := false
		for _, step := range wf.Steps {
			if completed[step.ID] {
				continue
			}

			canRun := true
			for _, dep := range step.DependsOn {
				if !completed[dep] {
					canRun = false
					break
				}
			}

			if canRun {
				res, err := r.executeStepWithPolicies(ctx, id, step, results)
				if err != nil {
					logger.Error("workflow execution failed", "id", id, "step", step.ID, "error", err)
					r.emit(ctx, "workflow.failed", map[string]any{"id": id, "error": err.Error()})
					
					r.compensate(ctx, id, history, results)
					return results, fmt.Errorf("workflow %s failed at step %s: %w", id, step.ID, err)
				}

				results[step.ID] = res
				completed[step.ID] = true
				history = append(history, step)
				madeProgress = true
			}
		}

		if !madeProgress {
			return results, fmt.Errorf("deadlock detected in workflow DAG")
		}
	}

	logger.Info("workflow completed", "id", id)
	r.emit(ctx, "workflow.completed", map[string]any{"id": id, "name": wf.Name})

	return results, nil
}

func (r *Runner) executeStepWithPolicies(ctx context.Context, id string, step Step, results map[string]any) (any, error) {
	r.emit(ctx, "workflow.step.started", map[string]any{"id": id, "step_id": step.ID})

	var lastErr error
	maxAttempts := 1
	if step.Retry != nil && step.Retry.MaxAttempts > 0 {
		maxAttempts = step.Retry.MaxAttempts
	}

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			r.emit(ctx, "workflow.step.retrying", map[string]any{
				"id":      id,
				"step_id": step.ID,
				"attempt": attempt,
			})
			r.applyBackoff(step.Retry, attempt)
		}

		r.mu.RLock()
		handler, ok := r.handlers[step.Uses]
		r.mu.RUnlock()

		if !ok {
			return nil, fmt.Errorf("no handler registered for task: %s", step.Uses)
		}

		res, err := handler(ctx, results)
		if err == nil {
			r.emit(ctx, "workflow.step.completed", map[string]any{"id": id, "step_id": step.ID})
			return res, nil
		}

		lastErr = err
	}

	// Try Fallback
	if step.Fallback != nil && step.Fallback.Step != "" {
		r.emit(ctx, "workflow.step.fallback", map[string]any{
			"id":          id,
			"step_id":     step.ID,
			"fallback_to": step.Fallback.Step,
		})
		
		r.mu.RLock()
		fallbackHandler, ok := r.handlers[step.Fallback.Step]
		r.mu.RUnlock()
		
		if ok {
			return fallbackHandler(ctx, results)
		}
	}

	// Try Escalation
	if step.Escalation != nil && step.Escalation.Strategy == "wait_human" {
		logger.Warn("step failed, waiting for human intervention", "id", id, "step", step.ID)
		r.emit(ctx, "workflow.waiting_human", map[string]any{
			"id":      id,
			"step_id": step.ID,
			"reason":  lastErr.Error(),
		})

		ch := make(chan string)
		r.mu.Lock()
		r.waiting[id] = ch
		r.mu.Unlock()

		select {
		case action := <-ch:
			r.mu.Lock()
			delete(r.waiting, id)
			r.mu.Unlock()

			if action == "retry" {
				return r.executeStepWithPolicies(ctx, id, step, results)
			}
			if action == "cancel" {
				return nil, fmt.Errorf("cancelled by operator")
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	r.emit(ctx, "workflow.step.failed", map[string]any{"id": id, "step_id": step.ID, "error": lastErr.Error()})
	return nil, lastErr
}

// ResumeWorkflow signals a paused workflow to continue.
func (r *Runner) ResumeWorkflow(id string, action string) error {
	r.mu.RLock()
	ch, ok := r.waiting[id]
	r.mu.RUnlock()

	if !ok {
		return fmt.Errorf("workflow %s is not waiting for intervention", id)
	}

	ch <- action
	return nil
}

func (r *Runner) applyBackoff(policy *Retry, attempt int) {
	if policy == nil {
		return
	}
	interval := policy.Interval
	if interval == 0 {
		interval = 100 * time.Millisecond
	}
	if policy.Backoff == "exponential" {
		mult := math.Pow(2, float64(attempt-2))
		interval = time.Duration(float64(interval) * mult)
	}
	time.Sleep(interval)
}

func (r *Runner) compensate(ctx context.Context, id string, history []Step, results map[string]any) {
	if len(history) == 0 {
		return
	}
	logger.Info("workflow compensating", "id", id, "steps_count", len(history))
	r.emit(ctx, "workflow.compensating", map[string]any{"id": id, "steps_count": len(history)})
	for i := len(history) - 1; i >= 0; i-- {
		step := history[i]
		if step.Saga == nil || step.Saga.Uses == "" {
			continue
		}
		r.mu.RLock()
		handler, ok := r.handlers[step.Saga.Uses]
		r.mu.RUnlock()
		if ok {
			logger.Info("running compensation task", "id", id, "uses", step.Saga.Uses)
			_, err := handler(ctx, results)
			if err != nil {
				logger.Error("compensation task failed", "id", id, "uses", step.Saga.Uses, "error", err)
			}
		} else {
			logger.Warn("compensation handler not found", "id", id, "uses", step.Saga.Uses)
		}
	}
}

func (r *Runner) emit(ctx context.Context, eventType string, payload any) {
	if r.bus == nil {
		return
	}
	event := eventbus.Event{
		ID:        fmt.Sprintf("evt_%d", time.Now().UnixNano()),
		Type:      eventType,
		Payload:   payload,
		Timestamp: time.Now(),
	}
	r.bus.Publish(ctx, event)
}
