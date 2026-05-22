package product

import (
	"context"
	"os"
	"testing"

	"github.com/GoHyperrr/hyperrr/internal/workflow"
	"github.com/GoHyperrr/hyperrr/pkg/config"
	"github.com/GoHyperrr/hyperrr/pkg/db"
	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
)

func TestProductWorkflow(t *testing.T) {
	dbFile := "product_test.db"
	defer os.Remove(dbFile)

	cfg := &config.Config{
		DBDriver: "sqlite",
		DBDSN:    dbFile,
	}

	database, _ := db.Connect(cfg)
	bus := eventbus.NewInMemBus()
	runner := workflow.NewRunner(bus)

	mod := NewModule()
	deps := &registry.Dependencies{
		DB:       database,
		EventBus: bus,
		Runner:   runner,
	}

	if err := mod.Init(context.Background(), deps); err != nil {
		t.Fatalf("failed to init module: %v", err)
	}

	db.Register(mod.Models()...)
	for name, handler := range mod.Handlers() {
		runner.RegisterTask(name, handler)
	}
	database.AutoMigrateAll()

	t.Run("Create Product Workflow", func(t *testing.T) {
		wf := &workflow.Workflow{
			Name: "product.create",
			Steps: []workflow.Step{
				{ID: "product.validate_product", Uses: "product.validate_product"},
				{ID: "product.persist_product", Uses: "product.persist_product", DependsOn: []string{"product.validate_product"}},
			},
		}

		input := map[string]any{
			"id":          "p1",
			"name":        "Test Product",
			"description": "Desc",
			"price":       100.0,
		}

		res, err := runner.Execute(context.Background(), "create_1", wf, input)
		if err != nil {
			t.Fatalf("workflow failed: %v", err)
		}

		p, ok := res["product.persist_product"].(*Product)
		if !ok || p.Name != "Test Product" {
			t.Errorf("expected Test Product, got %v", res["product.persist_product"])
		}
	})

	t.Run("Invalid Product", func(t *testing.T) {
		wf := &workflow.Workflow{
			Steps: []workflow.Step{
				{ID: "v1", Uses: "product.validate_product"},
			},
		}

		input := map[string]any{
			"name":  "",
			"price": -10.0,
		}

		_, err := runner.Execute(context.Background(), "create_invalid", wf, input)
		if err == nil {
			t.Error("expected validation error")
		}
	})

	t.Run("Handler Error Cases", func(t *testing.T) {
		dbFile := "prod_err_test.db"
		defer os.Remove(dbFile)
		cfg := &config.Config{DBDriver: "sqlite", DBDSN: dbFile}
		database, _ := db.Connect(cfg)
		bus := eventbus.NewInMemBus()
		runner := workflow.NewRunner(bus)
		mod := NewModule()
		mod.Init(context.Background(), &registry.Dependencies{DB: database, EventBus: bus, Runner: runner})
		db.Register(mod.Models()...)
		database.AutoMigrateAll()

		// 1. ValidateProduct - Invalid Input
		_, err := mod.ValidateProduct(context.Background(), "string")
		if err == nil { t.Error("expected error for invalid input type") }
		_, err = mod.ValidateProduct(context.Background(), map[string]any{"wrong": 1})
		if err == nil { t.Error("expected error for missing workflow input") }

		// 2. PersistProduct - Invalid Input
		_, err = mod.PersistProduct(context.Background(), "string")
		if err == nil { t.Error("expected error for invalid input type") }
		_, err = mod.PersistProduct(context.Background(), map[string]any{"wrong": 1})
		if err == nil { t.Error("expected error for missing validation results") }

		// 3. UpdateProductDetails - Invalid Input
		_, err = mod.UpdateProductDetails(context.Background(), "string")
		if err == nil { t.Error("expected error for invalid input type") }
		_, err = mod.UpdateProductDetails(context.Background(), map[string]any{"wrong": 1})
		if err == nil { t.Error("expected error for missing workflow input") }
	})
}
