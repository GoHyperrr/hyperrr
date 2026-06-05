package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/GoHyperrr/hyperrr/pkg/locking"
	"github.com/GoHyperrr/hyperrr/pkg/logger"
	"github.com/GoHyperrr/mdk"
	"github.com/google/uuid"
)

// Runner executes workflow instances.
type Runner struct {
	bus       mdk.EventBus
	store     StateStore
	locker    locking.Locker
	workflows map[string]mdk.Workflow
	cancels   map[string]context.CancelFunc
	waiting   map[string]chan string // runID -> signal channel
	runtime   mdk.Runtime
	handlers  map[string]mdk.StepHandler
	mu        sync.RWMutex
}

// NewRunner creates a new Runner.
func NewRunner(bus mdk.EventBus, store StateStore, locker locking.Locker) *Runner {
	if store == nil {
		store = NewInMemStore()
	}
	if locker == nil {
		locker = locking.NewInMemLocker()
	}
	return &Runner{
		bus:       bus,
		store:     store,
		locker:    locker,
		workflows: make(map[string]mdk.Workflow),
		cancels:   make(map[string]context.CancelFunc),
		waiting:   make(map[string]chan string),
		handlers:  make(map[string]mdk.StepHandler),
	}
}

// SetRuntime sets the active mdk.Runtime for execution context.
func (r *Runner) SetRuntime(rt mdk.Runtime) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.runtime = rt
}

// Register registers a workflow definition.
func (r *Runner) Register(w mdk.Workflow) error {
	if w.ID == "" {
		return fmt.Errorf("workflow ID cannot be empty")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.workflows[w.ID] = w
	return nil
}

// RegisterHandler registers a named step handler.
func (r *Runner) RegisterHandler(name string, handler mdk.StepHandler) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[name] = handler
	return nil
}

// Execute starts an asynchronous workflow run and returns the run ID.
func (r *Runner) Execute(ctx context.Context, workflowID string, input map[string]any) (string, error) {
	runID := "wf_run_" + uuid.New().String()
	go func() {
		_, _ = r.ExecuteSync(context.Background(), runID, workflowID, input)
	}()
	return runID, nil
}

// GetStateStore returns the StateStore attached to the runner.
func (r *Runner) GetStateStore() StateStore {
	return r.store
}

