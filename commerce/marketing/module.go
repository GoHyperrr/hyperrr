package marketing

import (
	"context"

	"github.com/GoHyperrr/hyperrr/pkg/workflow"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
)

// Module implements the registry.Module interface for Marketing.
type Module struct {
	repo *Repository
}

func NewModule() *Module {
	return &Module{}
}

func (m *Module) ID() string {
	return "commerce.marketing"
}

func (m *Module) Init(ctx context.Context, deps *registry.Dependencies) error {
	m.repo = NewRepository(deps.DB)

	// Register Workflows
	deps.Registry.Register(&workflow.Workflow{
		Name: "marketing.apply_coupon",
		Steps: []workflow.Step{
			{ID: "validate", Uses: "marketing.validate_coupon"},
			{ID: "apply", Uses: "cart.add_item", DependsOn: []string{"validate"}},
		},
	})

	return nil
}

func (m *Module) Models() []any {
	return []any{&Coupon{}, &LoyaltyPoints{}}
}

func (m *Module) Handlers() map[string]workflow.TaskHandler {
	return map[string]workflow.TaskHandler{
		"marketing.validate_coupon":    m.ValidateCoupon,
		"marketing.add_loyalty_points": m.AddLoyaltyPoints,
	}
}

func (m *Module) Shutdown(ctx context.Context) error {
	return nil
}

func (m *Module) Repo() *Repository {
	return m.repo
}
