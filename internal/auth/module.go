package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/GoHyperrr/hyperrr/internal/workflow"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
)

// Module implements the registry.Module interface for Auth.
type Module struct {
	store *AuthStore
}

func NewModule() *Module {
	return &Module{}
}

func (m *Module) ID() string {
	return "internal.auth"
}

func (m *Module) Init(ctx context.Context, deps *registry.Dependencies) error {
	exp, err := time.ParseDuration(deps.Config.JWTExpiration)
	if err != nil {
		return fmt.Errorf("invalid JWT_EXPIRATION format: %w", err)
	}
	m.store = NewAuthStore(deps.DB, deps.Config.JWTSecret, exp)
	return nil
}

func (m *Module) Shutdown(ctx context.Context) error {
	return nil
}

func (m *Module) Store() *AuthStore {
	return m.store
}

func (m *Module) Models() []any {
	return []any{&RefreshToken{}, &Blacklist{}}
}

func (m *Module) Handlers() map[string]workflow.TaskHandler {
	return nil
}
