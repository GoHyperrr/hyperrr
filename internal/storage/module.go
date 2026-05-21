package storage

import (
	"context"

	"github.com/GoHyperrr/hyperrr/internal/workflow"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
)

// Module implements the registry.Module interface for Storage.
type Module struct {
	provider ObjectStorage
}

// NewModule creates a new Storage module.
func NewModule() *Module {
	return &Module{}
}

// ID returns the unique identifier for the module.
func (m *Module) ID() string {
	return "core.storage"
}

// Init initializes the module.
func (m *Module) Init(ctx context.Context, deps *registry.Dependencies) error {
	// For now, hardcode local provider. In Task 4 we can make it swappable.
	p, err := NewLocalProvider("storage")
	if err != nil {
		return err
	}
	m.provider = p
	return nil
}

// Models returns the GORM models owned by this module.
func (m *Module) Models() []any {
	return nil
}

// Handlers returns the workflow task handlers provided by this module.
func (m *Module) Handlers() map[string]workflow.TaskHandler {
	return nil
}

// Provider returns the underlying storage provider.
func (m *Module) Provider() ObjectStorage {
	return m.provider
}
