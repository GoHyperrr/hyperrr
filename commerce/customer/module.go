package customer

import (
	"context"

	"github.com/GoHyperrr/hyperrr/internal/workflow"
	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
)

// Module implements the registry.Module interface for Customer.
type Module struct {
	repo *Repository
}

func NewModule() *Module {
	return &Module{}
}

func (m *Module) ID() string {
	return "commerce.customer"
}

func (m *Module) Init(ctx context.Context, deps *registry.Dependencies) error {
	m.repo = NewRepository(deps.DB)
	
	// Subscribe to order completions to trigger ML segmentation
	deps.EventBus.Subscribe(ctx, "order.completed", func(ctx context.Context, event eventbus.Event) error {
		payload, ok := event.Payload.(map[string]any)
		if !ok {
			return nil
		}

		// Trigger segmentation workflow asynchronously
		// For MVP, we load the workflow from a file or a predefined string
		// In a real system, the Registry or a WorkflowStore would provide this.
		
		workflowID := "seg_" + payload["customer_id"].(string)
		
		// In a real system, we'd load this from a store.
		wf := &workflow.Workflow{
			Name: "customer.segmentation",
			Steps: []workflow.Step{
				{ID: "customer.calculate_persona", Uses: "customer.calculate_persona"},
				{ID: "customer.update_persona", Uses: "customer.update_persona", DependsOn: []string{"customer.calculate_persona"}},
			},
		}

		go deps.Runner.Execute(ctx, workflowID, wf, payload)
		return nil 
	})

	return nil
}

func (m *Module) Models() []any {
	return []any{&Customer{}, &Address{}}
}

func (m *Module) Handlers() map[string]workflow.TaskHandler {
	return map[string]workflow.TaskHandler{
		"customer.calculate_persona": m.CalculatePersona,
		"customer.update_persona":    m.UpdatePersona,
	}
}

func (m *Module) Repo() *Repository {
	return m.repo
}
