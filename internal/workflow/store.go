package workflow

import (
	"context"
	"time"
)

// StateStore defines the interface for checkpointing workflow states.
type StateStore interface {
	// SaveState records the state of a specific step in a workflow execution.
	SaveState(ctx context.Context, execID string, stepID string, state string) error
	
	// GetState retrieves the state of all steps for a workflow execution.
	// Returns a map of stepID -> state (e.g. "COMPLETED", "FAILED").
	GetState(ctx context.Context, execID string) (map[string]string, error)
	
	// InitializeExecution sets up the initial state and input for a workflow.
	InitializeExecution(ctx context.Context, execID string, input []byte) error
	
	// SaveInput stores the initial input payload of the workflow for resumption.
	SaveInput(ctx context.Context, execID string, input []byte) error
	
	// GetInput retrieves the initial input payload of the workflow.
	GetInput(ctx context.Context, execID string) ([]byte, error)
	
	// SetTTL sets an expiration on the workflow execution state to prevent storage unbounded growth.
	SetTTL(ctx context.Context, execID string, ttl time.Duration) error
	
	// SaveStepOutput stores the successful result payload of a step for resumption.
	SaveStepOutput(ctx context.Context, execID string, stepID string, output []byte) error
	
	// GetStepOutput retrieves the successful result payload of a step.
	GetStepOutput(ctx context.Context, execID string, stepID string) ([]byte, error)
}
