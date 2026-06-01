package customer

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/GoHyperrr/hyperrr/pkg/workflow"
	"github.com/GoHyperrr/hyperrr/pkg/config"
	"github.com/GoHyperrr/hyperrr/pkg/db"
	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
	ctxEngine "github.com/GoHyperrr/hyperrr/pkg/ctxengine"
)

func TestCustomerWorkflow(t *testing.T) {
	cfg := &config.Config{
		DBDriver: "sqlite",
		DBDSN:    ":memory:",
	}

	database, _ := db.Connect(cfg)
	bus := eventbus.NewInMemBus()
	runner := workflow.NewRunner(bus, nil, nil)
	registryStore := workflow.NewRegistry()
	projector := ctxEngine.NewProjector(bus)
	projector.Start(context.Background())

	mod := NewModule()
	deps := &registry.Dependencies{
		DB:       database,
		EventBus: bus,
		Runner:   runner,
		Registry: registryStore,
	}

	if err := mod.Init(context.Background(), deps); err != nil {
		t.Fatalf("failed to init module: %v", err)
	}
	mod.SetProjector(projector)

	db.Register(mod.Models()...)
	for name, handler := range mod.Handlers() {
		runner.RegisterTask(name, handler)
	}
	database.AutoMigrateAll()

	t.Run("Segmentation Workflow", func(t *testing.T) {
		// Create a customer first
		c := &Customer{ID: "c1", Name: "John Doe", Email: "john@example.com"}
		mod.Repo().Save(context.Background(), c)

		// Seed lineages to get WHALE persona (needs > 5 orders)
		for i := 0; i < 6; i++ {
			wfID := fmt.Sprintf("wf_%d", i)
			bus.Publish(context.Background(), eventbus.Event{
				ID:   wfID + "_start",
				Type: "workflow.started",
				Payload: map[string]any{
					"name":    "fulfillment.v1",
					"id":      wfID,
					"version": "v1",
				},
			})
			bus.Publish(context.Background(), eventbus.Event{
				ID:   wfID + "_end",
				Type: "workflow.completed",
				Payload: map[string]any{
					"name": "fulfillment.v1",
					"id":   wfID,
				},
			})
		}
		// Give projector a moment to process events
		time.Sleep(100 * time.Millisecond)

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
		cfg := &config.Config{DBDriver: "sqlite", DBDSN: ":memory:"}
		database, _ := db.Connect(cfg)
		bus := eventbus.NewInMemBus()
		runner := workflow.NewRunner(bus, nil, nil)
		registryStore := workflow.NewRegistry()

		mod := NewModule()
		mod.Init(context.Background(), &registry.Dependencies{DB: database, EventBus: bus, Runner: runner, Registry: registryStore})
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
		
		// 5. identity.user_created - Missing actor_id (should skip gracefully)
		bus.Publish(context.Background(), eventbus.Event{
			Type: "identity.user_created",
			Payload: map[string]any{"user_id": "u_no_actor", "email": "test@test.com"},
		})
		time.Sleep(50 * time.Millisecond)
		_, err = mod.Repo().GetByUserID(context.Background(), "u_no_actor")
		if err == nil {
			t.Error("expected no customer to be created for missing actor_id")
		}

		// 6. CalculatePersona - Nil brain
		badMod := &Module{repo: mod.repo}
		_, err = badMod.CalculatePersona(context.Background(), map[string]any{"input": map[string]any{"customer_id": "c1"}})
		if err == nil || !strings.Contains(err.Error(), "ML brain not initialized") {
			t.Errorf("expected ML brain not initialized error, got %v", err)
		}

		// 7. order.completed - Missing customer_id
		bus.Publish(context.Background(), eventbus.Event{
			Type: "order.completed",
			Payload: map[string]any{"wrong": "data"},
		})
		// Should just skip gracefully

		// 8. GetByUserID - Success
		c := &Customer{ID: "c_user", UserID: "u_real", Name: "Real User"}
		mod.Repo().Save(context.Background(), c)
		got, err := mod.Repo().GetByUserID(context.Background(), "u_real")
		if err != nil || got.ID != "c_user" {
			t.Errorf("GetByUserID failed: %v", err)
		}

		// 9. UpdateCustomerDetails - Success
		updateInput := map[string]any{
			"input": map[string]any{
				"id":    "c_user",
				"name":  "Updated Name",
				"email": "updated@example.com",
			},
		}
		_, err = mod.UpdateCustomerDetails(context.Background(), updateInput)
		if err != nil {
			t.Errorf("UpdateCustomerDetails failed: %v", err)
		}
		updated, _ := mod.Repo().GetByID(context.Background(), "c_user")
		if updated.Name != "Updated Name" || updated.Email != "updated@example.com" {
			t.Error("UpdateCustomerDetails did not save changes")
		}

		// 10. UpdateCustomerDetails - Save Failure
		badCfg := &config.Config{DBDriver: "sqlite", DBDSN: ":memory:"}
		badDB, _ := db.Connect(badCfg)
		sqlDB, _ := badDB.DB.DB()
		sqlDB.Close()

		originalRepo := mod.repo
		mod.repo = NewRepository(badDB)
		_, err = mod.UpdateCustomerDetails(context.Background(), updateInput)
		if err == nil { t.Error("expected error for failed save in UpdateCustomerDetails") }
		
		// 11. UpdatePersona - Customer Not Found
		mod.repo = originalRepo // Restore to find "c_user" if needed, but here we want non-existent
		personaInputGhost := map[string]any{
			"calculate": map[string]any{
				"customer_id": "ghost_cust",
				"persona":     "WHALE",
			},
		}
		_, err = mod.UpdatePersona(context.Background(), personaInputGhost)
		if err == nil { t.Error("expected error for non-existent customer in UpdatePersona") }

		// 12. UpdatePersona - Save Failure
		mod.repo = NewRepository(badDB)
		personaInputValid := map[string]any{
			"calculate": map[string]any{
				"customer_id": "c_user",
				"persona":     "WHALE",
			},
		}
		_, err = mod.UpdatePersona(context.Background(), personaInputValid)
		if err == nil { t.Error("expected error for failed save in UpdatePersona") }
		
		mod.repo = originalRepo

		// 13. UpdatePersona - Fallback Step Name
		personaInputFallback := map[string]any{
			"customer.calculate_persona": map[string]any{
				"customer_id": "c_user",
				"persona":     "GOLD",
			},
		}
		_, err = mod.UpdatePersona(context.Background(), personaInputFallback)
		if err != nil { t.Errorf("UpdatePersona fallback failed: %v", err) }
		updated, _ = mod.repo.GetByID(context.Background(), "c_user")
		if updated.Persona != "GOLD" { t.Errorf("expected GOLD, got %s", updated.Persona) }

		// 14. UpdateCustomerDetails - Empty fields (no change)
		emptyInput := map[string]any{
			"input": map[string]any{
				"id":    "c_user",
				"name":  "",
				"email": "",
			},
		}
		_, err = mod.UpdateCustomerDetails(context.Background(), emptyInput)
		if err != nil { t.Errorf("UpdateCustomerDetails failed: %v", err) }
		updated, _ = mod.repo.GetByID(context.Background(), "c_user")
		if updated.Name != "Updated Name" || updated.Email != "updated@example.com" {
			t.Error("UpdateCustomerDetails changed fields that were empty in input")
		}

		// 15. identity.user_created - Success
		bus.Publish(context.Background(), eventbus.Event{
			Type: "identity.user_created",
			Payload: map[string]any{
				"actor_id": "u_new",
				"user_id":  "u_new_id",
				"name":     "New User",
				"email":    "new@test.com",
			},
		})
		time.Sleep(50 * time.Millisecond)
		got, err = mod.Repo().GetByUserID(context.Background(), "u_new")
		if err != nil || got.Name != "New User" {
			t.Errorf("identity.user_created success handler failed: %v", err)
		}

		// 16. order.completed - Success (Triggers background workflow)
		bus.Publish(context.Background(), eventbus.Event{
			Type: "order.completed",
			Payload: map[string]any{"customer_id": "c_user"},
		})
		time.Sleep(50 * time.Millisecond)
		// No direct way to check background execution easily without mocking runner,
		// but this covers the handler logic.
	})

	t.Run("Handler Error Paths Surgical", func(t *testing.T) {
		ctx := context.Background()
		// 1. CalculatePersona - Invalid Input
		_, err := mod.CalculatePersona(ctx, "string")
		if err == nil { t.Error("expected error for invalid input type") }
		
		_, err = mod.CalculatePersona(ctx, map[string]any{"wrong": 1})
		if err == nil { t.Error("expected error for missing workflow input") }

		// 2. UpdatePersona - Missing Data
		_, err = mod.UpdatePersona(ctx, map[string]any{})
		if err == nil { t.Error("expected error for missing persona data") }
	})
}
