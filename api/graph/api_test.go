package graph

import (
	"context"
	"os"
	"testing"

	"github.com/GoHyperrr/hyperrr/commerce/product"
	"github.com/GoHyperrr/hyperrr/commerce/customer"
	domain "github.com/GoHyperrr/hyperrr/internal/context"
	"github.com/GoHyperrr/hyperrr/pkg/config"
	"github.com/GoHyperrr/hyperrr/pkg/db"
	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
)

func TestResolvers(t *testing.T) {
	ctx := context.Background()
	bus := eventbus.NewInMemBus()
	projector := domain.NewProjector(bus)
	projector.Start(ctx)

	// Setup DB for Product module
	cfg := &config.Config{DBDriver: "sqlite", DBDSN: "api_test.db"}
	database, _ := db.Connect(cfg)
	defer func() {
		// underlying sqlite close
		d, _ := database.DB.DB()
		d.Close()
		os.Remove("api_test.db")
	}()

	prodMod := product.NewModule()
	prodMod.Init(ctx, &registry.Dependencies{DB: database, EventBus: bus})
	db.Register(prodMod.Models()...)

	custMod := customer.NewModule()
	custMod.Init(ctx, &registry.Dependencies{DB: database, EventBus: bus})
	db.Register(custMod.Models()...)

	database.AutoMigrateAll()

	resolver := &Resolver{
		Projector:     projector,
		ProductModule: prodMod,
	}

	t.Run("Health Query", func(t *testing.T) {
		res, err := resolver.Query().Health(ctx)
		if err != nil || res != "OK" {
			t.Errorf("Health failed: %v", err)
		}
	})

	t.Run("Product Resolvers", func(t *testing.T) {
		// Create a product
		p := &product.Product{ID: "p1", Name: "Product 1", Price: 10.0}
		prodMod.Repo().Save(ctx, p)

		res, err := resolver.Query().GetProduct(ctx, "p1")
		if err != nil || res.Name != "Product 1" {
			t.Errorf("GetProduct failed: %v", err)
		}

		// Test not found
		_, err = resolver.Query().GetProduct(ctx, "ghost")
		if err == nil {
			t.Error("expected error for non-existent product")
		}

		list, err := resolver.Query().ListProducts(ctx)
		if err != nil || len(list) == 0 {
			t.Errorf("ListProducts failed: %v", err)
		}
	})

	t.Run("Customer Resolvers", func(t *testing.T) {
		custMod := customer.NewModule()
		custMod.Init(ctx, &registry.Dependencies{DB: database, EventBus: bus})
		
		c := &customer.Customer{ID: "c1", Name: "John Doe", Email: "john@example.com"}
		custMod.Repo().Save(ctx, c)

		resolver.CustomerModule = custMod
		res, err := resolver.Query().GetCustomer(ctx, "c1")
		if err != nil || res.Name != "John Doe" {
			t.Errorf("GetCustomer failed: %v", err)
		}
	})

	t.Run("Context Resolvers", func(t *testing.T) {
		bus.Publish(ctx, eventbus.Event{
			Type: "workflow.started",
			Payload: map[string]any{"id": "wf1", "name": "n", "version": "v"},
		})
		
		res, err := resolver.Query().GetWorkflowLineage(ctx, "wf1")
		if err != nil || res.ID != "wf1" {
			t.Errorf("GetWorkflowLineage failed: %v", err)
		}

		lineages, _ := resolver.Query().ListLineages(ctx)
		if len(lineages) == 0 {
			t.Error("ListLineages empty")
		}

		evs, _ := resolver.WorkflowLineage().Events(ctx, res)
		if len(evs) == 0 {
			t.Error("Events empty")
		}

		rel, _ := resolver.WorkflowLineage().RelatedLineages(ctx, res)
		if rel == nil {
			t.Error("RelatedLineages nil")
		}
	})
}
