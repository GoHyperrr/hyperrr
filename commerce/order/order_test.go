package order

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

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

	t.Run("Fulfillment Success Path", func(t *testing.T) {
		res, err := runner.Execute(context.Background(), "f_success", wf, input)
		if err != nil {
			t.Fatalf("workflow failed: %v", err)
		}

		o := res["order.finalize"].(*Order)
		if o.Status != OrderPaid {
			t.Errorf("expected PAID status, got %s", o.Status)
		}
		if o.TotalPrice != 100.0 {
			t.Errorf("expected total price 100, got %f", o.TotalPrice)
		}
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
		oCreated := res["order.create"].(*Order)
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

		// 2. FinalizeOrder - Missing Order
		_, err = mod.FinalizeOrder(context.Background(), map[string]any{})
		if err == nil { t.Error("expected error for missing order result") }
	})
}
