package finance

import (
	"context"
	"testing"

	"github.com/GoHyperrr/hyperrr/pkg/workflow"
	"github.com/GoHyperrr/hyperrr/pkg/config"
	"github.com/GoHyperrr/hyperrr/pkg/db"
	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
)

type mockOrder struct {
	ID         string
	TotalPrice float64
	CustomerID string
}

func (m *mockOrder) GetOrderID() string    { return m.ID }
func (m *mockOrder) GetTotal() float64     { return m.TotalPrice }
func (m *mockOrder) GetCustomerID() string { return m.CustomerID }

type flexibleMockOrder struct {
	ID         string
	TotalPrice any
}

func (m *flexibleMockOrder) GetOrderID() string { return m.ID }
func (m *flexibleMockOrder) GetTotal() float64 {
	switch v := m.TotalPrice.(type) {
	case int:
		return float64(v)
	case float64:
		return v
	case int64:
		return float64(v)
	default:
		return 0
	}
}
func (m *flexibleMockOrder) GetCustomerID() string { return "" }

func TestFinanceWorkflow(t *testing.T) {
	cfg := &config.Config{DBDriver: "sqlite", DBDSN: ":memory:"}
	database, _ := db.Connect(cfg)
	bus := eventbus.NewInMemBus()
	runner := workflow.NewRunner(bus, nil, nil)

	mod := NewModule()
	mod.Init(context.Background(), &registry.Dependencies{DB: database, EventBus: bus, Runner: runner})
	db.Register(mod.Models()...)
	for name, h := range mod.Handlers() { runner.RegisterTask(name, h) }
	database.AutoMigrateAll()

	t.Run("Process Payment Success - float64", func(t *testing.T) {
		o := &flexibleMockOrder{ID: "ord1", TotalPrice: 100.50}
		results := map[string]any{
			"order.create": map[string]any{"order": o},
			"input":        map[string]any{},
		}
		resRaw, err := mod.ProcessPayment(context.Background(), results)
		if err != nil { t.Fatalf("failed: %v", err) }
		if resRaw.(map[string]any)["payment"].(*Payment).Amount != 100.50 {
			t.Error("amount mismatch")
		}
	})

	t.Run("Process Payment Success - int", func(t *testing.T) {
		o := &flexibleMockOrder{ID: "ord2", TotalPrice: 200}
		results := map[string]any{
			"order.create": map[string]any{"order": o},
			"input":        map[string]any{},
		}
		resRaw, err := mod.ProcessPayment(context.Background(), results)
		if err != nil { t.Fatalf("failed: %v", err) }
		if resRaw.(map[string]any)["payment"].(*Payment).Amount != 200.0 {
			t.Error("amount mismatch")
		}
	})

	t.Run("Process Payment Failure", func(t *testing.T) {
		o := &mockOrder{ID: "ord2", TotalPrice: 50.0}
		input := map[string]any{"fail_payment": true}
		
		results := map[string]any{
			"order.create": map[string]any{"order": o},
			"input": input,
		}

		_, err := mod.ProcessPayment(context.Background(), results)
		if err == nil {
			t.Error("expected payment failure error")
		}
	})

	t.Run("Idempotency", func(t *testing.T) {
		o := &mockOrder{ID: "ord_idem", TotalPrice: 10.0}
		results := map[string]any{
			"order.create": map[string]any{"order": o},
			"input":        map[string]any{"order_id": "ord_idem"},
			"_workflow_id": "wf_idem_1",
		}
		
		// First call
		_, err := mod.ProcessPayment(context.Background(), results)
		if err != nil { t.Fatal(err) }

		// Second call
		res, err := mod.ProcessPayment(context.Background(), results)
		if err != nil { t.Fatal(err) }
		if res.(map[string]any)["payment"].(*Payment).OrderID != "ord_idem" {
			t.Error("idempotency failed")
		}
	})

	t.Run("Compensate Payment", func(t *testing.T) {
		p := &Payment{ID: "pay1", Status: PaymentSuccess}
		mod.Repo().Save(context.Background(), p)

		results := map[string]any{
			"finance.process_payment": map[string]any{"payment": p},
		}

		resRaw, err := mod.CompensatePayment(context.Background(), results)
		if err != nil {
			t.Fatalf("CompensatePayment failed: %v", err)
		}

		resMap := resRaw.(map[string]any)
		refunded := resMap["payment"].(*Payment)
		if refunded.Status != PaymentRefunded {
			t.Errorf("expected REFUNDED status, got %s", refunded.Status)
		}

		// Compensate Idempotency
		results["_workflow_id"] = "wf_comp_idem"
		mod.CompensatePayment(context.Background(), results)
		resRaw2, _ := mod.CompensatePayment(context.Background(), results)
		if resRaw2 != nil { t.Error("compensate idempotency failed") }
	})
	
	t.Run("Handler Error Cases", func(t *testing.T) {
		// ProcessPayment - Invalid Input
		_, err := mod.ProcessPayment(context.Background(), "string")
		if err == nil { t.Error("expected error for invalid input type") }
		_, err = mod.ProcessPayment(context.Background(), map[string]any{"wrong": 1})
		if err == nil { t.Error("expected error for missing workflow input") }
		_, err = mod.ProcessPayment(context.Background(), map[string]any{"input": map[string]any{}})
		if err == nil { t.Error("expected error for missing order from create step") }
		_, err = mod.ProcessPayment(context.Background(), map[string]any{"input": map[string]any{}, "order.create": "not-a-map"})
		if err == nil { t.Error("expected error for invalid result format") }
		_, err = mod.ProcessPayment(context.Background(), map[string]any{"input": map[string]any{}, "order.create": map[string]any{"order": "not-an-order"}})
		if err == nil { t.Error("expected error for invalid order type") }

		// DB failure
		badMod := NewModule()
		badDB, _ := db.Connect(&config.Config{DBDriver: "sqlite", DBDSN: ":memory:"})
		sqlDB, _ := badDB.DB.DB()
		badMod.repo = NewRepository(badDB)
		sqlDB.Close()
		o := &mockOrder{ID: "ord_bad", TotalPrice: 1.0}
		_, err = badMod.ProcessPayment(context.Background(), map[string]any{"input": map[string]any{}, "order.create": map[string]any{"order": o}})
		if err == nil { t.Error("expected DB error") }

		// CompensatePayment - Invalid Input
		_, err = badMod.CompensatePayment(context.Background(), "string")
		if err == nil { t.Error("expected error for invalid input type") }
		
		// CompensatePayment - Missing Payment (should just skip)
		res, err := mod.CompensatePayment(context.Background(), map[string]any{})
		if err != nil || res != nil { t.Error("CompensatePayment failed on empty input") }
		
		// CompensatePayment - Invalid result map
		res, err = mod.CompensatePayment(context.Background(), map[string]any{"finance.process_payment": "not-a-map"})
		if res != nil { t.Error("expected nil on invalid result map") }
		res, err = mod.CompensatePayment(context.Background(), map[string]any{"finance.process_payment": map[string]any{"payment": "not-a-payment"}})
		if res != nil { t.Error("expected nil on invalid payment type") }
	})
}

func TestFinanceRepository(t *testing.T) {
	cfg := &config.Config{DBDriver: "sqlite", DBDSN: ":memory:"}
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
