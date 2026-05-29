package registry

import (
	"context"
	"time"

	"github.com/GoHyperrr/hyperrr/internal/workflow"
	"github.com/GoHyperrr/hyperrr/pkg/config"
	"github.com/GoHyperrr/hyperrr/pkg/db"
	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
)

// WorkflowRunner defines the interface for executing workflows.
type WorkflowRunner interface {
	Execute(ctx context.Context, id string, wf *workflow.Workflow, input any) (map[string]any, error)
}

// WorkflowRegistry defines the interface for managing workflow definitions.
type WorkflowRegistry interface {
	Register(wf *workflow.Workflow) error
	Get(name string) (*workflow.Workflow, error)
}

// Dependencies provides common utilities to modules.
type Dependencies struct {
	Config    *config.Config
	DB        *db.DB
	EventBus  eventbus.EventBus
	Runner    WorkflowRunner
	Registry  WorkflowRegistry
}


// LineageData defines the minimal interface for accessing workflow execution data.
type LineageData interface {
	GetID() string
	GetName() string
	GetState() string
	GetError() string
	GetStartedAt() time.Time
	GetEndedAt() *time.Time
}

// Projector defines the interface for querying execution lineages.
type Projector interface {
	ListLineages() []LineageData
	QueryLineages(filter func(LineageData) bool) []LineageData
}

// OrderResult defines the minimal interface for accessing order data across modules.
type OrderResult interface {
	GetOrderID() string
	GetTotal() float64
	GetCustomerID() string
}


// Module is the standard interface for all hyperrr plugins.
type Module interface {
	// ID returns the unique identifier for the module.
	ID() string
	
	// Init initializes the module with its dependencies.
	Init(ctx context.Context, deps *Dependencies) error
	
	// Models returns the GORM models owned by this module.
	Models() []any
	
	// Handlers returns the workflow task handlers provided by this module.
	Handlers() map[string]workflow.TaskHandler

	// Shutdown allows the module to release resources cleanly.
	Shutdown(ctx context.Context) error
}
