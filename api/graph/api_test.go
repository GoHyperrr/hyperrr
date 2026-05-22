package graph

import (
	"context"
	"os"
	"testing"

	"github.com/GoHyperrr/hyperrr/api/graph/model"
	"github.com/GoHyperrr/hyperrr/commerce/product"
	"github.com/GoHyperrr/hyperrr/commerce/customer"
	"github.com/GoHyperrr/hyperrr/commerce/cart"
	domain "github.com/GoHyperrr/hyperrr/internal/context"
	"github.com/GoHyperrr/hyperrr/internal/identity"
	"github.com/GoHyperrr/hyperrr/internal/workflow"
	"github.com/GoHyperrr/hyperrr/pkg/config"
	"github.com/GoHyperrr/hyperrr/pkg/db"
	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
)

func TestResolvers(t *testing.T) {
	ctx := context.Background()
	bus := eventbus.NewInMemBus()
	runner := workflow.NewRunner(bus)
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
	prodMod.Init(ctx, &registry.Dependencies{DB: database, EventBus: bus, Runner: runner})
	db.Register(prodMod.Models()...)
	for name, h := range prodMod.Handlers() {
		runner.RegisterTask(name, h)
	}

	identMod := identity.NewModule()
	identMod.Init(ctx, &registry.Dependencies{DB: database, EventBus: bus, Runner: runner})
	db.Register(identMod.Models()...)
	for name, h := range identMod.Handlers() {
		runner.RegisterTask(name, h)
	}

	custMod := customer.NewModule()
	custMod.Init(ctx, &registry.Dependencies{DB: database, EventBus: bus, Runner: runner})
	db.Register(custMod.Models()...)
	for name, h := range custMod.Handlers() {
		runner.RegisterTask(name, h)
	}

	cartMod := cart.NewModule()
	cartMod.Init(ctx, &registry.Dependencies{DB: database, EventBus: bus, Runner: runner})
	db.Register(cartMod.Models()...)
	for name, h := range cartMod.Handlers() {
		runner.RegisterTask(name, h)
	}

	database.AutoMigrateAll()

	resolver := &Resolver{
		Projector:      projector,
		ProductModule:  prodMod,
		CustomerModule: custMod,
		CartModule:     cartMod,
		IdentityModule: identMod,
		Runner:         runner,
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

	t.Run("Product Mutations", func(t *testing.T) {
		// Create
		createInput := model.CreateProductInput{
			ID:    "p_new",
			Name:  "New Product",
			Price: 50.0,
		}
		res, err := resolver.Mutation().CreateProduct(ctx, createInput)
		if err != nil || res.Name != "New Product" {
			t.Fatalf("CreateProduct failed: %v", err)
		}

		// Update
		newName := "Updated Name"
		updateInput := model.UpdateProductInput{Name: &newName}
		upRes, err := resolver.Mutation().UpdateProduct(ctx, "p_new", updateInput)
		if err != nil || upRes.Name != newName {
			t.Fatalf("UpdateProduct failed: %v", err)
		}

		// Delete
		delRes, err := resolver.Mutation().DeleteProduct(ctx, "p_new")
		if err != nil || !delRes {
			t.Fatalf("DeleteProduct failed: %v", err)
		}

		// Create failure (missing name)
		_, err = resolver.Mutation().CreateProduct(ctx, model.CreateProductInput{ID: "fail", Price: 10.0})
		if err == nil {
			t.Error("expected error for invalid product create")
		}
	})

	t.Run("Customer Mutations", func(t *testing.T) {
		// Create a customer first
		c := &customer.Customer{ID: "c1", Name: "John Doe", Email: "john@example.com"}
		custMod.Repo().Save(ctx, c)

		// Update
		newName := "Jane Doe"
		updateInput := model.UpdateCustomerInput{Name: &newName}
		upRes, err := resolver.Mutation().UpdateCustomer(ctx, "c1", updateInput)
		if err != nil || upRes.Name != newName {
			t.Fatalf("UpdateCustomer failed: %v", err)
		}

		// GetCustomer with addresses
		c.Addresses = []customer.Address{{ID: "a1", Line1: "123 Main St", City: "NY", State: "NY", Zip: "10001", Country: "USA"}}
		custMod.Repo().Save(ctx, c)

		custRes, err := resolver.Query().GetCustomer(ctx, "c1")
		if err != nil || len(custRes.Addresses) != 1 {
			t.Fatalf("GetCustomer with addresses failed: %v", err)
		}

		// Update failure (non-existent customer)
		_, err = resolver.Mutation().UpdateCustomer(ctx, "ghost", updateInput)
		if err == nil {
			t.Error("expected error for non-existent customer update")
		}
	})

	t.Run("Cart Resolvers", func(t *testing.T) {
		// 1. Get/Create Active Cart
		c, err := resolver.Query().GetActiveCart(ctx, "c1")
		if err != nil || c.CustomerID != "c1" {
			t.Fatalf("GetActiveCart failed: %v", err)
		}

		// 2. Add Item
		addInput := model.AddItemInput{
			ProductID: "p1",
			Quantity:  2,
			Price:     10.0,
		}
		updated, err := resolver.Mutation().AddItemToCart(ctx, c.ID, addInput)
		if err != nil || len(updated.Items) != 1 {
			t.Fatalf("AddItemToCart failed: %v", err)
		}

		// 3. Remove Item
		itemID := updated.Items[0].ID
		afterRemove, err := resolver.Mutation().RemoveItemFromCart(ctx, c.ID, itemID)
		if err != nil || len(afterRemove.Items) != 0 {
			t.Fatalf("RemoveItemFromCart failed: %v", err)
		}

		// 4. Checkout (add item back first)
		resolver.Mutation().AddItemToCart(ctx, c.ID, addInput)
		ok, err := resolver.Mutation().CheckoutCart(ctx, c.ID)
		if err != nil || !ok {
			t.Fatalf("CheckoutCart failed: %v", err)
		}

		final, _ := resolver.Query().GetCart(ctx, c.ID)
		if final.Status != "COMPLETED" {
			t.Errorf("expected COMPLETED status, got %s", final.Status)
		}

		// 5. GetActiveCart (should create new one)
		cNew, err := resolver.Query().GetActiveCart(ctx, "c2")
		if err != nil || cNew.CustomerID != "c2" {
			t.Fatalf("GetActiveCart auto-creation failed: %v", err)
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

		// Test not found
		_, err = resolver.Query().GetWorkflowLineage(ctx, "ghost")
		if err == nil {
			t.Error("expected error for non-existent lineage")
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
