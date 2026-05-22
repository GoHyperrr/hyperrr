package order

import (
	"context"

	"github.com/GoHyperrr/hyperrr/internal/workflow"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
)

// Module implements the registry.Module interface for Order.
type Module struct {
	repo *Repository
}

func NewModule() *Module {
	return &Module{}
}

func (m *Module) ID() string {
	return "commerce.order"
}

func (m *Module) Init(ctx context.Context, deps *registry.Dependencies) error {
	m.repo = NewRepository(deps.DB)
	return nil
}

func (m *Module) Models() []any {
	return []any{&Order{}, &OrderItem{}}
}

func (m *Module) Handlers() map[string]workflow.TaskHandler {
	return map[string]workflow.TaskHandler{
		"order.create":             m.CreateOrder,
		"order.process_payment":    m.ProcessPayment,
		"order.finalize":           m.FinalizeOrder,
		"order.compensate_payment": m.CompensatePayment,
	}
}

func (m *Module) Repo() *Repository {
	return m.repo
}
