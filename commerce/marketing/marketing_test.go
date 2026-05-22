package marketing

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

func TestMarketingModule(t *testing.T) {
	dbFile := "marketing_test.db"
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

	t.Run("Validate Coupon", func(t *testing.T) {
		c := &Coupon{ID: "c1", Code: "SAVE10", DiscountPercentage: 10.0, Active: true}
		mod.Repo().SaveCoupon(context.Background(), c)

		input := map[string]any{
			"input": map[string]any{"coupon_code": "SAVE10"},
		}

		res, err := mod.ValidateCoupon(context.Background(), input)
		if err != nil {
			t.Fatalf("ValidateCoupon failed: %v", err)
		}

		resMap := res.(map[string]any)
		coupon := resMap["coupon"].(*Coupon)
		if coupon.Code != "SAVE10" {
			t.Error("coupon code mismatch")
		}
	})

	t.Run("Add Loyalty Points", func(t *testing.T) {
		o := &order.Order{ID: "ord1", CustomerID: "cust1", TotalPrice: 150.0}
		
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
	dbFile := "marketing_repo_test.db"
	defer os.Remove(dbFile)
	cfg := &config.Config{DBDriver: "sqlite", DBDSN: dbFile}
	database, _ := db.Connect(cfg)
	
	repo := NewRepository(database)
	database.AutoMigrate(&Coupon{}, &LoyaltyPoints{})

	t.Run("Coupon CRUD", func(t *testing.T) {
		c := &Coupon{ID: "c1", Code: "C1", DiscountPercentage: 5.0}
		repo.SaveCoupon(context.Background(), c)

		got, _ := repo.GetCouponByCode(context.Background(), "C1")
		if got.ID != "c1" { t.Error("GetCouponByCode failed") }
	})

	t.Run("Loyalty CRUD", func(t *testing.T) {
		lp := &LoyaltyPoints{ID: "lp1", CustomerID: "cust1", Balance: 100}
		repo.SaveLoyaltyPoints(context.Background(), lp)

		got, _ := repo.GetLoyaltyPointsByCustomerID(context.Background(), "cust1")
		if got.Balance != 100 { t.Error("GetLoyaltyPointsByCustomerID failed") }
	})
}
