package auth

import (
	"context"

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
	m.store = NewAuthStore(deps.DB)
	SetStore(m.store)
	return nil
}

func (m *Module) Models() []any {
	return []any{&RefreshToken{}, &Blacklist{}}
}

func (m *Module) Handlers() map[string]workflow.TaskHandler {
	return nil
}
