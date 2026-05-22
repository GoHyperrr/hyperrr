package search

import (
	"context"

	"github.com/GoHyperrr/hyperrr/commerce/product"
	"github.com/GoHyperrr/hyperrr/internal/workflow"
	"github.com/GoHyperrr/hyperrr/pkg/db"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
)

// Module implements the registry.Module interface for Search.
type Module struct {
	db      *db.DB
	prodMod *product.Module
}

func NewModule() *Module {
	return &Module{}
}

func (m *Module) ID() string {
	return "commerce.search"
}

func (m *Module) Init(ctx context.Context, deps *registry.Dependencies) error {
	m.db = deps.DB
	// Note: prodMod will be set via app registration if needed, 
	// or we find it in registry
	return nil
}

func (m *Module) Models() []any {
	return []any{&SearchHistory{}}
}

func (m *Module) Handlers() map[string]workflow.TaskHandler {
	return map[string]workflow.TaskHandler{
		"search.product_catalog": m.SearchProducts,
	}
}

func (m *Module) SetProductModule(pm *product.Module) {
	m.prodMod = pm
}
