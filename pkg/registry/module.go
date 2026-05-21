package registry

import (
	"context"

	"github.com/GoHyperrr/hyperrr/internal/workflow"
	"github.com/GoHyperrr/hyperrr/pkg/db"
	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
)

// Dependencies provides access to core OS services for modules.
type Dependencies struct {
	DB       *db.DB
	EventBus eventbus.EventBus
	Runner   *workflow.Runner
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
}
