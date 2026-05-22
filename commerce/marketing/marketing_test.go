package marketing

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/GoHyperrr/hyperrr/commerce/order"
	"github.com/GoHyperrr/hyperrr/internal/workflow"
	"github.com/GoHyperrr/hyperrr/pkg/config"
	"github.com/GoHyperrr/hyperrr/pkg/db"
	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
)

func TestMarketingModule(t *testing.T) {
	dbFile := fmt.Sprintf("marketing_mod_test_%d.db", time.Now().UnixNano())
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
	database.AutoMigrateAll()

	t.Run("Validate Coupon", func(t *testing.T) {
		code := fmt.Sprintf("SAVE_%d", time.Now().UnixNano())
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
		customerID := fmt.Sprintf("cust_%d", time.Now().UnixNano())
		o := &order.Order{ID: "ord1", CustomerID: customerID, TotalPrice: 150.0}
		
		results := map[string]any{
			"order.finalize": o,
		}

		res, err := mod.AddLoyaltyPoints(context.Background(), results)
		if err != nil {
			t.Fatalf("AddLoyaltyPoints failed: %v", err)
		}

		lp := res.(*LoyaltyPoints)
		if lp.Balance != 15 { // 150 / 10 = 15
			t.Errorf("expected balance 15, got %d", lp.Balance)
		}
	})

	t.Run("Add Loyalty Points Fallback", func(t *testing.T) {
		customerID := fmt.Sprintf("cust_fb_%d", time.Now().UnixNano())
		o := &order.Order{ID: "ord_fb", CustomerID: customerID, TotalPrice: 100.0}
		
		results := map[string]any{
			"order.create": o,
		}

		res, err := mod.AddLoyaltyPoints(context.Background(), results)
		if err != nil {
			t.Fatalf("AddLoyaltyPoints fallback failed: %v", err)
		}

		lp := res.(*LoyaltyPoints)
		if lp.Balance != 10 { 
			t.Errorf("expected balance 10, got %d", lp.Balance)
		}
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
	dbFile := fmt.Sprintf("marketing_repo_test_%d.db", time.Now().UnixNano())
	defer os.Remove(dbFile)
	cfg := &config.Config{DBDriver: "sqlite", DBDSN: dbFile}
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
