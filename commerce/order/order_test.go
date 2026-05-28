package order

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/GoHyperrr/hyperrr/internal/workflow"
	"github.com/GoHyperrr/hyperrr/pkg/config"
	"github.com/GoHyperrr/hyperrr/pkg/db"
	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
)

func TestOrderWorkflow(t *testing.T) {
	dbFile := "order_test.db"
	defer os.Remove(dbFile)

	cfg := &config.Config{DBDriver: "sqlite", DBDSN: dbFile}
	database, _ := db.Connect(cfg)
	bus := eventbus.NewInMemBus()
	runner := workflow.NewRunner(bus)
	registryStore := workflow.NewRegistry()

	mod := NewModule()
	mod.Init(context.Background(), &registry.Dependencies{DB: database, EventBus: bus, Runner: runner, Registry: registryStore})
	db.Register(mod.Models()...)
	for name, h := range mod.Handlers() { runner.RegisterTask(name, h) }
	
	// Mock external handlers
	runner.RegisterTask("finance.process_payment", func(ctx context.Context, input any) (any, error) {
		data := input.(map[string]any)
		workflowInput := data["input"].(map[string]any)
		if fail, _ := workflowInput["fail_payment"].(bool); fail {
			return nil, fmt.Errorf("payment gateway rejected transaction")
		}
		return map[string]any{"status": "SUCCESS"}, nil
	})
	runner.RegisterTask("finance.compensate_payment", func(ctx context.Context, input any) (any, error) {
		return nil, nil
	})
	
	// Mock fulfillment handlers
	runner.RegisterTask("fulfillment.reserve_inventory", func(ctx context.Context, input any) (any, error) {
		return nil, nil
	})
	runner.RegisterTask("fulfillment.release_inventory", func(ctx context.Context, input any) (any, error) {
		return nil, nil
	})
	runner.RegisterTask("fulfillment.create_shipment", func(ctx context.Context, input any) (any, error) {
		return nil, nil
	})
	runner.RegisterTask("marketing.add_loyalty_points", func(ctx context.Context, input any) (any, error) {
		return nil, nil
	})

	database.AutoMigrateAll()

	wf := &workflow.Workflow{
		Name: "fulfillment.v1",
		Steps: []workflow.Step{
			{
				ID:   "fulfillment.reserve_inventory",
				Uses: "fulfillment.reserve_inventory",
				Saga: &workflow.Saga{Uses: "fulfillment.release_inventory"},
			},
			{
				ID:   "order.create",
				Uses: "order.create",
				Saga: &workflow.Saga{Uses: "order.compensate_payment"},
				DependsOn: []string{"fulfillment.reserve_inventory"},
			},
			{
				ID:        "finance.process_payment",
				Uses:      "finance.process_payment",
				DependsOn: []string{"order.create"},
				Saga:      &workflow.Saga{Uses: "finance.compensate_payment"},
			},
			{
				ID:         "fulfillment.create_shipment",
				Uses:       "fulfillment.create_shipment",
				DependsOn:  []string{"finance.process_payment"},
			},
			{ID: "order.finalize", Uses: "order.finalize", DependsOn: []string{"fulfillment.create_shipment"}},
			{ID: "marketing.add_loyalty_points", Uses: "marketing.add_loyalty_points", DependsOn: []string{"order.finalize"}},
		},
	}

	input := map[string]any{
		"customer_id": "cust1",
		"cart_id":     "cart1",
		"items": []any{
			map[string]any{"product_id": "p1", "quantity": 1.0, "price": 100.0},
		},
	}

	t.Run("Fulfillment Success Path with Events", func(t *testing.T) {
		// Reset bus to capture events
		testBus := eventbus.NewInMemBus()
		mod.bus = testBus
		
		events := make(chan eventbus.Event, 10)
		testBus.Subscribe(context.Background(), "order.created", func(ctx context.Context, e eventbus.Event) error {
			events <- e
			return nil
		})
		testBus.Subscribe(context.Background(), "order.paid", func(ctx context.Context, e eventbus.Event) error {
			events <- e
			return nil
		})

		res, err := runner.Execute(context.Background(), "f_success_ev", wf, input)
		if err != nil {
			t.Fatalf("workflow failed: %v", err)
		}

		resMap := res["order.finalize"].(map[string]any)
		o := resMap["order"].(*Order)
		if o.Status != OrderPaid {
			t.Errorf("expected PAID status, got %s", o.Status)
		}

		// Verify events
		foundCreated := false
		foundPaid := false
		for i := 0; i < 2; i++ {
			select {
			case e := <-events:
				if e.Type == "order.created" { foundCreated = true }
				if e.Type == "order.paid" { foundPaid = true }
			case <-time.After(100 * time.Millisecond):
			}
		}

		if !foundCreated { t.Error("order.created event not emitted") }
		if !foundPaid { t.Error("order.paid event not emitted") }
	})

	t.Run("Fulfillment Saga Compensation (Payment Fail)", func(t *testing.T) {
		failInput := map[string]any{
			"customer_id":  "cust2",
			"cart_id":      "cart2",
			"fail_payment": true,
			"items": []any{
				map[string]any{"product_id": "p2", "quantity": 1.0, "price": 50.0},
			},
		}

		res, err := runner.Execute(context.Background(), "f_fail", wf, failInput)
		if err == nil || !strings.Contains(err.Error(), "payment gateway rejected transaction") {
			t.Fatalf("expected payment failure error, got %v", err)
		}

		// Check if order was compensated (cancelled)
		resMap := res["order.create"].(map[string]any)
		oCreated := resMap["order"].(*Order)
		refreshed, _ := mod.Repo().GetByID(context.Background(), oCreated.ID)
		if refreshed.Status != OrderCancelled {
			t.Errorf("expected CANCELLED status after compensation, got %s", refreshed.Status)
		}
	})
}

