package cart

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

func TestCartWorkflow(t *testing.T) {
	t.Run("Add Item Workflow", func(t *testing.T) {
		dbFile := "cart_add_test.db"
		defer os.Remove(dbFile)

		cfg := &config.Config{DBDriver: "sqlite", DBDSN: dbFile}
		database, _ := db.Connect(cfg)
		bus := eventbus.NewInMemBus()
		runner := workflow.NewRunner(bus)

		mod := NewModule()
		mod.Init(context.Background(), &registry.Dependencies{DB: database, EventBus: bus, Runner: runner})
		db.Register(mod.Models()...)
		for name, h := range mod.Handlers() { runner.RegisterTask(name, h) }
		database.AutoMigrateAll()

		c := &Cart{ID: "cart1", CustomerID: "cust1", Status: CartActive}
		mod.Repo().Save(context.Background(), c)

		wf := &workflow.Workflow{
			Steps: []workflow.Step{{ID: "add", Uses: "cart.add_item"}},
		}
		input := map[string]any{"cart_id": "cart1", "product_id": "prod1", "quantity": 2, "price": 25.0}

		res, err := runner.Execute(context.Background(), "add_1", wf, input)
		if err != nil { t.Fatalf("workflow failed: %v", err) }

		updated := res["add"].(*Cart)
		if len(updated.Items) != 1 { t.Errorf("expected 1 item, got %d", len(updated.Items)) }
	})

	t.Run("Remove Item Workflow", func(t *testing.T) {
		dbFile := "cart_remove_test.db"
		defer os.Remove(dbFile)

		cfg := &config.Config{DBDriver: "sqlite", DBDSN: dbFile}
		database, _ := db.Connect(cfg)
		bus := eventbus.NewInMemBus()
		runner := workflow.NewRunner(bus)

		mod := NewModule()
		mod.Init(context.Background(), &registry.Dependencies{DB: database, EventBus: bus, Runner: runner})
		db.Register(mod.Models()...)
		for name, h := range mod.Handlers() { runner.RegisterTask(name, h) }
		database.AutoMigrateAll()

		c := &Cart{ID: "cart1", Items: []CartItem{{ID: "i1", CartID: "cart1", ProductID: "p1", Quantity: 1}}, Status: CartActive}
		mod.Repo().Save(context.Background(), c)

		wf := &workflow.Workflow{
			Steps: []workflow.Step{{ID: "remove", Uses: "cart.remove_item"}},
		}
		input := map[string]any{"cart_id": "cart1", "item_id": "i1"}

		res, err := runner.Execute(context.Background(), "remove_1", wf, input)
		if err != nil { t.Fatalf("workflow failed: %v", err) }

		updated := res["remove"].(*Cart)
		if len(updated.Items) != 0 { t.Errorf("expected 0 items, got %d", len(updated.Items)) }
	})

	t.Run("Checkout Workflow", func(t *testing.T) {
		dbFile := "cart_checkout_test.db"
		defer os.Remove(dbFile)

		cfg := &config.Config{DBDriver: "sqlite", DBDSN: dbFile}
		database, _ := db.Connect(cfg)
		bus := eventbus.NewInMemBus()
		runner := workflow.NewRunner(bus)

		mod := NewModule()
		mod.Init(context.Background(), &registry.Dependencies{DB: database, EventBus: bus, Runner: runner})
		db.Register(mod.Models()...)
		for name, h := range mod.Handlers() { runner.RegisterTask(name, h) }
		database.AutoMigrateAll()

		c := &Cart{ID: "cart1", Items: []CartItem{{ID: "i1", CartID: "cart1", ProductID: "p1", Quantity: 1}}, Status: CartActive}
		mod.Repo().Save(context.Background(), c)

		wf := &workflow.Workflow{
			Steps: []workflow.Step{{ID: "checkout", Uses: "cart.checkout"}},
		}
		_, err := runner.Execute(context.Background(), "checkout_1", wf, map[string]any{"cart_id": "cart1"})
		if err != nil { t.Fatalf("workflow failed: %v", err) }

		final, _ := mod.Repo().GetByID(context.Background(), "cart1")
		if final.Status != CartCompleted { t.Errorf("expected COMPLETED status, got %s", final.Status) }
	})

	t.Run("Handler Error Cases", func(t *testing.T) {
		dbFile := "cart_err_test.db"
		defer os.Remove(dbFile)
		cfg := &config.Config{DBDriver: "sqlite", DBDSN: dbFile}
		database, _ := db.Connect(cfg)
		bus := eventbus.NewInMemBus()
		runner := workflow.NewRunner(bus)
		mod := NewModule()
		mod.Init(context.Background(), &registry.Dependencies{DB: database, EventBus: bus, Runner: runner})
		db.Register(mod.Models()...)
		database.AutoMigrateAll()

		// 1. AddItem - Invalid Input
		_, err := mod.AddItem(context.Background(), "string")
		if err == nil { t.Error("expected error for invalid input type") }

		_, err = mod.AddItem(context.Background(), map[string]any{"wrong": 1})
		if err == nil { t.Error("expected error for missing workflow input") }

		// 2. AddItem - Cart Not Active
		c := &Cart{ID: "c-inactive", Status: CartCompleted}
		mod.Repo().Save(context.Background(), c)
		_, err = mod.AddItem(context.Background(), map[string]any{"input": map[string]any{"cart_id": "c-inactive"}})
		if err == nil { t.Error("expected error for inactive cart") }

		// 3. RemoveItem - Invalid Input
		_, err = mod.RemoveItem(context.Background(), "string")
		if err == nil { t.Error("expected error for invalid input type") }

		_, err = mod.RemoveItem(context.Background(), map[string]any{"wrong": 1})
		if err == nil { t.Error("expected error for missing workflow input") }

		// 4. Checkout - Empty Cart
		c2 := &Cart{ID: "c-empty", Status: CartActive}
		mod.Repo().Save(context.Background(), c2)
		_, err = mod.Checkout(context.Background(), map[string]any{"input": map[string]any{"cart_id": "c-empty"}})
		if err == nil { t.Error("expected error for empty cart") }
	})
}
