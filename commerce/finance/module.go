package finance

import (
	"context"

	"github.com/GoHyperrr/hyperrr/internal/workflow"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
)

// Module implements the registry.Module interface for Finance.
type Module struct {
	repo *Repository
}

func NewModule() *Module {
	return &Module{}
}

func (m *Module) ID() string {
	return "commerce.finance"
}

func (m *Module) Init(ctx context.Context, deps *registry.Dependencies) error {
	m.repo = NewRepository(deps.DB)
	return nil
}

func (m *Module) Models() []any {
	return []any{&Payment{}}
}

func (m *Module) Handlers() map[string]workflow.TaskHandler {
	return map[string]workflow.TaskHandler{
		"finance.process_payment":    m.ProcessPayment,
		"finance.compensate_payment": m.CompensatePayment,
	}
}

func (m *Module) Repo() *Repository {
	return m.repo
}
