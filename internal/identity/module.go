package identity

import (
	"context"
	"fmt"
	"time"

	"github.com/GoHyperrr/hyperrr/internal/workflow"
	"github.com/GoHyperrr/hyperrr/pkg/db"
	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
	"github.com/GoHyperrr/hyperrr/pkg/logger"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
)

// Module implements the registry.Module interface for Identity.
type Module struct {
	database *db.DB
	bus      eventbus.EventBus
}

// NewModule creates a new Identity module.
func NewModule() *Module {
	return &Module{}
}

// ID returns the unique identifier for the module.
func (m *Module) ID() string {
	return "core.identity"
}

// Init initializes the module with its dependencies.
func (m *Module) Init(ctx context.Context, deps *registry.Dependencies) error {
	m.database = deps.DB
	m.bus = deps.EventBus
	return nil
}

// Models returns the GORM models owned by this module.
func (m *Module) Models() []any {
	return []any{
		&Actor{},
		&User{},
		&APIKey{},
	}
}

// Handlers returns the workflow task handlers provided by this module.
func (m *Module) Handlers() map[string]workflow.TaskHandler {
	return map[string]workflow.TaskHandler{
		TaskValidateActor: m.ValidateActor,
	}
}

func (m *Module) emit(ctx context.Context, eventType string, payload any) {
	if m.bus == nil {
		return
	}
	event := eventbus.Event{
		ID:        fmt.Sprintf("evt_%d", time.Now().UnixNano()),
		Type:      eventType,
		Payload:   payload,
		Timestamp: time.Now(),
	}
	if err := m.bus.Publish(ctx, event); err != nil {
		logger.Error("failed to publish identity event", "type", eventType, "error", err)
	}
}
