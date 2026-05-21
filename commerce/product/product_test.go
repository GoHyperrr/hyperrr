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

	database, err := db.Connect(cfg)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}

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

	// Register models and handlers
	db.Register(mod.Models()...)
	for name, handler := range mod.Handlers() {
		runner.RegisterTask(name, handler)
	}

	if err := database.AutoMigrateAll(); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	t.Run("Create Product Workflow", func(t *testing.T) {
		parser := workflow.NewParser()
		yamlData, _ := os.ReadFile("workflows/product_create.yaml")
		wf, err := parser.Parse(yamlData)
		if err != nil {
			t.Fatalf("failed to parse workflow: %v", err)
		}

		input := map[string]any{
			"id":          "prod_1",
			"name":        "Test Product",
			"description": "Best product ever",
			"price":       99.99,
		}

		err = runner.Execute(context.Background(), "exec_1", wf, input)
		if err != nil {
			t.Fatalf("workflow execution failed: %v", err)
		}

		// Verify in DB
		p, err := mod.repo.GetByID(context.Background(), "prod_1")
		if err != nil {
			t.Fatalf("failed to get product: %v", err)
		}
		if p.Name != "Test Product" {
			t.Errorf("expected Test Product, got %s", p.Name)
		}
	})

	t.Run("Validation Failure", func(t *testing.T) {
		parser := workflow.NewParser()
		yamlData, _ := os.ReadFile("workflows/product_create.yaml")
		wf, _ := parser.Parse(yamlData)

		input := map[string]any{
			"id":    "prod_fail",
			"name":  "", // Should fail
			"price": 10.0,
		}

		err = runner.Execute(context.Background(), "exec_fail", wf, input)
		if err == nil {
			t.Error("expected validation error, got nil")
		}
	})
	
	t.Run("List and Repository", func(t *testing.T) {
		products, err := mod.repo.List(context.Background())
		if err != nil {
			t.Errorf("failed to list: %v", err)
		}
		if len(products) < 1 {
			t.Error("expected at least 1 product")
		}
	})
}
