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
	dbFile := "cart_test.db"
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

	t.Run("Add Item Workflow", func(t *testing.T) {
		// 1. Create a cart
		c := &Cart{ID: "cart1", CustomerID: "cust1", Status: CartActive}
		mod.Repo().Save(context.Background(), c)

		wf := &workflow.Workflow{
			Name: "cart.add_item",
			Steps: []workflow.Step{
				{ID: "add", Uses: "cart.add_item"},
			},
		}

		input := map[string]any{
			"cart_id":    "cart1",
			"product_id": "prod1",
			"quantity":   2,
			"price":      25.0,
		}

		res, err := runner.Execute(context.Background(), "add_1", wf, input)
		if err != nil {
			t.Fatalf("workflow failed: %v", err)
		}

		updated := res["add"].(*Cart)
		if len(updated.Items) != 1 {
			t.Errorf("expected 1 item, got %d", len(updated.Items))
		}
		if updated.Items[0].ProductID != "prod1" || updated.Items[0].Quantity != 2 {
			t.Errorf("item mismatch: %v", updated.Items[0])
		}
	})

	t.Run("Remove Item Workflow", func(t *testing.T) {
		c, _ := mod.Repo().GetByID(context.Background(), "cart1")
		itemID := c.Items[0].ID

		wf := &workflow.Workflow{
			Name: "cart.remove_item",
			Steps: []workflow.Step{
				{ID: "remove", Uses: "cart.remove_item"},
			},
		}

		input := map[string]any{
			"cart_id": "cart1",
			"item_id": itemID,
		}

		res, err := runner.Execute(context.Background(), "remove_1", wf, input)
		if err != nil {
			t.Fatalf("workflow failed: %v", err)
		}

		updated := res["remove"].(*Cart)
		if len(updated.Items) != 0 {
			t.Errorf("expected 0 items, got %d", len(updated.Items))
		}
	})

	t.Run("Checkout Workflow", func(t *testing.T) {
		// Add an item back
		c, _ := mod.Repo().GetByID(context.Background(), "cart1")
		c.Items = append(c.Items, CartItem{ID: "item2", CartID: "cart1", ProductID: "prod2", Quantity: 1, Price: 10.0})
		mod.Repo().Save(context.Background(), c)

		wf := &workflow.Workflow{
			Name: "cart.checkout",
			Steps: []workflow.Step{
				{ID: "checkout", Uses: "cart.checkout"},
			},
		}

		input := map[string]any{
			"cart_id": "cart1",
		}

		_, err := runner.Execute(context.Background(), "checkout_1", wf, input)
		if err != nil {
			t.Fatalf("workflow failed: %v", err)
		}

		final, _ := mod.Repo().GetByID(context.Background(), "cart1")
		if final.Status != CartCompleted {
			t.Errorf("expected COMPLETED status, got %s", final.Status)
		}
	})
}
