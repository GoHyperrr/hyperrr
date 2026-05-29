package support

import (
	"context"

	ctxEngine "github.com/GoHyperrr/hyperrr/internal/context"
	"github.com/GoHyperrr/hyperrr/internal/workflow"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
)

// Module implements the registry.Module interface for Support.
type Module struct {
	repo      *Repository
	projector *ctxEngine.Projector
}

func NewModule() *Module {
	return &Module{}
}

func (m *Module) ID() string {
	return "commerce.support"
}

func (m *Module) Init(ctx context.Context, deps *registry.Dependencies) error {
	m.repo = NewRepository(deps.DB)

	// Register Workflows
	deps.Registry.Register(&workflow.Workflow{
		Name: "support.create",
		Steps: []workflow.Step{
			{ID: "ticket", Uses: "support.create_ticket"},
			{ID: "ai_reply", Uses: "support.dispatch_ai_response", DependsOn: []string{"ticket"}},
		},
	})

	return nil
}

func (m *Module) Shutdown(ctx context.Context) error {
	return nil
}

func (m *Module) Models() []any {
	return []any{&Ticket{}, &Message{}}
}

func (m *Module) Handlers() map[string]workflow.TaskHandler {
	return map[string]workflow.TaskHandler{
		"support.create_ticket":       m.CreateTicket,
		"support.dispatch_ai_response": m.DispatchAIResponse,
	}
}

func (m *Module) Repo() *Repository {
	return m.repo
}

func (m *Module) SetProjector(p *ctxEngine.Projector) {
	m.projector = p
}
