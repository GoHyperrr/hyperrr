package workflow

import (
	"github.com/GoHyperrr/mdk"
)

// Re-export/alias mdk workflow types
type Workflow = mdk.Workflow
type Step = mdk.Step
type StepStatus = mdk.StepStatus
type StepContext = mdk.StepContext
type StepResult = mdk.StepResult
type StepHandler = mdk.StepHandler

const (
	StepPending   = mdk.StepPending
	StepRunning   = mdk.StepRunning
	StepCompleted = mdk.StepCompleted
	StepFailed    = mdk.StepFailed
	StepRetrying  = mdk.StepRetrying
	StepSkipped   = mdk.StepSkipped
)
