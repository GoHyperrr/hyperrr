package ctxengine

import (
	"context"
	"github.com/GoHyperrr/hyperrr/pkg/workflow"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
)

// Module implements the registry.Module interface.
type Module struct {
	projector *Projector
}

func NewModule() *Module {
	return &Module{}
}

func init() {
	registry.Register(NewModule())
}

func (m *Module) ID() string {
	return "core.context"
}

func (m *Module) Init(ctx context.Context, deps *registry.Dependencies) error {
	m.projector = NewProjector(deps.EventBus)
	m.projector.SetDB(deps.DB)
	return m.projector.Start(ctx)
}

func (m *Module) Shutdown(ctx context.Context) error {
	return nil
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




