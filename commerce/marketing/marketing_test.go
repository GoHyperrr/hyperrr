package marketing

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/GoHyperrr/hyperrr/internal/workflow"
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

func TestMarketingModule(t *testing.T) {
	cfg := &config.Config{DBDriver: "sqlite", DBDSN: ":memory:"}
	database, _ := db.Connect(cfg)
	bus := eventbus.NewInMemBus()
	runner := workflow.NewRunner(bus, nil, nil)
	registryStore := workflow.NewRegistry()

	mod := NewModule()
	mod.Init(context.Background(), &registry.Dependencies{DB: database, EventBus: bus, Runner: runner, Registry: registryStore})
	db.Register(mod.Models()...)
	for name, h := range mod.Handlers() { runner.RegisterTask(name, h) }
	database.AutoMigrateAll()

	t.Run("Validate Coupon", func(t *testing.T) {
		code := fmt.Sprintf("SAVE_%s", uuid.New().String()[:8])
		c := &Coupon{ID: "c1", Code: code, DiscountPercentage: 10.0, Active: true}
		mod.Repo().SaveCoupon(context.Background(), c)

		input := map[string]any{
			"input": map[string]any{"coupon_code": code},
		}

		res, err := mod.ValidateCoupon(context.Background(), input)
		if err != nil {
			t.Fatalf("ValidateCoupon failed: %v", err)
		}

		resMap := res.(map[string]any)
		coupon := resMap["coupon"].(*Coupon)
		if coupon.Code != code {
			t.Error("coupon code mismatch")
		}
	})

	t.Run("Add Loyalty Points", func(t *testing.T) {
		customerID := fmt.Sprintf("cust_%s", uuid.New().String()[:8])
		o := &mockOrder{ID: "ord1", CustomerID: customerID, TotalPrice: 150.0}
		
		results := map[string]any{
			"order.finalize": map[string]any{"order": o},
		}

		resRaw, err := mod.AddLoyaltyPoints(context.Background(), results)
		if err != nil {
			t.Fatalf("AddLoyaltyPoints failed: %v", err)
		}

		resMap := resRaw.(map[string]any)
		lp := resMap["loyalty_points"].(*LoyaltyPoints)
		if lp.Balance != 15 { // 150 / 10 = 15
			t.Errorf("expected balance 15, got %d", lp.Balance)
		}
	})

	t.Run("Add Loyalty Points Fallback", func(t *testing.T) {
		customerID := fmt.Sprintf("cust_fb_%s", uuid.New().String()[:8])
		o := &mockOrder{ID: "ord_fb", CustomerID: customerID, TotalPrice: 100.0}
		
		results := map[string]any{
			"order.create": map[string]any{"order": o},
		}

		resRaw, err := mod.AddLoyaltyPoints(context.Background(), results)
		if err != nil {
			t.Fatalf("AddLoyaltyPoints fallback failed: %v", err)
		}

		resMap := resRaw.(map[string]any)
		lp := resMap["loyalty_points"].(*LoyaltyPoints)
		if lp.Balance != 10 { 
			t.Errorf("expected balance 10, got %d", lp.Balance)
		}
	})

	t.Run("Handler Error Paths", func(t *testing.T) {
		ctx := context.Background()
		// 1. ValidateCoupon - Invalid Input
		_, err := mod.ValidateCoupon(ctx, "string")
		if err == nil { t.Error("expected error for invalid input type") }

		_, err = mod.ValidateCoupon(ctx, map[string]any{"wrong": 1})
		if err == nil { t.Error("expected error for missing workflow input") }

		// 2. AddLoyaltyPoints - Invalid Input
		_, err = mod.AddLoyaltyPoints(ctx, "string")
		if err == nil { t.Error("expected error for invalid input type") }

		// 3. AddLoyaltyPoints - Missing Order
		_, err = mod.AddLoyaltyPoints(ctx, map[string]any{"input": map[string]any{}})
		if err == nil { t.Error("expected error for missing order result") }
	})

	t.Run("Handler Error Cases", func(t *testing.T) {
		// ValidateCoupon
		_, err := mod.ValidateCoupon(context.Background(), "string")
		if err == nil { t.Error("expected error for invalid input type") }
		_, err = mod.ValidateCoupon(context.Background(), map[string]any{"input": map[string]any{"coupon_code": ""}})
		if err == nil { t.Error("expected error for empty code") }
		_, err = mod.ValidateCoupon(context.Background(), map[string]any{"input": map[string]any{"coupon_code": "GHOST"}})
		if err == nil { t.Error("expected error for invalid code") }

		// AddLoyaltyPoints
		_, err = mod.AddLoyaltyPoints(context.Background(), "string")
		if err == nil { t.Error("expected error for invalid input type") }
		_, err = mod.AddLoyaltyPoints(context.Background(), map[string]any{})
		if err == nil { t.Error("expected error for missing order") }
	})
}

func TestMarketingRepository(t *testing.T) {
	cfg := &config.Config{DBDriver: "sqlite", DBDSN: ":memory:"}
	database, _ := db.Connect(cfg)
	
	repo := NewRepository(database)
	database.AutoMigrate(&Coupon{}, &LoyaltyPoints{})

	t.Run("Coupon CRUD", func(t *testing.T) {
		c := &Coupon{ID: "c_repo", Code: "REPO1", DiscountPercentage: 5.0, Active: true}
		repo.SaveCoupon(context.Background(), c)

		got, err := repo.GetCouponByCode(context.Background(), "REPO1")
		if err != nil || got == nil || got.ID != "c_repo" { 
			t.Fatalf("GetCouponByCode failed: %v", err) 
		}
	})

	t.Run("Loyalty CRUD", func(t *testing.T) {
		lp := &LoyaltyPoints{ID: "lp_repo", CustomerID: "cust_repo", Balance: 100}
		repo.SaveLoyaltyPoints(context.Background(), lp)

		got, err := repo.GetLoyaltyPointsByCustomerID(context.Background(), "cust_repo")
		if err != nil || got == nil || got.Balance != 100 { 
			t.Fatalf("GetLoyaltyPointsByCustomerID failed: %v", err) 
		}
	})
}
