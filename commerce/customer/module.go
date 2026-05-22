package customer
import (
	"context"

	ctxEngine "github.com/GoHyperrr/hyperrr/internal/context"
	"github.com/GoHyperrr/hyperrr/internal/workflow"
	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
	"github.com/GoHyperrr/hyperrr/pkg/logger"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
	"github.com/GoHyperrr/hyperrr/pkg/utils"
)

// Module implements the registry.Module interface for Customer.
type Module struct {
	repo    *Repository
	brain   *MLBrainV2
	projector *ctxEngine.Projector
}

func NewModule() *Module {
	return &Module{}
}

func (m *Module) ID() string {
	return "commerce.customer"
}

func (m *Module) Init(ctx context.Context, deps *registry.Dependencies) error {
	m.repo = NewRepository(deps.DB)

	// Register Workflows
	deps.Registry.Register(&workflow.Workflow{
		Name: "customer.segmentation",
		Steps: []workflow.Step{
			{ID: "calculate", Uses: "customer.calculate_persona"},
			{ID: "update", Uses: "customer.update_persona", DependsOn: []string{"calculate"}},
		},
	})

	// Subscribe to user creation to create a business profile
	deps.EventBus.Subscribe(ctx, "identity.user_created", func(ctx context.Context, event eventbus.Event) error {
		payload, ok := event.Payload.(map[string]any)
		if !ok {
			return nil
		}

		actorID := utils.GetString(payload, "actor_id")
		userID := utils.GetString(payload, "user_id")
		if userID == "" {
			userID = actorID
		}
		name := utils.GetString(payload, "name")
		email := utils.GetString(payload, "email")

		if actorID == "" {
			return nil
		}

		c := &Customer{
			ID:     "cust_" + userID,
			UserID: actorID,
			Name:   name,
			Email:  email,
		}

		if err := m.repo.Save(ctx, c); err != nil {
			logger.Error("failed to create customer from event", "error", err)
			return err
		}

		logger.Info("Customer profile created for user", "id", c.ID)
		return nil
	})

	// Subscribe to order completions to trigger ML segmentation
	deps.EventBus.Subscribe(ctx, "order.completed", func(ctx context.Context, event eventbus.Event) error {
		payload, ok := event.Payload.(map[string]any)
		if !ok {
			return nil
		}

		workflowID := "seg_" + payload["customer_id"].(string)
		wf, err := deps.Registry.Get("customer.segmentation")
		if err != nil {
			return err
		}

		go func() {
			_, _ = deps.Runner.Execute(ctx, workflowID, wf, payload)
		}()
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
		"customer.update_details":    m.UpdateCustomerDetails,
	}
}

func (m *Module) Repo() *Repository {
	return m.repo
}

func (m *Module) SetProjector(p *ctxEngine.Projector) {
	m.projector = p
	m.brain = NewMLBrainV2(p)
}
