package analytics

import (
	"context"

	ctxEngine "github.com/GoHyperrr/hyperrr/internal/context"
	"github.com/GoHyperrr/hyperrr/internal/workflow"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
)

// Module implements the registry.Module interface for Analytics.
type Module struct {
	projector *ctxEngine.Projector
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

func (m *Module) Projector() *ctxEngine.Projector {
	return m.projector
}

func (m *Module) SetProjector(p *ctxEngine.Projector) {
	m.projector = p
}
