package workflow

// Workflow and Step statuses.
const (
	StatePending      = "PENDING"
	StateRunning      = "RUNNING"
	StateCompleted    = "COMPLETED"
	StateFailed       = "FAILED"
	StateWaitingHuman = "WAITING_HUMAN"
	StateRetrying     = "RETRYING"
	StateFallback     = "FALLBACK"
	StateCompensating = "COMPENSATING"
)

// Escalation strategies.
const (
	EscalationWaitHuman = "wait_human"
)

// Action types for human intervention.
const (
	ActionRetry  = "retry"
	ActionCancel = "cancel"
)

// Backoff strategies.
const (
	BackoffExponential = "exponential"
	BackoffConstant    = "constant"
)

// Workflow Event types.
const (
	EventWorkflowStarted       = "workflow.started"
	EventWorkflowCompleted     = "workflow.completed"
	EventWorkflowFailed        = "workflow.failed"
	EventWorkflowCompensating  = "workflow.compensating"
	EventStepStarted           = "workflow.step.started"
	EventStepCompleted         = "workflow.step.completed"
	EventStepFailed            = "workflow.step.failed"
	EventStepRetrying          = "workflow.step.retrying"
	EventStepFallback          = "workflow.step.fallback"
	EventWaitingHuman          = "workflow.waiting_human"
)
