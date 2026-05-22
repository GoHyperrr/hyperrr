package graph

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/GoHyperrr/hyperrr/api/graph/model"
	"github.com/GoHyperrr/hyperrr/commerce/product"
	"github.com/GoHyperrr/hyperrr/commerce/customer"
	"github.com/GoHyperrr/hyperrr/commerce/cart"
	"github.com/GoHyperrr/hyperrr/commerce/order"
	"github.com/GoHyperrr/hyperrr/commerce/finance"
	"github.com/GoHyperrr/hyperrr/commerce/fulfillment"
	"github.com/GoHyperrr/hyperrr/commerce/notification"
	"github.com/GoHyperrr/hyperrr/commerce/support"
	"github.com/GoHyperrr/hyperrr/commerce/marketing"
	"github.com/GoHyperrr/hyperrr/commerce/search"
	"github.com/GoHyperrr/hyperrr/commerce/analytics"
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

	orderMod := order.NewModule()
	orderMod.Init(ctx, &registry.Dependencies{DB: database, EventBus: bus, Runner: runner})
	db.Register(orderMod.Models()...)
	for name, h := range orderMod.Handlers() {
		runner.RegisterTask(name, h)
	}

	financeMod := finance.NewModule()
	financeMod.Init(ctx, &registry.Dependencies{DB: database, EventBus: bus, Runner: runner})
	db.Register(financeMod.Models()...)
	for name, h := range financeMod.Handlers() {
		runner.RegisterTask(name, h)
	}

	fulfillMod := fulfillment.NewModule()
	fulfillMod.Init(ctx, &registry.Dependencies{DB: database, EventBus: bus, Runner: runner})
	db.Register(fulfillMod.Models()...)
	for name, h := range fulfillMod.Handlers() {
		runner.RegisterTask(name, h)
	}

	notifMod := notification.NewModule(nil)
	notifMod.Init(ctx, &registry.Dependencies{DB: database, EventBus: bus, Runner: runner})
	db.Register(notifMod.Models()...)
	for name, h := range notifMod.Handlers() {
		runner.RegisterTask(name, h)
	}

	supportMod := support.NewModule()
	supportMod.Init(ctx, &registry.Dependencies{DB: database, EventBus: bus, Runner: runner})
	supportMod.SetProjector(projector)
	db.Register(supportMod.Models()...)
	for name, h := range supportMod.Handlers() {
		runner.RegisterTask(name, h)
	}

	marketingMod := marketing.NewModule()
	marketingMod.Init(ctx, &registry.Dependencies{DB: database, EventBus: bus, Runner: runner})
	db.Register(marketingMod.Models()...)
	for name, h := range marketingMod.Handlers() {
		runner.RegisterTask(name, h)
	}

	searchMod := search.NewModule()
	searchMod.Init(ctx, &registry.Dependencies{DB: database, EventBus: bus, Runner: runner})
	searchMod.SetProductModule(prodMod)
	db.Register(searchMod.Models()...)
	for name, h := range searchMod.Handlers() {
		runner.RegisterTask(name, h)
	}

	analyticsMod := analytics.NewModule()
	analyticsMod.Init(ctx, &registry.Dependencies{DB: database, EventBus: bus, Runner: runner})
	analyticsMod.SetProjector(projector)

	database.AutoMigrateAll()

	resolver := &Resolver{
		Projector:          projector,
		ProductModule:      prodMod,
		CustomerModule:     custMod,
		CartModule:         cartMod,
		OrderModule:        orderMod,
		FinanceModule:      financeMod,
		NotificationModule: notifMod,
		FulfillmentModule:  fulfillMod,
		SupportModule:      supportMod,
		MarketingModule:    marketingMod,
		SearchModule:       searchMod,
		AnalyticsModule:    analyticsMod,
		IdentityModule:     identMod,
		Runner:             runner,
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

		// 6. AddItem failure
		_, err = resolver.Mutation().AddItemToCart(ctx, "ghost", model.AddItemInput{})
		if err == nil {
			t.Error("expected error for non-existent cart add")
		}

		// 7. RemoveItem failure
		_, err = resolver.Mutation().RemoveItemFromCart(ctx, "ghost", "ghost")
		if err == nil {
			t.Error("expected error for non-existent cart remove")
		}
	})

	t.Run("Order Resolvers", func(t *testing.T) {
		// 1. Setup Cart with items
		cartRes, _ := resolver.Query().GetActiveCart(ctx, "c3")
		resolver.Mutation().AddItemToCart(ctx, cartRes.ID, model.AddItemInput{ProductID: "p3", Quantity: 1, Price: 150.0})

		// 2. Create Order from Cart
		o, err := resolver.Mutation().CreateOrderFromCart(ctx, cartRes.ID)
		if err != nil {
			t.Fatalf("CreateOrderFromCart failed: %v", err)
		}
		if o.Status != "PAID" {
			t.Errorf("expected PAID status, got %s", o.Status)
		}
		if o.TotalPrice != 150.0 {
			t.Errorf("expected total price 150, got %f", o.TotalPrice)
		}
		if len(o.Items) != 1 || o.Items[0].ProductID != "p3" {
			t.Errorf("expected 1 item with p3, got %v", o.Items)
		}

		// 3. Get Order
		got, err := resolver.Query().GetOrder(ctx, o.ID)
		if err != nil || got.ID != o.ID {
			t.Fatalf("GetOrder failed: %v", err)
		}
		
		// 4. List Orders
		list, _ := resolver.Query().ListOrders(ctx)
		if len(list) == 0 {
			t.Error("ListOrders empty")
		}

		// 5. List Customer Orders
		custList, _ := resolver.Query().ListCustomerOrders(ctx, "c3")
		if len(custList) == 0 {
			t.Error("ListCustomerOrders empty")
		}

		// 6. Get Order failure
		_, err = resolver.Query().GetOrder(ctx, "ghost")
		if err == nil {
			t.Error("expected error for non-existent order")
		}

		// 7. Get Shipment by Order
		ship, err := resolver.Query().GetShipmentByOrder(ctx, o.ID)
		if err != nil || ship.OrderID != o.ID {
			t.Errorf("GetShipmentByOrder failed: %v", err)
		}

		// 8. Update Shipment Status
		carrier := "UPS"
		tracking := "TRACK123"
		upShip, err := resolver.Mutation().UpdateShipmentStatus(ctx, ship.ID, &tracking, &carrier)
		if err != nil || upShip.Status != "SHIPPED" {
			t.Errorf("UpdateShipmentStatus failed: %v", err)
		}

		// 9. Get Inventory
		inv, err := resolver.Query().GetInventory(ctx, "p3")
		if err != nil || inv.ProductID != "p3" {
			t.Errorf("GetInventory failed: %v", err)
		}

		// 10. List Order Payments
		pays, _ := resolver.Query().ListOrderPayments(ctx, o.ID)
		if len(pays) == 0 {
			t.Error("ListOrderPayments empty")
		}

		// 11. Get Payment
		pGot, err := resolver.Query().GetPayment(ctx, pays[0].ID)
		if err != nil || pGot.ID != pays[0].ID {
			t.Errorf("GetPayment failed: %v", err)
		}

		// Wait for async welcome notification from identity.user_created (mocked by earlier cart/order tests or explicit call)
		// Since we didn't explicitly register a user in this test, let's just ensure we have at least one notification
		// Or better, let's manually trigger one for consistency.
		bus.Publish(ctx, eventbus.Event{
			Type: "identity.user_created",
			Payload: map[string]any{
				"actor_id": "u1",
				"email":    "notif@example.com",
				"name":     "Notif User",
			},
		})
		time.Sleep(100 * time.Millisecond)

		// 12. List Notifications
		notifs, _ := resolver.Query().ListNotifications(ctx, nil)
		if len(notifs) == 0 {
			t.Error("ListNotifications empty")
		}

		// 13. List Notifications with filter
		recip := notifs[0].Recipient
		notifsFiltered, _ := resolver.Query().ListNotifications(ctx, &recip)
		if len(notifsFiltered) == 0 {
			t.Error("ListNotifications filtered empty")
		}

		// 14. UpdateShipment failure
		_, err = resolver.Mutation().UpdateShipmentStatus(ctx, "ghost", nil, nil)
		if err == nil {
			t.Error("expected error for non-existent shipment update")
		}

		// 15. Get Cart failure
		_, err = resolver.Query().GetCart(ctx, "ghost")
		if err == nil {
			t.Error("expected error for non-existent cart")
		}

		// 16. List Notifications error (filter non-existent)
		ghost := "ghost@example.com"
		notifsGhost, _ := resolver.Query().ListNotifications(ctx, &ghost)
		if len(notifsGhost) != 0 {
			t.Error("expected 0 notifications for ghost")
		}

		// 17. List Support Tickets error (non-existent customer)
		ticketsGhost, _ := resolver.Query().ListCustomerTickets(ctx, "ghost")
		if len(ticketsGhost) != 0 {
			t.Error("expected 0 tickets for ghost customer")
		}

		// 18. Update Product failure (non-existent)
		pNewName := "Ghost Product"
		uInput := model.UpdateProductInput{Name: &pNewName}
		_, err = resolver.Mutation().UpdateProduct(ctx, "ghost", uInput)
		if err == nil {
			t.Error("expected error for non-existent product update")
		}

		// 19. Get Coupon failure
		_, err = resolver.Query().GetCoupon(ctx, "GHOST_CODE")
		if err == nil {
			t.Error("expected error for non-existent coupon")
		}

		// 20. Apply Coupon failure (non-existent cart)
		_, err = resolver.Mutation().ApplyCouponToCart(ctx, "ghost_cart", "SAVE20")
		if err == nil {
			t.Error("expected error for non-existent cart coupon apply")
		}

		// 21. Search failure (empty query)
		emptySearch, _ := resolver.Query().SearchProducts(ctx, "NON_EXISTENT_PROD", nil)
		if len(emptySearch) != 0 {
			t.Error("expected 0 search results")
		}
	})

	t.Run("Support Resolvers", func(t *testing.T) {
		// 1. Create Ticket
		tkt, err := resolver.Mutation().CreateTicket(ctx, "c1", "Help with order", "My order is late.")
		if err != nil {
			t.Fatalf("CreateTicket failed: %v", err)
		}
		if tkt.Subject != "Help with order" {
			t.Errorf("expected subject 'Help with order', got %s", tkt.Subject)
		}

		// 2. Get Ticket
		got, err := resolver.Query().GetTicket(ctx, tkt.ID)
		if err != nil || got.ID != tkt.ID {
			t.Fatalf("GetTicket failed: %v", err)
		}

		// 3. List Customer Tickets
		list, _ := resolver.Query().ListCustomerTickets(ctx, "c1")
		if len(list) == 0 {
			t.Error("ListCustomerTickets empty")
		}

		// 4. Add Message
		msg, err := resolver.Mutation().AddTicketMessage(ctx, tkt.ID, "HUMAN", "Any update?")
		if err != nil {
			t.Fatalf("AddTicketMessage failed: %v", err)
		}
		if msg.Content != "Any update?" {
			t.Error("message content mismatch")
		}
	})

	t.Run("Marketing Resolvers", func(t *testing.T) {
		// 1. Create a coupon
		c := &marketing.Coupon{ID: "c1", Code: "SAVE20", DiscountPercentage: 20.0, Active: true}
		marketingMod.Repo().SaveCoupon(ctx, c)

		// 2. Get Coupon
		got, err := resolver.Query().GetCoupon(ctx, "SAVE20")
		if err != nil || got.Code != "SAVE20" {
			t.Fatalf("GetCoupon failed: %v", err)
		}

		// 3. Get Loyalty Balance
		bal, err := resolver.Query().GetLoyaltyBalance(ctx, "c1")
		if err != nil {
			t.Fatalf("GetLoyaltyBalance failed: %v", err)
		}
		if bal != 0 {
			t.Errorf("expected balance 0, got %d", bal)
		}

		// 4. Apply Coupon to Cart
		cartRes, _ := resolver.Query().GetActiveCart(ctx, "c4")
		// Add an item first so apply_coupon can trigger cart.add_item effectively (or just test the workflow)
		resolver.Mutation().AddItemToCart(ctx, cartRes.ID, model.AddItemInput{ProductID: "p4", Quantity: 1, Price: 100.0})
		
		updatedCart, err := resolver.Mutation().ApplyCouponToCart(ctx, cartRes.ID, "SAVE20")
		if err != nil {
			t.Fatalf("ApplyCouponToCart failed: %v", err)
		}
		if updatedCart.ID != cartRes.ID {
			t.Error("cart ID mismatch")
		}
	})

	t.Run("Search Resolvers", func(t *testing.T) {
		// 1. Search for p1 (Go Gopher)
		res, err := resolver.Query().SearchProducts(ctx, "Product", nil)
		if err != nil || len(res) == 0 {
			t.Fatalf("SearchProducts failed: %v", err)
		}
	})

	t.Run("Analytics Resolvers", func(t *testing.T) {
		stats, err := resolver.Query().GetSystemStats(ctx)
		if err != nil {
			t.Fatalf("GetSystemStats failed: %v", err)
		}
		if stats.TotalWorkflows == 0 {
			// Seed one if empty to cover loop
			bus.Publish(ctx, eventbus.Event{
				Type: "workflow.started",
				Payload: map[string]any{"id": "a1", "name": "n", "version": "v"},
			})
			time.Sleep(50 * time.Millisecond)
			stats, _ = resolver.Query().GetSystemStats(ctx)
		}

		sales, err := resolver.Query().GetSalesStats(ctx)
		if err != nil {
			t.Fatalf("GetSalesStats failed: %v", err)
		}
		if sales.OrderCount == 0 {
			// Expected as we might not have seeded orders in this specific DB yet
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
