package cart

import (
	"context"
	"os"
	"testing"

	"github.com/GoHyperrr/hyperrr/pkg/config"
	"github.com/GoHyperrr/hyperrr/pkg/db"
	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
	"github.com/GoHyperrr/hyperrr/internal/workflow"
)

func TestCartRepository(t *testing.T) {
	dbFile := "cart_repo_test.db"
	defer os.Remove(dbFile)

	cfg := &config.Config{DBDriver: "sqlite", DBDSN: dbFile}
	database, _ := db.Connect(cfg)
	bus := eventbus.NewInMemBus()

	mod := NewModule()
	mod.Init(context.Background(), &registry.Dependencies{DB: database, EventBus: bus, Registry: workflow.NewRegistry()})
	db.Register(mod.Models()...)
	database.AutoMigrateAll()

	t.Run("GetActiveByCustomerID", func(t *testing.T) {
		c2 := &Cart{ID: "cart2", CustomerID: "cust2", Status: CartActive}
		mod.Repo().Save(context.Background(), c2)
		
		got, err := mod.Repo().GetActiveByCustomerID(context.Background(), "cust2")
		if err != nil || got.ID != "cart2" {
			t.Errorf("GetActiveByCustomerID failed: %v", err)
		}
	})

	t.Run("ClearItems", func(t *testing.T) {
		c2 := &Cart{ID: "cart3", CustomerID: "cust3", Status: CartActive}
		mod.Repo().Save(context.Background(), c2)
		
		c2.Items = append(c2.Items, CartItem{ID: "item-c3", CartID: "cart3", ProductID: "p1", Quantity: 1})
		mod.Repo().Save(context.Background(), c2)
		
		err := mod.Repo().ClearItems(context.Background(), "cart3")
		if err != nil {
			t.Fatalf("ClearItems failed: %v", err)
		}
		
		refreshed, _ := mod.Repo().GetByID(context.Background(), "cart3")
		if len(refreshed.Items) != 0 {
			t.Errorf("expected 0 items after clear, got %d", len(refreshed.Items))
		}
	})
}
