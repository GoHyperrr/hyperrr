package workflow

import (
	"context"
	"fmt"
	"time"

	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
)

type contextKey string

const (
	runnerKey    contextKey = "workflow_runner"
	workflowIDKey contextKey = "workflow_id"
)

// WithRunner injects the Runner and Workflow ID into the context.
func WithRunner(ctx context.Context, r *Runner, id string) context.Context {
	ctx = context.WithValue(ctx, runnerKey, r)
	ctx = context.WithValue(ctx, workflowIDKey, id)
	return ctx
}

// Emit sends an event to the EventBus attached to the current workflow runner.
// It is idempotent: it tracks emitted events in the StateStore to prevent duplicates during resumption.
func Emit(ctx context.Context, eventType string, payload any) error {
	r, ok := ctx.Value(runnerKey).(*Runner)
	if !ok || r == nil {
		return fmt.Errorf("no workflow runner found in context")
	}

	wfID, _ := ctx.Value(workflowIDKey).(string)

	// Idempotency check: don't double-emit during resumption
	if r.store != nil && wfID != "" {
		emitted, err := r.store.IsEventEmitted(ctx, wfID, eventType)
		if err == nil && emitted {
			return nil // Already sent
		}
	}
	
	// If payload is a map, automatically inject the workflow ID for traceability
	if pMap, ok := payload.(map[string]any); ok {
		if _, exists := pMap["_workflow_id"]; !exists {
			pMap["_workflow_id"] = wfID
		}
	}

	r.emit(ctx, eventType, payload)

	// Persist emission record
	if r.store != nil && wfID != "" {
		_ = r.store.RecordEventEmitted(ctx, wfID, eventType)
	}

	return nil
}

// AcquireLock attempts to acquire a lock for the current workflow.
func AcquireLock(ctx context.Context, key string, ttl time.Duration, timeout time.Duration) (bool, error) {
	r, ok := ctx.Value(runnerKey).(*Runner)
	if !ok || r == nil {
		return false, fmt.Errorf("no workflow runner found in context")
	}
	if r.locker == nil {
		return false, fmt.Errorf("no locker configured in workflow runner")
	}
	return r.locker.Acquire(ctx, key, ttl, timeout)
}

// ReleaseLock releases a lock for the current workflow.
func ReleaseLock(ctx context.Context, key string) error {
	r, ok := ctx.Value(runnerKey).(*Runner)
	if !ok || r == nil {
		return fmt.Errorf("no workflow runner found in context")
	}
	if r.locker == nil {
		return fmt.Errorf("no locker configured in workflow runner")
	}
	return r.locker.Release(ctx, key)
}

// GetWorkflowID retrieves the current executing Workflow ID from the context.
func GetWorkflowID(ctx context.Context) string {
	id, _ := ctx.Value(workflowIDKey).(string)
	return id
}

// GetStateStore retrieves the StateStore attached to the current workflow runner.
func GetStateStore(ctx context.Context) StateStore {
	r, ok := ctx.Value(runnerKey).(*Runner)
	if !ok || r == nil {
		return nil
	}
	return r.store
}

// GetEventBus retrieves the EventBus attached to the current workflow runner.
func GetEventBus(ctx context.Context) eventbus.EventBus {
	r, ok := ctx.Value(runnerKey).(*Runner)
	if !ok || r == nil {
		return nil
	}
	return r.bus
}
