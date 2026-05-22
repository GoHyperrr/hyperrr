package search

import (
	"context"
	"os"
	"testing"

	"github.com/GoHyperrr/hyperrr/commerce/product"
	"github.com/GoHyperrr/hyperrr/internal/workflow"
	"github.com/GoHyperrr/hyperrr/pkg/config"
	"github.com/GoHyperrr/hyperrr/pkg/db"
	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
)

func TestSearchModule(t *testing.T) {
	dbFile := "search_test.db"
	defer os.Remove(dbFile)

	cfg := &config.Config{DBDriver: "sqlite", DBDSN: dbFile}
	database, _ := db.Connect(cfg)
	bus := eventbus.NewInMemBus()
	runner := workflow.NewRunner(bus)
	registryStore := workflow.NewRegistry()

	// Mock Product module
	prodMod := product.NewModule()
	prodMod.Init(context.Background(), &registry.Dependencies{DB: database, EventBus: bus, Runner: runner, Registry: registryStore})
	db.Register(prodMod.Models()...)
	
	mod := NewModule()
	mod.Init(context.Background(), &registry.Dependencies{DB: database, EventBus: bus, Runner: runner, Registry: registryStore})
	mod.SetProductModule(prodMod)
	db.Register(mod.Models()...)
	for name, h := range mod.Handlers() { runner.RegisterTask(name, h) }
	database.AutoMigrateAll()

	// Seed products
	prodMod.Repo().Save(context.Background(), &product.Product{ID: "p1", Name: "Go Gopher", Price: 10.0})
	prodMod.Repo().Save(context.Background(), &product.Product{ID: "p2", Name: "Rust Crab", Price: 15.0})

	t.Run("Search Success", func(t *testing.T) {
		wf := &workflow.Workflow{
			Steps: []workflow.Step{{ID: "search", Uses: "search.product_catalog"}},
		}

		input := map[string]any{"query": "Go", "limit": 1.0}
		res, err := runner.Execute(context.Background(), "s1", wf, input)
		if err != nil {
			t.Fatalf("workflow failed: %v", err)
		}

		results := res["search"].([]*product.Product)
		if len(results) != 1 || results[0].Name != "Go Gopher" {
			t.Errorf("unexpected results: %v", results)
		}
	})

	t.Run("Handler Error Cases", func(t *testing.T) {
		_, err := mod.SearchProducts(context.Background(), "string")
		if err == nil { t.Error("expected error for invalid input type") }
		
		_, err = mod.SearchProducts(context.Background(), map[string]any{"wrong": 1})
		if err == nil { t.Error("expected error for missing workflow input") }

		mNoProd := NewModule()
		mNoProd.Init(context.Background(), &registry.Dependencies{DB: database, Registry: workflow.NewRegistry()})
		_, err = mNoProd.SearchProducts(context.Background(), map[string]any{"input": map[string]any{"query": "x"}})
		if err == nil { t.Error("expected error for missing product module") }
	})
}
