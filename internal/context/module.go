package context

import (
	"context"
	"github.com/GoHyperrr/hyperrr/internal/workflow"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
)

// Module implements the registry.Module interface.
type Module struct {
	projector *Projector
}

func NewModule() *Module {
	return &Module{}
}

func (m *Module) ID() string {
	return "core.context"
}

func (m *Module) Init(ctx context.Context, deps *registry.Dependencies) error {
	m.projector = NewProjector(deps.EventBus)
	m.projector.store = NewLineageStore(deps.DB)
	return m.projector.Start(ctx)
}

func (m *Module) Models() []any {
	return []any{&LineageModel{}}
}

func (m *Module) Handlers() map[string]workflow.TaskHandler {
	return nil
}

func (m *Module) Projector() *Projector {
	return m.projector
}
