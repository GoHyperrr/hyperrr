package workflow

import "time"

// Workflow represents a declarative commerce workflow.
type Workflow struct {
	Name        string         `yaml:"name" json:"name"`
	Version     string         `yaml:"version" json:"version"`
	Description string         `yaml:"description,omitempty" json:"description,omitempty"`
	Steps       []Step         `yaml:"steps" json:"steps"`
	ExposeToAI  bool           `yaml:"expose_to_ai,omitempty" json:"expose_to_ai,omitempty"`
	InputSchema map[string]any `yaml:"input_schema,omitempty" json:"input_schema,omitempty"`
}

// Step represents a single unit of execution in a workflow.
type Step struct {
	ID         string      `yaml:"id" json:"id"`
	Uses       string      `yaml:"uses" json:"uses"`
	DependsOn  []string    `yaml:"depends_on,omitempty" json:"depends_on,omitempty"`
	Retry      *Retry      `yaml:"retry,omitempty" json:"retry,omitempty"`
	Fallback   *Fallback   `yaml:"fallback,omitempty" json:"fallback,omitempty"`
	Saga       *Saga       `yaml:"saga,omitempty" json:"saga,omitempty"`
	Escalation *Escalation `yaml:"escalation,omitempty" json:"escalation,omitempty"`
}

// Escalation defines the human intervention policy.
type Escalation struct {
	Strategy string `yaml:"strategy" json:"strategy"` // "wait_human"
}

// Retry defines the retry policy for a step.
type Retry struct {
	MaxAttempts int           `yaml:"max_attempts" json:"max_attempts"`
	Backoff     string        `yaml:"backoff" json:"backoff"` // "constant" or "exponential"
	Interval    time.Duration `yaml:"interval" json:"interval"`
}

// Fallback defines the fallback strategy for a step.
type Fallback struct {
	Step string `yaml:"step" json:"step"`
}

// Saga defines the compensation logic for a step.
type Saga struct {
	Uses       string `yaml:"uses" json:"uses"`
	IsCritical bool   `yaml:"is_critical,omitempty" json:"is_critical,omitempty"`
}

// ExecutionState represents the current state of a workflow execution.
type ExecutionState string
