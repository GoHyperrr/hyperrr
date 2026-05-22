package finance

import (
	"context"
	"os"
	"testing"

	"github.com/GoHyperrr/hyperrr/commerce/order"
	"github.com/GoHyperrr/hyperrr/internal/workflow"
	"github.com/GoHyperrr/hyperrr/pkg/config"
	"github.com/GoHyperrr/hyperrr/pkg/db"
	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
)

func TestFinanceWorkflow(t *testing.T) {
	dbFile := "finance_test.db"
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

	t.Run("Process Payment Success", func(t *testing.T) {
		o := &order.Order{ID: "ord1", TotalPrice: 100.0}
		
		input := map[string]any{}
		
		results := map[string]any{
			"order.create": o,
			"input": input,
		}

		// Since we don't have order.create in this DAG, we'll manually execute the handler to mock the context
		// Wait, runner.Execute starts from scratch, but here we need "order.create" in the results map for the handler to pick up.
		// Let's just call the handler directly.
		res, err := mod.ProcessPayment(context.Background(), results)
		if err != nil {
			t.Fatalf("ProcessPayment failed: %v", err)
		}

		p := res.(*Payment)
		if p.Status != PaymentSuccess {
			t.Errorf("expected SUCCESS status, got %s", p.Status)
		}
		if p.Amount != 100.0 {
			t.Errorf("expected amount 100.0, got %f", p.Amount)
		}
	})

	t.Run("Process Payment Failure", func(t *testing.T) {
		o := &order.Order{ID: "ord2", TotalPrice: 50.0}
		input := map[string]any{"fail_payment": true}
		
		results := map[string]any{
			"order.create": o,
			"input": input,
		}

		_, err := mod.ProcessPayment(context.Background(), results)
		if err == nil {
			t.Error("expected payment failure error")
		}
	})

	t.Run("Compensate Payment", func(t *testing.T) {
		p := &Payment{ID: "pay1", Status: PaymentSuccess}
		mod.Repo().Save(context.Background(), p)

		results := map[string]any{
			"finance.process_payment": p,
		}

		res, err := mod.CompensatePayment(context.Background(), results)
		if err != nil {
			t.Fatalf("CompensatePayment failed: %v", err)
		}

		refunded := res.(*Payment)
		if refunded.Status != PaymentRefunded {
			t.Errorf("expected REFUNDED status, got %s", refunded.Status)
		}
	})
	
	t.Run("Handler Error Cases", func(t *testing.T) {
		// ProcessPayment - Invalid Input
		_, err := mod.ProcessPayment(context.Background(), "string")
		if err == nil { t.Error("expected error for invalid input type") }
		_, err = mod.ProcessPayment(context.Background(), map[string]any{"wrong": 1})
		if err == nil { t.Error("expected error for missing workflow input") }
		_, err = mod.ProcessPayment(context.Background(), map[string]any{"input": map[string]any{}})
		if err == nil { t.Error("expected error for missing order from create step") }

		// CompensatePayment - Invalid Input
		_, err = mod.CompensatePayment(context.Background(), "string")
		if err == nil { t.Error("expected error for invalid input type") }
		
		// CompensatePayment - Missing Payment (should just skip)
		res, err := mod.CompensatePayment(context.Background(), map[string]any{})
		if err != nil || res != nil { t.Error("CompensatePayment failed on empty input") }
	})
}

func TestFinanceRepository(t *testing.T) {
	dbFile := "finance_repo_test.db"
	defer os.Remove(dbFile)
	cfg := &config.Config{DBDriver: "sqlite", DBDSN: dbFile}
	database, _ := db.Connect(cfg)
	
	repo := NewRepository(database)
	database.AutoMigrate(&Payment{})

	t.Run("CRUD", func(t *testing.T) {
		p := &Payment{ID: "pay1", OrderID: "ord1", Amount: 15.0, Status: PaymentSuccess}
		err := repo.Save(context.Background(), p)
		if err != nil { t.Error(err) }

		got, _ := repo.GetByID(context.Background(), "pay1")
		if got.ID != "pay1" { t.Error("GetByID failed") }

		list, _ := repo.ListByOrderID(context.Background(), "ord1")
		if len(list) != 1 { t.Error("ListByOrderID failed") }
	})
}
