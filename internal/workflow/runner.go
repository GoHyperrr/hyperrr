package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
	"github.com/GoHyperrr/hyperrr/pkg/logger"
	"github.com/google/uuid"
)

// TaskHandler is a function that executes a workflow step.
type TaskHandler func(ctx context.Context, input any) (any, error)

// Runner executes workflow instances.
type Runner struct {
	bus      eventbus.EventBus
	store    StateStore
	handlers map[string]TaskHandler
	mu       sync.RWMutex
	waiting  map[string]chan string // workflowID -> signal channel
}

// NewRunner creates a new Runner.
func NewRunner(bus eventbus.EventBus, store StateStore) *Runner {
	if store == nil {
		store = NewInMemStore() // Fallback for tests not explicitly providing one
	}
	return &Runner{
		bus:      bus,
		store:    store,
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
// Independent steps (branches in the DAG) are executed in parallel.
func (r *Runner) Execute(ctx context.Context, id string, wf *Workflow, input any) (map[string]any, error) {
	logger.Info("starting workflow", "name", wf.Name, "id", id)
	r.emit(ctx, EventWorkflowStarted, map[string]any{
		"id":      id,
		"name":    wf.Name,
		"version": wf.Version,
	})

	results := make(map[string]any)
	results["input"] = input
	results["_workflow_id"] = id

	// Checkpoint initial input and initialize execution state
	if r.store != nil {
		if inputBytes, err := json.Marshal(input); err == nil {
			_ = r.store.InitializeExecution(ctx, id, inputBytes)
		}
	}

	completed := make(map[string]bool)
	launched := make(map[string]bool)
	var history []Step
	var stateMu sync.Mutex

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	stepFinished := make(chan error, len(wf.Steps))
	activeCount := 0

	for len(completed) < len(wf.Steps) {
		// 1. Identify and launch all ready steps
		stateMu.Lock()
		var ready []Step
		for _, step := range wf.Steps {
			if launched[step.ID] {
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
				ready = append(ready, step)
				launched[step.ID] = true
			}
		}

		if len(ready) == 0 && activeCount == 0 {
			stateMu.Unlock()
			return results, fmt.Errorf("deadlock detected in workflow DAG")
		}

		for _, step := range ready {
			activeCount++

			// Snapshot results for this step to prevent data races in handlers.
			// Each step only sees results of its completed dependencies.
			inputCopy := make(map[string]any)
			for k, v := range results {
				inputCopy[k] = v
			}

			go func(s Step, inp map[string]any) {
				stepCtx := WithRunner(ctx, r, id)
				
				if r.store != nil {
					_ = r.store.SaveState(stepCtx, id, s.ID, StateRunning)
				}
				
				res, err := r.executeStepWithPolicies(stepCtx, id, s, inp)

				if err != nil {
					if r.store != nil {
						_ = r.store.SaveState(stepCtx, id, s.ID, StateFailed)
					}
					stepFinished <- fmt.Errorf("workflow %s failed at step %s: %w", id, s.ID, err)
					return
				}

				if r.store != nil {
					if resBytes, err := json.Marshal(res); err == nil {
						_ = r.store.SaveStepOutput(stepCtx, id, s.ID, resBytes)
					}
					_ = r.store.SaveState(stepCtx, id, s.ID, StateCompleted)
				}

				stateMu.Lock()
				results[s.ID] = res
				completed[s.ID] = true
				history = append(history, s)
				stateMu.Unlock()

				stepFinished <- nil
			}(step, inputCopy)
		}
		stateMu.Unlock()

		// 2. Wait for at least one step to finish or context failure
		select {
		case err := <-stepFinished:
			activeCount--
			if err != nil {
				cancel()
				// Drain active goroutines
				for activeCount > 0 {
					<-stepFinished
					activeCount--
				}

				logger.Error("workflow execution failed", "id", id, "error", err)
				r.emit(ctx, EventWorkflowFailed, map[string]any{"id": id, "error": err.Error()})
				r.compensate(context.WithoutCancel(ctx), id, history, results)
				return results, err
			}
		case <-ctx.Done():
			// External cancellation
			for activeCount > 0 {
				<-stepFinished
				activeCount--
			}
			return results, ctx.Err()
		}
	}

	logger.Info("workflow completed", "id", id)
	r.emit(ctx, EventWorkflowCompleted, map[string]any{"id": id, "name": wf.Name})

	return results, nil
}

// ResumeExecution resumes a previously crashed or interrupted workflow.
func (r *Runner) ResumeExecution(ctx context.Context, id string, wf *Workflow) (map[string]any, error) {
	if r.store == nil {
		return nil, fmt.Errorf("cannot resume workflow without a configured StateStore")
	}

	logger.Info("resuming workflow", "name", wf.Name, "id", id)

	// 1. Retrieve initial input
	inputBytes, err := r.store.GetInput(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve workflow input: %w", err)
	}

	var input any
	if err := json.Unmarshal(inputBytes, &input); err != nil {
		return nil, fmt.Errorf("failed to unmarshal workflow input: %w", err)
	}

	// 2. Retrieve execution state
	states, err := r.store.GetState(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve workflow state: %w", err)
	}

	results := make(map[string]any)
	results["input"] = input
	results["_workflow_id"] = id

	completed := make(map[string]bool)
	launched := make(map[string]bool)
	var history []Step
	var stateMu sync.Mutex

	// Pre-fill results and completed states from Store
	for _, step := range wf.Steps {
		if state, ok := states[step.ID]; ok {
			if state == StateCompleted {
				outBytes, err := r.store.GetStepOutput(ctx, id, step.ID)
				if err == nil && len(outBytes) > 0 {
					var out any
					if err := json.Unmarshal(outBytes, &out); err == nil {
						results[step.ID] = out
					}
				}
				completed[step.ID] = true
				launched[step.ID] = true
				history = append(history, step)
			} else if state == StateRunning || state == StateFailed {
				// Mark as launched but not completed, allowing the main loop to pick it up for re-execution
				launched[step.ID] = false // Setting to false forces identification in the loop as "ready" if dependencies met
			}
		}
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	stepFinished := make(chan error, len(wf.Steps))
	activeCount := 0

	for len(completed) < len(wf.Steps) {
		// 1. Identify and launch all ready steps
		stateMu.Lock()
		var ready []Step
		for _, step := range wf.Steps {
			if launched[step.ID] {
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
				ready = append(ready, step)
				launched[step.ID] = true
			}
		}

		if len(ready) == 0 && activeCount == 0 {
			stateMu.Unlock()
			return results, fmt.Errorf("deadlock detected in workflow DAG")
		}

		for _, step := range ready {
			activeCount++

			inputCopy := make(map[string]any)
			for k, v := range results {
				inputCopy[k] = v
			}

			go func(s Step, inp map[string]any) {
				_ = r.store.SaveState(ctx, id, s.ID, StateRunning)
				
				res, err := r.executeStepWithPolicies(ctx, id, s, inp)

				if err != nil {
					_ = r.store.SaveState(ctx, id, s.ID, StateFailed)
					stepFinished <- fmt.Errorf("workflow %s failed at step %s: %w", id, s.ID, err)
					return
				}

				if resBytes, err := json.Marshal(res); err == nil {
					_ = r.store.SaveStepOutput(ctx, id, s.ID, resBytes)
				}
				_ = r.store.SaveState(ctx, id, s.ID, StateCompleted)

				stateMu.Lock()
				results[s.ID] = res
				completed[s.ID] = true
				history = append(history, s)
				stateMu.Unlock()

				stepFinished <- nil
			}(step, inputCopy)
		}
		stateMu.Unlock()

		select {
		case err := <-stepFinished:
			activeCount--
			if err != nil {
				cancel()
				for activeCount > 0 {
					<-stepFinished
					activeCount--
				}
				logger.Error("workflow execution failed", "id", id, "error", err)
				r.emit(ctx, EventWorkflowFailed, map[string]any{"id": id, "error": err.Error()})
				r.compensate(context.WithoutCancel(ctx), id, history, results)
				return results, err
			}
		case <-ctx.Done():
			for activeCount > 0 {
				<-stepFinished
				activeCount--
			}
			return results, ctx.Err()
		}
	}

	logger.Info("workflow resumed and completed", "id", id)
	r.emit(ctx, EventWorkflowCompleted, map[string]any{"id": id, "name": wf.Name})

	return results, nil
}

func (r *Runner) executeStepWithPolicies(ctx context.Context, id string, step Step, results map[string]any) (any, error) {
	r.emit(ctx, EventStepStarted, map[string]any{"id": id, "step_id": step.ID})

	var lastErr error
	maxAttempts := 1
	if step.Retry != nil && step.Retry.MaxAttempts > 0 {
		maxAttempts = step.Retry.MaxAttempts
	}

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			r.emit(ctx, EventStepRetrying, map[string]any{
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
			r.emit(ctx, EventStepCompleted, map[string]any{"id": id, "step_id": step.ID})
			return res, nil
		}

		lastErr = err
	}

	// Try Fallback
	if step.Fallback != nil && step.Fallback.Step != "" {
		r.emit(ctx, EventStepFallback, map[string]any{
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
	if step.Escalation != nil && step.Escalation.Strategy == EscalationWaitHuman {
		logger.Warn("step failed, waiting for human intervention", "id", id, "step", step.ID)
		r.emit(ctx, EventWaitingHuman, map[string]any{
			"id":      id,
			"step_id": step.ID,
			"reason":  lastErr.Error(),
		})

		ch := make(chan string, 1)
		r.mu.Lock()
		r.waiting[id] = ch
		r.mu.Unlock()

		defer func() {
			r.mu.Lock()
			delete(r.waiting, id)
			r.mu.Unlock()
		}()

		select {
		case action := <-ch:
			if action == ActionRetry {
				return r.executeStepWithPolicies(ctx, id, step, results)
			}
			if action == ActionCancel {
				return nil, fmt.Errorf("cancelled by operator")
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	r.emit(ctx, EventStepFailed, map[string]any{"id": id, "step_id": step.ID, "error": lastErr.Error()})
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

	select {
	case ch <- action:
		return nil
	case <-time.After(500 * time.Millisecond):
		return fmt.Errorf("timeout signaling workflow %s - channel might be abandoned", id)
	}
}

func (r *Runner) applyBackoff(policy *Retry, attempt int) {
	if policy == nil {
		return
	}
	interval := policy.Interval
	if interval == 0 {
		interval = 100 * time.Millisecond
	}
	if policy.Backoff == BackoffExponential {
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
	r.emit(ctx, EventWorkflowCompensating, map[string]any{"id": id, "steps_count": len(history)})
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
				if step.Saga.IsCritical {
					logger.Error("CRITICAL COMPENSATION FAILED", "id", id, "uses", step.Saga.Uses, "error", err)
				} else {
					logger.Error("compensation task failed", "id", id, "uses", step.Saga.Uses, "error", err)
				}
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
		ID:        "evt_" + uuid.New().String(),
		Type:      eventType,
		Payload:   payload,
		Timestamp: time.Now(),
	}
	if err := r.bus.Publish(ctx, event); err != nil {
		logger.Error("failed to publish workflow event", "type", eventType, "error", err)
	}
}
