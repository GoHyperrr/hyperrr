package ctxengine

import (
	"context"
	"github.com/GoHyperrr/mdk"
)

// Module implements the mdk.Module interface.
type Module struct {
	projector *Projector
}

func NewModule() *Module {
	return &Module{}
}

func (m *Module) ID() string {
	return "core.context"
}

func (m *Module) Init(ctx context.Context, rt mdk.Runtime) error {
	m.projector = NewProjector(rt.Bus())
	m.projector.SetDB(rt.DB())
	return m.projector.Start(ctx)
}

func (m *Module) Shutdown(ctx context.Context) error {
	return nil
}

func (m *Module) Models() []any {
	return []any{&LineageModel{}}
}

func (m *Module) Routes() []mdk.Route {
	return nil
}

func (m *Module) Projector() mdk.Projector {
	return m.projector
}





