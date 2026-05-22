package fulfillment

import (
	"context"

	"github.com/GoHyperrr/hyperrr/internal/workflow"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
)

// Module implements the registry.Module interface for Fulfillment.
type Module struct {
	repo *Repository
}

func NewModule() *Module {
	return &Module{}
}

func (m *Module) ID() string {
	return "commerce.fulfillment"
}

func (m *Module) Init(ctx context.Context, deps *registry.Dependencies) error {
	m.repo = NewRepository(deps.DB)

	// Register Workflows
	deps.Registry.Register(&workflow.Workflow{
		Name: "fulfillment.ship_order",
		Steps: []workflow.Step{
			{ID: "ship", Uses: "fulfillment.ship_order"},
		},
	})

	return nil
}

func (m *Module) Models() []any {
	return []any{&Inventory{}, &Shipment{}}
}

func (m *Module) Handlers() map[string]workflow.TaskHandler {
	return map[string]workflow.TaskHandler{
		"fulfillment.reserve_inventory": m.ReserveInventory,
		"fulfillment.release_inventory": m.ReleaseInventory,
		"fulfillment.create_shipment":   m.CreateShipment,
		"fulfillment.ship_order":        m.ShipOrder, // To update status later
	}
}

func (m *Module) Repo() *Repository {
	return m.repo
}
