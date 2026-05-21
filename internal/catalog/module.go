package catalog

import (
	"context"

	"github.com/GoHyperrr/hyperrr/internal/workflow"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
)

// Module implements the registry.Module interface for the Catalog.
type Module struct {
	repo *Repository
}

func NewModule() *Module {
	return &Module{}
}

func (m *Module) ID() string {
	return "commerce.catalog"
}

func (m *Module) Init(ctx context.Context, deps *registry.Dependencies) error {
	m.repo = NewRepository(deps.DB)
	return nil
}

func (m *Module) Models() []any {
	return []any{&Product{}}
}

func (m *Module) Handlers() map[string]workflow.TaskHandler {
	return map[string]workflow.TaskHandler{
		"catalog.validate_product": m.ValidateProduct,
		"catalog.persist_product":  m.PersistProduct,
	}
}
