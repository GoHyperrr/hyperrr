package fulfillment

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

type mockOrder struct {
	ID         string
	TotalPrice float64
	CustomerID string
}

func (m *mockOrder) GetOrderID() string    { return m.ID }
func (m *mockOrder) GetTotal() float64     { return m.TotalPrice }
func (m *mockOrder) GetCustomerID() string { return m.CustomerID }

func TestFulfillmentWorkflow(t *testing.T) {
	dbFile := fmt.Sprintf("fulfillment_test_%d.db", time.Now().UnixNano())
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

	t.Run("Reserve Inventory Success", func(t *testing.T) {
		productID := fmt.Sprintf("p_res_%d", time.Now().UnixNano())
		wf := &workflow.Workflow{
			Steps: []workflow.Step{{ID: "reserve", Uses: "fulfillment.reserve_inventory"}},
		}

		input := map[string]any{
			"items": []any{
				map[string]any{"product_id": productID, "quantity": 10.0},
			},
		}

		res, err := runner.Execute(context.Background(), "r1", wf, input)
		if err != nil {
			t.Fatalf("workflow failed: %v", err)
		}

		resRaw := res["reserve"].(map[string]any)
		if resRaw["reserved"] != true {
			t.Error("expected reserved true")
		}

		// Verify inventory was auto-created and decremented (100 - 10 = 90)
		inv, _ := mod.Repo().GetInventoryByProductID(context.Background(), productID)
		if inv.AvailableQuantity != 90 {
			t.Errorf("expected 90, got %d", inv.AvailableQuantity)
		}
	})

	t.Run("Reserve Inventory Failure", func(t *testing.T) {
		productID := fmt.Sprintf("p_fail_%d", time.Now().UnixNano())
		wf := &workflow.Workflow{
			Steps: []workflow.Step{{ID: "reserve", Uses: "fulfillment.reserve_inventory"}},
		}

		input := map[string]any{
			"items": []any{
				map[string]any{"product_id": productID, "quantity": 1000.0}, 
			},
		}

		_, err := runner.Execute(context.Background(), "r2", wf, input)
		if err == nil || !strings.Contains(err.Error(), "insufficient inventory") {
			t.Fatalf("expected insufficient inventory error, got %v", err)
		}
	})

	t.Run("Release Inventory Compensation", func(t *testing.T) {
		productID := fmt.Sprintf("p_rel_%d", time.Now().UnixNano())
		inv := &Inventory{ID: "inv_rel", ProductID: productID, AvailableQuantity: 50}
		mod.Repo().SaveInventory(context.Background(), inv)

		input := map[string]any{
			"fulfillment.reserve_inventory": map[string]any{
				"items": []any{
					map[string]any{"product_id": productID, "quantity": 5.0},
				},
			},
		}

		_, err := mod.ReleaseInventory(context.Background(), input)
		if err != nil {
			t.Fatalf("ReleaseInventory failed: %v", err)
		}

		inv, _ = mod.Repo().GetInventoryByProductID(context.Background(), productID)
		if inv.AvailableQuantity != 55 {
			t.Errorf("expected 55, got %d", inv.AvailableQuantity)
		}
	})

	t.Run("Create Shipment", func(t *testing.T) {
		orderID := fmt.Sprintf("ord_ship_%d", time.Now().UnixNano())
		o := &mockOrder{ID: orderID}

		input := map[string]any{}
		
		results := map[string]any{
			"order.create": map[string]any{"order": o},
			"input": input,
		}

		resRaw, err := mod.CreateShipment(context.Background(), results)
		if err != nil {
			t.Fatalf("CreateShipment failed: %v", err)
		}

		resMap := resRaw.(map[string]any)
		s := resMap["shipment"].(*Shipment)
		if s.Status != ShipmentPending || s.OrderID != orderID {
			t.Error("shipment creation failed")
		}
	})

	t.Run("Ship Order", func(t *testing.T) {
		orderID := fmt.Sprintf("ord_shipped_%d", time.Now().UnixNano())
		shipID := fmt.Sprintf("s_%d", time.Now().UnixNano())
		s := &Shipment{ID: shipID, OrderID: orderID, Status: ShipmentPending}
		mod.Repo().SaveShipment(context.Background(), s)

		wf := &workflow.Workflow{
			Steps: []workflow.Step{{ID: "update", Uses: "fulfillment.ship_order"}},
		}

		input := map[string]any{
			"shipment_id":     shipID,
			"tracking_number": "123XYZ",
			"carrier":         "FedEx",
		}

		res, err := runner.Execute(context.Background(), "ship1", wf, input)
		if err != nil {
			t.Fatalf("ShipOrder failed: %v", err)
		}

		resMap := res["update"].(map[string]any)
		updated := resMap["shipment"].(*Shipment)
		if updated.Status != ShipmentShipped || updated.TrackingNumber != "123XYZ" {
			t.Error("shipment update failed")
		}
	})

	t.Run("Handler Error Cases", func(t *testing.T) {
		// Reserve
		_, err := mod.ReserveInventory(context.Background(), "string")
		if err == nil { t.Error("expected error for invalid input type") }
		_, err = mod.ReserveInventory(context.Background(), map[string]any{"wrong": 1})
		if err == nil { t.Error("expected error for missing workflow input") }
		_, err = mod.ReserveInventory(context.Background(), map[string]any{"input": map[string]any{}})
		if err == nil { t.Error("expected error for missing items") }

		// Release
		_, err = mod.ReleaseInventory(context.Background(), "string")
		if err == nil { t.Error("expected error for invalid input type") }
		res, err := mod.ReleaseInventory(context.Background(), map[string]any{})
		if err != nil || res != nil { t.Error("ReleaseInventory failed on empty input") }

		// Release - Missing reserve key
		res, err = mod.ReleaseInventory(context.Background(), map[string]any{"other": 1})
		if err != nil || res != nil { t.Error("expected nil and no error for missing reserve key in ReleaseInventory") }

		// Create Shipment
		_, err = mod.CreateShipment(context.Background(), "string")
		if err == nil { t.Error("expected error for invalid input type") }
		_, err = mod.CreateShipment(context.Background(), map[string]any{})
		if err == nil { t.Error("expected error for missing order") }

		// Ship Order
		_, err = mod.ShipOrder(context.Background(), "string")
		if err == nil { t.Error("expected error for invalid input type") }
		_, err = mod.ShipOrder(context.Background(), map[string]any{"wrong": 1})
		if err == nil { t.Error("expected error for missing workflow input") }
		_, err = mod.ShipOrder(context.Background(), map[string]any{"input": map[string]any{"shipment_id": "ghost"}})
		if err == nil { t.Error("expected error for missing shipment") }
	})
}

