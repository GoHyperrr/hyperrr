package cart

import (
	"context"

	"github.com/GoHyperrr/hyperrr/pkg/workflow"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
)

// Module implements the registry.Module interface for Cart.
type Module struct {
	repo *Repository
}

func NewModule() *Module {
	return &Module{}
}

func (m *Module) ID() string {
	return "commerce.cart"
}

func (m *Module) Init(ctx context.Context, deps *registry.Dependencies) error {
	m.repo = NewRepository(deps.DB)

	// Register Workflows
	deps.Registry.Register(&workflow.Workflow{
		Name: "cart.add",
		Steps: []workflow.Step{
			{ID: "add", Uses: "cart.add_item"},
		},
	})

	deps.Registry.Register(&workflow.Workflow{
		Name: "cart.remove",
		Steps: []workflow.Step{
			{ID: "remove", Uses: "cart.remove_item"},
		},
	})

	deps.Registry.Register(&workflow.Workflow{
		Name: "cart.checkout",
		Steps: []workflow.Step{
			{ID: "checkout", Uses: "cart.checkout"},
		},
	})

	return nil
}

func (m *Module) Models() []any {
	return []any{&Cart{}, &CartItem{}}
}

func (m *Module) Handlers() map[string]workflow.TaskHandler {
	return map[string]workflow.TaskHandler{
		"cart.add_item":    m.AddItem,
		"cart.remove_item": m.RemoveItem,
		"cart.checkout":    m.Checkout,
	}
}

func (m *Module) Shutdown(ctx context.Context) error {
	return nil
}

func (m *Module) Repo() *Repository {
	return m.repo
}