// ExecuteSyncWorkflow runs a workflow synchronously and returns the results map.
func (r *Runner) ExecuteSyncWorkflow(ctx context.Context, id string, wf *Workflow, input any) (map[string]any, error) {
	logger.Info("starting workflow", "name", wf.Name, "id", id)
	r.emit(ctx, EventWorkflowStarted, map[string]any{
		"id":   id,
		"name": wf.Name,
	})

	inputMap, ok := input.(map[string]any)
	if !ok {
		if bytes, err := json.Marshal(input); err == nil {
			_ = json.Unmarshal(bytes, &inputMap)
		}
	}
	if inputMap == nil {
		inputMap = make(map[string]any)
	}

	results := make(map[string]any)
	for k, v := range inputMap {
		results[k] = v
	}
	results["input"] = inputMap
	results["_workflow_id"] = id

	if r.store != nil {
		if inputBytes, err := json.Marshal(inputMap); err == nil {
			_ = r.store.InitializeExecution(ctx, id, inputBytes)
			_ = r.store.SaveState(ctx, id, "__wf_name", wf.Name)
			_ = r.store.SaveState(ctx, id, "", StateRunning)
		}
	}

	completed := make(map[string]bool)
	launched := make(map[string]bool)
	var history []mdk.Step
	var stateMu sync.Mutex

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	r.mu.Lock()
	r.cancels[id] = cancel
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		delete(r.cancels, id)
		r.mu.Unlock()
	}()

	stepFinished := make(chan error, len(wf.Steps))
	activeCount := 0

	for len(completed) < len(wf.Steps) {
		stateMu.Lock()
		var ready []mdk.Step
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

			go func(s mdk.Step, inp map[string]any) {
				stepCtx := ctx
				if r.store != nil {
					_ = r.store.SaveState(stepCtx, id, s.ID, StateRunning)
				}

				r.emit(stepCtx, EventStepStarted, map[string]any{"id": id, "step_id": s.ID})

				sCtx := mdk.StepContext{
					Ctx:        stepCtx,
					Runtime:    r.runtime,
					WorkflowID: wf.ID,
					RunID:      id,
					StepID:     s.ID,
					Input:      inp,
				}

				var res mdk.StepResult
				maxAttempts := 1 + s.MaxRetries
				for attempt := 1; attempt <= maxAttempts; attempt++ {
					if attempt > 1 {
						r.emit(stepCtx, EventStepRetrying, map[string]any{
							"id":      id,
							"step_id": s.ID,
							"attempt": attempt,
						})
						time.Sleep(100 * time.Millisecond)
					}

					var handler mdk.StepHandler
					if s.Uses != "" {
						r.mu.RLock()
						h, ok := r.handlers[s.Uses]
						r.mu.RUnlock()
						if ok {
							handler = h
						}
					}

					if handler == nil {
						res = mdk.StepResult{Err: fmt.Errorf("no step handler found for step %s (uses %s)", s.ID, s.Uses)}
					} else {
						res = handler(sCtx)
					}
					if res.Err == nil {
						break
					}
				}

				if res.Err != nil {
					if r.store != nil {
						_ = r.store.SaveState(stepCtx, id, s.ID, StateFailed)
					}
					r.emit(stepCtx, EventStepFailed, map[string]any{"id": id, "step_id": s.ID, "error": res.Err.Error()})
					stepFinished <- fmt.Errorf("workflow %s failed at step %s: %w", id, s.ID, res.Err)
					return
				}

				if r.store != nil {
					if resBytes, err := json.Marshal(res.Output); err == nil {
						_ = r.store.SaveStepOutput(stepCtx, id, s.ID, resBytes)
					}
					_ = r.store.SaveState(stepCtx, id, s.ID, StateCompleted)
				}

				r.emit(stepCtx, EventStepCompleted, map[string]any{"id": id, "step_id": s.ID})

				stateMu.Lock()
				for k, v := range res.Output {
					results[k] = v
				}
				results[s.ID] = res.Output
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
				if r.store != nil {
					_ = r.store.SaveState(context.Background(), id, "", StateFailed)
				}
				return results, err
			}
		case <-ctx.Done():
			for activeCount > 0 {
				<-stepFinished
				activeCount--
			}
			if r.store != nil {
				_ = r.store.SaveState(context.Background(), id, "", StateFailed)
			}
			return results, ctx.Err()
		}
	}

	logger.Info("workflow completed", "id", id)
	if r.store != nil {
		_ = r.store.SaveState(ctx, id, "", StateCompleted)
	}
	r.emit(ctx, EventWorkflowCompleted, map[string]any{"id": id, "name": wf.Name})

	return results, nil
}