func TestSupportRepository(t *testing.T) {
	dbFile := "fulfillment_repo_test.db"
	defer os.Remove(dbFile)
	cfg := &config.Config{DBDriver: "sqlite", DBDSN: dbFile}
	database, _ := db.Connect(cfg)
	
	repo := NewRepository(database)
	database.AutoMigrate(&Inventory{}, &Shipment{})

	t.Run("CRUD", func(t *testing.T) {
		inv := &Inventory{ID: "i1", ProductID: "p1", AvailableQuantity: 10}
		err := repo.SaveInventory(context.Background(), inv)
		if err != nil { t.Error(err) }

		got, _ := repo.GetInventoryByProductID(context.Background(), "p1")
		if got.AvailableQuantity != 10 { t.Error("GetInventory failed") }

		ship := &Shipment{ID: "s1", OrderID: "o1", Status: ShipmentPending}
		repo.SaveShipment(context.Background(), ship)
		
		s1, _ := repo.GetShipment(context.Background(), "s1")
		if s1.OrderID != "o1" { t.Error("GetShipment failed") }

		s2, err := repo.GetShipmentByOrderID(context.Background(), "o1")
		if err != nil || s2.ID != "s1" { t.Error("GetShipmentByOrderID failed") }

		_, err = repo.GetShipmentByOrderID(context.Background(), "ghost")
		if err == nil { t.Error("expected error for non-existent order shipment") }
	})
}
