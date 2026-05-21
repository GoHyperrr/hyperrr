package identity

import (
	"context"

	"github.com/GoHyperrr/hyperrr/internal/workflow"
	"github.com/GoHyperrr/hyperrr/pkg/db"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
)

// Module implements the registry.Module interface for Identity.
type Module struct {
	database *db.DB
}

// NewModule creates a new Identity module.
func NewModule() *Module {
	return &Module{}
}

// ID returns the unique identifier for the module.
func (m *Module) ID() string {
	return "core.identity"
}

// Init initializes the module with its dependencies.
func (m *Module) Init(ctx context.Context, deps *registry.Dependencies) error {
	m.database = deps.DB
	return nil
}

// Models returns the GORM models owned by this module.
func (m *Module) Models() []any {
	return []any{
		&Actor{},
		&User{},
		&APIKey{},
	}
}

// Handlers returns the workflow task handlers provided by this module.
func (m *Module) Handlers() map[string]workflow.TaskHandler {
	return map[string]workflow.TaskHandler{
		"identity.validate_actor": m.ValidateActor,
	}
}