// ResumeExecution resumes a previously crashed or interrupted workflow.
func (r *Runner) ResumeExecution(ctx context.Context, id string, wf *Workflow) (map[string]any, error) {
	if r.store == nil {
		return nil, fmt.Errorf("cannot resume workflow without a configured StateStore")
	}

	logger.Info("resuming workflow", "name", wf.Name, "id", id)

	inputBytes, err := r.store.GetInput(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve workflow input: %w", err)
	}

	var input map[string]any
	if err := json.Unmarshal(inputBytes, &input); err != nil {
		return nil, fmt.Errorf("failed to unmarshal workflow input: %w", err)
	}

	states, err := r.store.GetState(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve workflow state: %w", err)
	}

	results := make(map[string]any)
	for k, v := range input {
		results[k] = v
	}
	results["input"] = input
	results["_workflow_id"] = id

	completed := make(map[string]bool)
	launched := make(map[string]bool)
	var history []mdk.Step
	var stateMu sync.Mutex

	for _, step := range wf.Steps {
		if state, ok := states[step.ID]; ok {
			if state == StateCompleted {
				outBytes, err := r.store.GetStepOutput(ctx, id, step.ID)
				if err != nil {
					return nil, fmt.Errorf("failed to retrieve output for completed step %s: %w", step.ID, err)
				}
				if len(outBytes) > 0 {
					var out any
					if err := json.Unmarshal(outBytes, &out); err != nil {
						return nil, fmt.Errorf("failed to unmarshal output for completed step %s: %w", step.ID, err)
					}
					results[step.ID] = out
					if outMap, ok := out.(map[string]any); ok {
						for k, v := range outMap {
							results[k] = v
						}
					}
				}
				completed[step.ID] = true
				launched[step.ID] = true
				history = append(history, step)
			} else if state == StateRunning || state == StateFailed {
				launched[step.ID] = false
			}
		}
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	r.mu.Lock()
	r.cancels[id] = cancel
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		delete(r.cancels, id)
		r.mu.Unlock()
	}()

	stepFinished := make(chan error, len(wf.Steps))
	activeCount := 0

	for len(completed) < len(wf.Steps) {
		stateMu.Lock()
		var ready []mdk.Step
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

			go func(s mdk.Step, inp map[string]any) {
				stepCtx := ctx
				if r.store != nil {
					_ = r.store.SaveState(stepCtx, id, s.ID, StateRunning)
				}

				r.emit(stepCtx, EventStepStarted, map[string]any{"id": id, "step_id": s.ID})

				sCtx := mdk.StepContext{
					Ctx:        stepCtx,
					Runtime:    r.runtime,
					WorkflowID: wf.ID,
					RunID:      id,
					StepID:     s.ID,
					Input:      inp,
				}

				var res mdk.StepResult
				maxAttempts := 1 + s.MaxRetries
				for attempt := 1; attempt <= maxAttempts; attempt++ {
					if attempt > 1 {
						r.emit(stepCtx, EventStepRetrying, map[string]any{
							"id":      id,
							"step_id": s.ID,
							"attempt": attempt,
						})
						time.Sleep(100 * time.Millisecond)
					}

					var handler mdk.StepHandler
					if s.Uses != "" {
						r.mu.RLock()
						h, ok := r.handlers[s.Uses]
						r.mu.RUnlock()
						if ok {
							handler = h
						}
					}

					if handler == nil {
						res = mdk.StepResult{Err: fmt.Errorf("no step handler found for step %s (uses %s)", s.ID, s.Uses)}
					} else {
						res = handler(sCtx)
					}
					if res.Err == nil {
						break
					}
				}

				if res.Err != nil {
					if r.store != nil {
						_ = r.store.SaveState(stepCtx, id, s.ID, StateFailed)
					}
					r.emit(stepCtx, EventStepFailed, map[string]any{"id": id, "step_id": s.ID, "error": res.Err.Error()})
					stepFinished <- fmt.Errorf("workflow %s failed at step %s: %w", id, s.ID, res.Err)
					return
				}

				if r.store != nil {
					if resBytes, err := json.Marshal(res.Output); err == nil {
						_ = r.store.SaveStepOutput(stepCtx, id, s.ID, resBytes)
					}
					_ = r.store.SaveState(stepCtx, id, s.ID, StateCompleted)
				}

				r.emit(stepCtx, EventStepCompleted, map[string]any{"id": id, "step_id": s.ID})

				stateMu.Lock()
				for k, v := range res.Output {
					results[k] = v
				}
				results[s.ID] = res.Output
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
				if r.store != nil {
					_ = r.store.SaveState(context.Background(), id, "", StateFailed)
				}
				return results, err
			}
		case <-ctx.Done():
			for activeCount > 0 {
				<-stepFinished
				activeCount--
			}
			if r.store != nil {
				_ = r.store.SaveState(context.Background(), id, "", StateFailed)
			}
			return results, ctx.Err()
		}
	}

	logger.Info("workflow completed", "id", id)
	if r.store != nil {
		_ = r.store.SaveState(ctx, id, "", StateCompleted)
	}
	r.emit(ctx, EventWorkflowCompleted, map[string]any{"id": id, "name": wf.Name})

	return results, nil
}

