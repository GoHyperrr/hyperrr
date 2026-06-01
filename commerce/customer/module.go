package customer
import (
	"context"

	ctxEngine "github.com/GoHyperrr/hyperrr/pkg/ctxengine"
	"github.com/GoHyperrr/hyperrr/pkg/workflow"
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

func init() {
	registry.Register(NewModule())
}

func (m *Module) ID() string {
	return "commerce.customer"
}

func (m *Module) Init(ctx context.Context, deps *registry.Dependencies) error {
	m.repo = NewRepository(deps.DB)

	// Try to resolve Projector from registry if not explicitly set
	if m.projector == nil {
		if ctxModVal, ok := registry.Get("core.context"); ok {
			if ctxMod, ok := ctxModVal.(*ctxEngine.Module); ok {
				m.projector = ctxMod.Projector()
				m.brain = NewMLBrainV2(m.projector)
			}
		}
	}

	// Register Workflows
	deps.Registry.Register(&workflow.Workflow{
		Name: "customer.segmentation",
		Steps: []workflow.Step{
			{ID: "calculate", Uses: "customer.calculate_persona"},
			{ID: "update", Uses: "customer.update_persona", DependsOn: []string{"calculate"}},
		},
	})

	// Subscribe to user creation to create a business profile
	_, _ = deps.EventBus.Subscribe(ctx, "identity.user_created", func(ctx context.Context, event eventbus.Event) error {
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
	_, _ = deps.EventBus.Subscribe(ctx, "order.completed", func(ctx context.Context, event eventbus.Event) error {
		payload, ok := event.Payload.(map[string]any)
		if !ok {
			return nil
		}

		customerID := utils.GetString(payload, "customer_id")
		if customerID == "" {
			return nil
		}

		workflowID := "seg_" + customerID
		wf, err := deps.Registry.Get("customer.segmentation")
		if err != nil {
			return err
		}

		go func() {
			if _, err := deps.Runner.Execute(ctx, workflowID, wf, payload); err != nil {
				logger.Error("background segmentation failed", "customer_id", customerID, "error", err)
			}
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

func (m *Module) Shutdown(ctx context.Context) error {
	return nil
}

func (m *Module) Repo() *Repository {
	return m.repo
}

func (m *Module) SetProjector(p *ctxEngine.Projector) {
	m.projector = p
	m.brain = NewMLBrainV2(p)
}

// Ensure Module implements registry.TUIProvider at compile-time.
var _ registry.TUIProvider = (*Module)(nil)

// TUIPages registers the customer administration dashboard page.
func (m *Module) TUIPages() []registry.TUIPage {
	return []registry.TUIPage{
		&customerPage{},
	}
}

