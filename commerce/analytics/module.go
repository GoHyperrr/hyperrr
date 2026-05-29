package analytics

import (
	"context"

	"github.com/GoHyperrr/hyperrr/internal/workflow"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
)

// Module implements the registry.Module interface for Analytics.
type Module struct {
}

func NewModule() *Module {
	return &Module{}
}

func (m *Module) ID() string {
	return "commerce.analytics"
}

func (m *Module) Init(ctx context.Context, deps *registry.Dependencies) error {
	return nil
}

func (m *Module) Models() []any {
	return nil
}

func (m *Module) Handlers() map[string]workflow.TaskHandler {
	return nil
}

func (m *Module) Shutdown(ctx context.Context) error {
	return nil
}

func (m *Module) Repo() any {
	return nil
}
