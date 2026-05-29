package notification

import (
	"context"
	"fmt"

	"github.com/GoHyperrr/hyperrr/internal/workflow"
	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
	"github.com/GoHyperrr/hyperrr/pkg/logger"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
	"github.com/GoHyperrr/hyperrr/pkg/utils"
	"github.com/google/uuid"
)

// Module implements the registry.Module interface for Notification.
type Module struct {
	repo     *Repository
	provider Provider
}

func NewModule(provider Provider) *Module {
	if provider == nil {
		provider = &MockProvider{} // Default to mock
	}
	return &Module{provider: provider}
}

func (m *Module) ID() string {
	return "commerce.notification"
}

func (m *Module) Init(ctx context.Context, deps *registry.Dependencies) error {
	m.repo = NewRepository(deps.DB)

	// Subscribe to Identity User Created
	_, _ = deps.EventBus.Subscribe(ctx, "identity.user_created", func(ctx context.Context, event eventbus.Event) error {
		payload, ok := event.Payload.(map[string]any)
		if !ok {
			return nil
		}

		email := utils.GetString(payload, "email")
		name := utils.GetString(payload, "name")

		wf := &workflow.Workflow{
			Steps: []workflow.Step{{ID: "send", Uses: "notification.send"}},
		}

		input := map[string]any{
			"recipient": email,
			"channel":   string(ChannelEmail),
			"subject":   "Welcome to hyperrr!",
			"body":      fmt.Sprintf("Hi %s, thanks for joining.", name),
		}

		go deps.Runner.Execute(ctx, "notify_"+uuid.New().String(), wf, input)
		return nil
	})

	// Subscribe to Order Completed (Workflow Completed)
	_, _ = deps.EventBus.Subscribe(ctx, "workflow.completed", func(ctx context.Context, event eventbus.Event) error {
		payload, ok := event.Payload.(map[string]any)
		if !ok {
			return nil
		}

		wfName := utils.GetString(payload, "name")
		if wfName != "fulfillment.v1" {
			return nil
		}


		// In a real system, we'd fetch the order details here to get the email.
		// For this MVP, we'll just log that we would send it if we had the context easily available.
		logger.Info("Fulfillment completed, would send order confirmation email")

		return nil
	})

	return nil
}

func (m *Module) Shutdown(ctx context.Context) error {
	return nil
}

func (m *Module) Models() []any {
	return []any{&Notification{}}
}

func (m *Module) Handlers() map[string]workflow.TaskHandler {
	return map[string]workflow.TaskHandler{
		"notification.send": m.SendNotification,
	}
}

func (m *Module) Repo() *Repository {
	return m.repo
}
