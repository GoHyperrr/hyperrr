package storage

import (
	"context"
	"fmt"

	"github.com/GoHyperrr/hyperrr/pkg/workflow"
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
	bucketURL := deps.Config.StorageBucketURL
	if bucketURL == "" {
		bucketURL = "mem://"
	}

	p, err := NewCloudProvider(ctx, bucketURL)
	if err != nil {
		return fmt.Errorf("failed to initialize cloud storage: %w", err)
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

// Shutdown releases storage resources.
func (m *Module) Shutdown(ctx context.Context) error {
	if m.provider != nil {
		if cp, ok := m.provider.(*CloudProvider); ok {
			return cp.Close()
		}
	}
	return nil
}

// Provider returns the underlying storage provider.
func (m *Module) Provider() ObjectStorage {
	return m.provider
}
