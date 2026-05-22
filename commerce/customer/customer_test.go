package customer

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

func TestCustomerWorkflow(t *testing.T) {
	dbFile := "customer_test.db"
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

	t.Run("Segmentation Workflow", func(t *testing.T) {
		// Create a customer first
		c := &Customer{ID: "c1", Name: "John Doe", Email: "john@example.com"}
		mod.Repo().Save(context.Background(), c)

		wf := &workflow.Workflow{
			Name: "customer.segmentation",
			Steps: []workflow.Step{
				{ID: "customer.calculate_persona", Uses: "customer.calculate_persona"},
				{ID: "customer.update_persona", Uses: "customer.update_persona", DependsOn: []string{"customer.calculate_persona"}},
			},
		}

		input := map[string]any{
			"customer_id": "c1",
			"order_total": 1500.0,
		}

		_, err := runner.Execute(context.Background(), "seg_1", wf, input)
		if err != nil {
			t.Fatalf("workflow failed: %v", err)
		}

		// Verify Persona update
		updated, _ := mod.Repo().GetByID(context.Background(), "c1")
		if updated.Persona != "WHALE" {
			t.Errorf("expected WHALE persona, got %s", updated.Persona)
		}
	})

	t.Run("Get and List", func(t *testing.T) {
		c, err := mod.Repo().GetByID(context.Background(), "c1")
		if err != nil || c.Name != "John Doe" {
			t.Error("GetByID failed")
		}

		c2, err := mod.Repo().GetByUserID(context.Background(), "u123")
		if err == nil {
			t.Error("expected error for non-existent user_id")
		}
		if c2 != nil {
			t.Error("expected nil customer for non-existent user_id")
		}
	})

	t.Run("Handler Error Cases", func(t *testing.T) {
		dbFile := "cust_err_test.db"
		defer os.Remove(dbFile)
		cfg := &config.Config{DBDriver: "sqlite", DBDSN: dbFile}
		database, _ := db.Connect(cfg)
		bus := eventbus.NewInMemBus()
		runner := workflow.NewRunner(bus)
		mod := NewModule()
		mod.Init(context.Background(), &registry.Dependencies{DB: database, EventBus: bus, Runner: runner})
		db.Register(mod.Models()...)
		database.AutoMigrateAll()

		// 1. CalculatePersona - Invalid Input
		_, err := mod.CalculatePersona(context.Background(), "string")
		if err == nil { t.Error("expected error for invalid input type") }
		_, err = mod.CalculatePersona(context.Background(), map[string]any{"wrong": 1})
		if err == nil { t.Error("expected error for missing workflow input") }

		// 2. UpdatePersona - Invalid Input
		_, err = mod.UpdatePersona(context.Background(), "string")
		if err == nil { t.Error("expected error for invalid input type") }
		_, err = mod.UpdatePersona(context.Background(), map[string]any{"wrong": 1})
		if err == nil { t.Error("expected error for missing persona data") }

		// 3. UpdateCustomerDetails - Invalid Input
		_, err = mod.UpdateCustomerDetails(context.Background(), "string")
		if err == nil { t.Error("expected error for invalid input type") }
		_, err = mod.UpdateCustomerDetails(context.Background(), map[string]any{"wrong": 1})
		if err == nil { t.Error("expected error for missing workflow input") }

		// 4. UpdateCustomerDetails - Customer Not Found
		_, err = mod.UpdateCustomerDetails(context.Background(), map[string]any{"input": map[string]any{"id": "ghost"}})
		if err == nil { t.Error("expected error for non-existent customer") }
	})
}