// Status returns the current execution status of a run.
func (r *Runner) Status(ctx context.Context, runID string) (mdk.StepStatus, error) {
	if r.store == nil {
		return mdk.StepPending, nil
	}
	states, err := r.store.GetState(ctx, runID)
	if err != nil {
		return mdk.StepFailed, err
	}
	state, ok := states[""]
	if !ok {
		return mdk.StepPending, nil
	}
	switch state {
	case "COMPLETED":
		return mdk.StepCompleted, nil
	case "FAILED":
		return mdk.StepFailed, nil
	case "RUNNING":
		return mdk.StepRunning, nil
	default:
		return mdk.StepPending, nil
	}
}

// Cancel cancels a running workflow.
func (r *Runner) Cancel(ctx context.Context, runID string) error {
	r.mu.Lock()
	cancel, ok := r.cancels[runID]
	r.mu.Unlock()
	if ok {
		cancel()
	}
	return nil
}

// ExecuteSync runs a workflow synchronously and returns the output results map.
func (r *Runner) ExecuteSync(ctx context.Context, id string, workflowID string, input map[string]any) (map[string]any, error) {
	r.mu.RLock()
	wf, ok := r.workflows[workflowID]
	r.mu.RUnlock()
	if !ok {
		return nil, mdk.ErrWorkflowNotFound
	}

	logger.Info("starting workflow", "name", wf.Name, "id", id)
	r.emit(ctx, EventWorkflowStarted, map[string]any{
		"id":   id,
		"name": wf.Name,
	})

	results := make(map[string]any)
	for k, v := range input {
		results[k] = v
	}
	results["input"] = input
	results["_workflow_id"] = id

	if r.store != nil {
		if inputBytes, err := json.Marshal(input); err == nil {
			_ = r.store.InitializeExecution(ctx, id, inputBytes)
			_ = r.store.SaveState(ctx, id, "", StateRunning)
		}
	}

	completed := make(map[string]bool)
	launched := make(map[string]bool)
	var history []mdk.Step
	var stateMu sync.Mutex

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	r.mu.Lock()
	r.cancels[id] = cancel
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		delete(r.cancels, id)
		r.mu.Unlock()
	}()

	stepFinished := make(chan error, len(wf.Steps))
	activeCount := 0

	for len(completed) < len(wf.Steps) {
		stateMu.Lock()
		var ready []mdk.Step
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

			go func(s mdk.Step, inp map[string]any) {
				stepCtx := ctx
				if r.store != nil {
					_ = r.store.SaveState(stepCtx, id, s.ID, StateRunning)
				}

				r.emit(stepCtx, EventStepStarted, map[string]any{"id": id, "step_id": s.ID})

				sCtx := mdk.StepContext{
					Ctx:        stepCtx,
					Runtime:    r.runtime,
					WorkflowID: workflowID,
					RunID:      id,
					StepID:     s.ID,
					Input:      inp,
				}

				var res mdk.StepResult
				maxAttempts := 1 + s.MaxRetries
				for attempt := 1; attempt <= maxAttempts; attempt++ {
					if attempt > 1 {
						r.emit(stepCtx, EventStepRetrying, map[string]any{
							"id":      id,
							"step_id": s.ID,
							"attempt": attempt,
						})
						time.Sleep(100 * time.Millisecond) // Static backoff
					}

					var handler mdk.StepHandler
					if s.Uses != "" {
						r.mu.RLock()
						h, ok := r.handlers[s.Uses]
						r.mu.RUnlock()
						if ok {
							handler = h
						}
					}

					if handler == nil {
						res = mdk.StepResult{Err: fmt.Errorf("no step handler found for step %s (uses %s)", s.ID, s.Uses)}
					} else {
						res = handler(sCtx)
					}
					if res.Err == nil {
						break
					}
				}

				if res.Err != nil {
					if r.store != nil {
						_ = r.store.SaveState(stepCtx, id, s.ID, StateFailed)
					}
					r.emit(stepCtx, EventStepFailed, map[string]any{"id": id, "step_id": s.ID, "error": res.Err.Error()})
					stepFinished <- fmt.Errorf("workflow %s failed at step %s: %w", id, s.ID, res.Err)
					return
				}

				if r.store != nil {
					if resBytes, err := json.Marshal(res.Output); err == nil {
						_ = r.store.SaveStepOutput(stepCtx, id, s.ID, resBytes)
					}
					_ = r.store.SaveState(stepCtx, id, s.ID, StateCompleted)
				}

				r.emit(stepCtx, EventStepCompleted, map[string]any{"id": id, "step_id": s.ID})

				stateMu.Lock()
				for k, v := range res.Output {
					results[k] = v
				}
				results[s.ID] = res.Output
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
				if r.store != nil {
					_ = r.store.SaveState(context.Background(), id, "", StateFailed)
				}
				return results, err
			}
		case <-ctx.Done():
			for activeCount > 0 {
				<-stepFinished
				activeCount--
			}
			if r.store != nil {
				_ = r.store.SaveState(context.Background(), id, "", StateFailed)
			}
			return results, ctx.Err()
		}
	}

	logger.Info("workflow completed", "id", id)
	if r.store != nil {
		_ = r.store.SaveState(ctx, id, "", StateCompleted)
	}
	r.emit(ctx, EventWorkflowCompleted, map[string]any{"id": id, "name": wf.Name})

	return results, nil
}

