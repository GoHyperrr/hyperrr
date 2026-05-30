package order

import (
	"context"

	"github.com/GoHyperrr/hyperrr/internal/workflow"
	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
)

// Module implements the registry.Module interface for Order.
type Module struct {
	repo *Repository
	bus  eventbus.EventBus
}

func NewModule() *Module {
	return &Module{}
}

func (m *Module) ID() string {
	return "commerce.order"
}

func (m *Module) Init(ctx context.Context, deps *registry.Dependencies) error {
	m.repo = NewRepository(deps.DB)
	m.bus = deps.EventBus

	// Register Fulfillment Saga
	deps.Registry.Register(&workflow.Workflow{
		Name:        "fulfillment.v1",
		Description: "Orchestrates the full order lifecycle including inventory reservation, payment processing, and shipment creation.",
		ExposeToAI:  true,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"customer_id": map[string]any{"type": "string"},
				"items": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"product_id": map[string]any{"type": "string"},
							"quantity":   map[string]any{"type": "integer"},
						},
					},
				},
			},
			"required": []string{"customer_id", "items"},
		},
		Steps: []workflow.Step{
			{
				ID:   "fulfillment.reserve_inventory",
				Uses: "fulfillment.reserve_inventory",
				Saga: &workflow.Saga{Uses: "fulfillment.release_inventory"},
			},
			{
				ID:        TaskCreateOrder,
				Uses:      TaskCreateOrder,
				Saga:      &workflow.Saga{Uses: TaskCompensatePayment},
				DependsOn: []string{"fulfillment.reserve_inventory"},
			},
			{
				ID:        "finance.process_payment",
				Uses:      "finance.process_payment",
				DependsOn: []string{TaskCreateOrder},
				Saga:      &workflow.Saga{Uses: "finance.compensate_payment"},
			},
			{
				ID:        "fulfillment.create_shipment",
				Uses:      "fulfillment.create_shipment",
				DependsOn: []string{"finance.process_payment"},
			},
			{
				ID:        TaskFinalizeOrder,
				Uses:      TaskFinalizeOrder,
				DependsOn: []string{"fulfillment.create_shipment"},
			},
			{
				ID:        "marketing.add_loyalty_points",
				Uses:      "marketing.add_loyalty_points",
				DependsOn: []string{TaskFinalizeOrder},
			},
		},
	})

	return nil
}

func (m *Module) Shutdown(ctx context.Context) error {
	return nil
}

func (m *Module) Models() []any {
	return []any{&Order{}, &OrderItem{}}
}

func (m *Module) Handlers() map[string]workflow.TaskHandler {
	return map[string]workflow.TaskHandler{
		TaskCreateOrder:       m.CreateOrder,
		TaskFinalizeOrder:     m.FinalizeOrder,
		TaskCompensatePayment: m.CompensatePayment,
	}
}

func (m *Module) Repo() *Repository {
	return m.repo
}