func TestOrderRepository(t *testing.T) {
	dbFile := "order_repo_test.db"
	defer os.Remove(dbFile)
	cfg := &config.Config{DBDriver: "sqlite", DBDSN: dbFile}
	database, _ := db.Connect(cfg)
	
	repo := NewRepository(database)
	database.AutoMigrate(&Order{}, &OrderItem{})

	t.Run("CRUD", func(t *testing.T) {
		o := &Order{ID: "o1", CustomerID: "c1", Status: OrderPending}
		err := repo.Save(context.Background(), o)
		if err != nil { t.Error(err) }

		got, _ := repo.GetByID(context.Background(), "o1")
		if got.ID != "o1" { t.Error("GetByID failed") }

		list, _ := repo.ListByCustomerID(context.Background(), "c1")
		if len(list) != 1 { t.Error("ListByCustomerID failed") }

		all, _ := repo.List(context.Background())
		if len(all) == 0 { t.Error("List failed") }
	})

	t.Run("Handler Error Cases", func(t *testing.T) {
		dbFile := "order_err_test.db"
		defer os.Remove(dbFile)
		cfg := &config.Config{DBDriver: "sqlite", DBDSN: dbFile}
		database, _ := db.Connect(cfg)
		bus := eventbus.NewInMemBus()
		runner := workflow.NewRunner(bus)
		registryStore := workflow.NewRegistry()
		mod := NewModule()
		mod.Init(context.Background(), &registry.Dependencies{DB: database, EventBus: bus, Runner: runner, Registry: registryStore})
		db.Register(mod.Models()...)
		database.AutoMigrateAll()

		// 1. CreateOrder - Invalid Input
		_, err := mod.CreateOrder(context.Background(), "string")
		if err == nil { t.Error("expected error for invalid input type") }
		_, err = mod.CreateOrder(context.Background(), map[string]any{"wrong": 1})
		if err == nil { t.Error("expected error for missing workflow input") }
		_, err = mod.CreateOrder(context.Background(), map[string]any{
			"input": map[string]any{"customer_id": "c1"},
			"items": []any{"invalid_format"}, // should be map[string]any
		})
		if err == nil { t.Error("expected error for malformed item") }

		// 2. FinalizeOrder - Missing Order
		_, err = mod.FinalizeOrder(context.Background(), map[string]any{})
		if err == nil { t.Error("expected error for missing order result") }
		_, err = mod.FinalizeOrder(context.Background(), map[string]any{"order.create": "invalid"})
		if err == nil { t.Error("expected error for invalid result format") }
		_, err = mod.FinalizeOrder(context.Background(), map[string]any{"order.create": map[string]any{"order": "not_an_order"}})
		if err == nil { t.Error("expected error for invalid order type") }

		// 3. CompensatePayment
		res, err := mod.CompensatePayment(context.Background(), map[string]any{})
		if res != nil || err != nil { t.Error("expected nil result") }
		res, err = mod.CompensatePayment(context.Background(), map[string]any{"order.create": "invalid"})
		if res != nil || err != nil { t.Error("expected nil result for invalid order type") }
		res, err = mod.CompensatePayment(context.Background(), map[string]any{"order.create": nil})
		if res != nil || err != nil { t.Error("expected nil result for nil order") }

		// 4. FinalizeOrder - Nil order
		_, err = mod.FinalizeOrder(context.Background(), map[string]any{"order.create": nil})
		if err == nil { t.Error("expected error for nil order in FinalizeOrder") }
		
		// 5. FinalizeOrder - Wrong key
		_, err = mod.FinalizeOrder(context.Background(), map[string]any{"wrong_key": "val"})
		if err == nil { t.Error("expected error for missing order.create key in FinalizeOrder") }

		// 6. CreateOrder - Float vs Int Quantity
		floatInput := map[string]any{
			"input": map[string]any{
				"customer_id": "cf1",
				"cart_id":     "cartf1",
				"items": []any{
					map[string]any{"product_id": "p1", "quantity": 2.5, "price": 100.0}, // float
					map[string]any{"product_id": "p2", "quantity": 3, "price": 50.0},   // int
				},
			},
		}
		res, err = mod.CreateOrder(context.Background(), floatInput)
		if err != nil { t.Fatalf("CreateOrder with mixed quantities failed: %v", err) }
		o := res.(map[string]any)["order"].(*Order)
		if len(o.Items) != 2 { t.Errorf("expected 2 items, got %d", len(o.Items)) }
		if o.Items[0].Quantity != 2 { t.Errorf("expected float 2.5 to be int 2, got %d", o.Items[0].Quantity) }
		if o.Items[1].Quantity != 3 { t.Errorf("expected int 3, got %d", o.Items[1].Quantity) }

		// 7. CreateOrder - Missing/Empty Fields
		missingFields := []map[string]any{
			{"input": map[string]any{"customer_id": "", "cart_id": "c1", "items": []any{map[string]any{"p": "1"}}}},
			{"input": map[string]any{"customer_id": "c1", "cart_id": "", "items": []any{map[string]any{"p": "1"}}}},
			{"input": map[string]any{"customer_id": "c1", "cart_id": "c1", "items": []any{}}},
		}
		for i, inp := range missingFields {
			_, err = mod.CreateOrder(context.Background(), inp)
			if err == nil { t.Errorf("case %d: expected error for missing/empty fields", i) }
		}

		// 8. FinalizeOrder - Missing order.create in result map
		_, err = mod.FinalizeOrder(context.Background(), map[string]any{"other_key": "val"})
		if err == nil { t.Error("expected error for missing order.create key") }

		// 9. CreateOrder - Invalid items (not a map, missing product_id, negative quantity)
		invalidItemsInput := map[string]any{
			"input": map[string]any{
				"customer_id": "c_inv",
				"cart_id":     "cart_inv",
				"items": []any{
					"not-a-map",
					map[string]any{"product_id": "", "quantity": 1, "price": 10.0},
					map[string]any{"product_id": "p1", "quantity": -1, "price": 10.0},
					map[string]any{"product_id": "p2", "quantity": 0, "price": 10.0},
					map[string]any{"product_id": "p3", "quantity": 1, "price": "not-a-float"}, // price will be 0
				},
			},
		}
		res, err = mod.CreateOrder(context.Background(), invalidItemsInput)
		if err != nil { t.Fatalf("CreateOrder should not fail even if some items are invalid: %v", err) }
		o = res.(map[string]any)["order"].(*Order)
		// p3 should be added with price 0
		if len(o.Items) != 1 || o.Items[0].ProductID != "p3" {
			t.Errorf("expected 1 item (p3), got %d items", len(o.Items))
		}

		// 10. CreateOrder - Price as int
		priceIntInput := map[string]any{
			"input": map[string]any{
				"customer_id": "c_int",
				"cart_id":     "cart_int",
				"items": []any{
					map[string]any{"product_id": "p_int", "quantity": 1, "price": 100},
				},
			},
		}
		res, err = mod.CreateOrder(context.Background(), priceIntInput)
		if err != nil { t.Fatalf("CreateOrder with int price failed: %v", err) }
		o = res.(map[string]any)["order"].(*Order)
		if o.TotalPrice != 100.0 { t.Errorf("expected price 100.0, got %f", o.TotalPrice) }

		// 11. FinalizeOrder - Invalid order type
		_, err = mod.FinalizeOrder(context.Background(), map[string]any{"order.create": map[string]any{"order": "not-an-order-struct"}})
		if err == nil { t.Error("expected error for invalid order type in result map") }

		// 12. CompensatePayment - Order does not exist in repo
		// We need to create a module with a repo that fails or returns not found
		// But CompensatePayment only gets the order from the input map.
		// If it's in the input map, it tries to save it.
		// If repo.Save fails, it returns error.
		
		// To test repo.Save failure, we can use a closed DB or similar, but that's overkill.
		// Let's just test the path where order is NOT in the input.
		res, err = mod.CompensatePayment(context.Background(), map[string]any{"order.create": nil})
		if res != nil || err != nil { t.Error("CompensatePayment should return nil, nil if order.create is nil") }
		
		res, err = mod.CompensatePayment(context.Background(), map[string]any{"order.create": map[string]any{"order": "not-an-order"}})
		if res != nil || err != nil { t.Error("CompensatePayment should return nil, nil if order is not an Order struct") }
	})
}