func (r *Runner) compensate(ctx context.Context, id string, history []mdk.Step, results map[string]any) {
	if len(history) == 0 {
		return
	}
	logger.Info("workflow compensating", "id", id, "steps_count", len(history))
	r.emit(ctx, EventWorkflowCompensating, map[string]any{"id": id, "steps_count": len(history)})
	for i := len(history) - 1; i >= 0; i-- {
		step := history[i]
		var compensate mdk.StepHandler
		if step.Saga != nil && step.Saga.Uses != "" {
			r.mu.RLock()
			h, ok := r.handlers[step.Saga.Uses]
			r.mu.RUnlock()
			if ok {
				compensate = h
			}
		}

		if compensate == nil {
			continue
		}

		logger.Info("running compensation task", "id", id, "step", step.ID)
		sCtx := mdk.StepContext{
			Ctx:        ctx,
			Runtime:    r.runtime,
			WorkflowID: "",
			RunID:      id,
			StepID:     step.ID,
			Input:      results,
		}
		res := compensate(sCtx)
		if res.Err != nil {
			if step.Saga != nil && step.Saga.IsCritical {
				logger.Error("CRITICAL COMPENSATION FAILED", "id", id, "step", step.ID, "error", res.Err)
			} else {
				logger.Error("compensation task failed", "id", id, "step", step.ID, "error", res.Err)
			}
		}
	}
}

func (r *Runner) emit(ctx context.Context, eventType string, payload any) {
	if r.bus == nil {
		return
	}
	var payloadMap map[string]any
	if bytes, err := json.Marshal(payload); err == nil {
		_ = json.Unmarshal(bytes, &payloadMap)
	}
	parts := strings.SplitN(eventType, ".", 2)
	var ns, et string
	if len(parts) == 2 {
		ns, et = parts[0], parts[1]
	} else {
		ns, et = "workflow", eventType
	}
	event := mdk.Event{
		ID:         "evt_" + uuid.New().String(),
		Namespace:  ns,
		Type:       et,
		Payload:    payloadMap,
		OccurredAt: time.Now(),
	}
	if err := r.bus.Publish(ctx, event); err != nil {
		logger.Error("failed to publish workflow event", "type", eventType, "error", err)
	}
}
