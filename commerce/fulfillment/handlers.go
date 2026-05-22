package fulfillment

import (
	"context"
	"fmt"
	"time"

	"github.com/GoHyperrr/hyperrr/commerce/order"
	"github.com/GoHyperrr/hyperrr/pkg/logger"
)

func (m *Module) ReserveInventory(ctx context.Context, input any) (any, error) {
	data, ok := input.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid input type")
	}

	workflowInput, ok := data["input"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("missing workflow input")
	}

	itemsRaw, ok := workflowInput["items"].([]any)
	if !ok {
		return nil, fmt.Errorf("missing items in input")
	}

	// We will reserve all items or fail.
	var reservedItems []string

	for _, itemRaw := range itemsRaw {
		itemMap := itemRaw.(map[string]any)
		productID := itemMap["product_id"].(string)
		quantity := int(itemMap["quantity"].(float64))

		inv, err := m.repo.GetInventoryByProductID(ctx, productID)
		if err != nil {
			// In MVP, we might auto-create inventory if it doesn't exist just for testing
			inv = &Inventory{
				ID:                fmt.Sprintf("inv_%d", time.Now().UnixNano()),
				ProductID:         productID,
				AvailableQuantity: 100, // Mock stock
			}
		}

		if inv.AvailableQuantity < quantity {
			return nil, fmt.Errorf("insufficient inventory for product %s", productID)
		}

		inv.AvailableQuantity -= quantity
		if err := m.repo.SaveInventory(ctx, inv); err != nil {
			return nil, fmt.Errorf("failed to update inventory: %w", err)
		}
		reservedItems = append(reservedItems, productID)
	}

	logger.Info("Inventory reserved successfully", "items", reservedItems)
	return map[string]any{"reserved": true, "items": itemsRaw}, nil
}

func (m *Module) ReleaseInventory(ctx context.Context, input any) (any, error) {
	data, ok := input.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid input type")
	}

	// Get what was reserved
	resRaw, ok := data["fulfillment.reserve_inventory"]
	if !ok {
		return nil, nil // Nothing to release
	}
	resMap := resRaw.(map[string]any)
	itemsRaw := resMap["items"].([]any)

	for _, itemRaw := range itemsRaw {
		itemMap := itemRaw.(map[string]any)
		productID := itemMap["product_id"].(string)
		quantity := int(itemMap["quantity"].(float64))

		inv, err := m.repo.GetInventoryByProductID(ctx, productID)
		if err == nil {
			inv.AvailableQuantity += quantity
			m.repo.SaveInventory(ctx, inv)
		}
	}

	logger.Warn("Saga Compensation: Inventory released")
	return nil, nil
}

func (m *Module) CreateShipment(ctx context.Context, input any) (any, error) {
	data, ok := input.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid input type")
	}

	oRaw, ok := data["order.create"]
	if !ok {
		return nil, fmt.Errorf("missing order from create step")
	}
	o := oRaw.(*order.Order)

	s := &Shipment{
		ID:      fmt.Sprintf("shp_%d", time.Now().UnixNano()),
		OrderID: o.ID,
		Status:  ShipmentPending,
	}

	if err := m.repo.SaveShipment(ctx, s); err != nil {
		return nil, fmt.Errorf("failed to save shipment: %w", err)
	}

	logger.Info("Shipment created", "shipment_id", s.ID, "order_id", o.ID)
	return s, nil
}

// ShipOrder simulates shipping the order (updates status to SHIPPED and adds tracking number).
func (m *Module) ShipOrder(ctx context.Context, input any) (any, error) {
	data, ok := input.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid input type")
	}
	
	workflowInput, ok := data["input"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("missing workflow input")
	}

	shipmentID, _ := workflowInput["shipment_id"].(string)
	trackingNumber, _ := workflowInput["tracking_number"].(string)
	carrier, _ := workflowInput["carrier"].(string)

	s, err := m.repo.GetShipment(ctx, shipmentID)
	if err != nil {
		return nil, fmt.Errorf("shipment not found: %w", err)
	}

	s.Status = ShipmentShipped
	if trackingNumber != "" {
		s.TrackingNumber = trackingNumber
	}
	if carrier != "" {
		s.Carrier = carrier
	}

	if err := m.repo.SaveShipment(ctx, s); err != nil {
		return nil, fmt.Errorf("failed to update shipment: %w", err)
	}

	logger.Info("Shipment shipped", "shipment_id", s.ID, "tracking_number", s.TrackingNumber)
	return s, nil
}
