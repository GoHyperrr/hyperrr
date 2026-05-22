package order

import (
	"context"
	"fmt"
	"time"

	"github.com/GoHyperrr/hyperrr/pkg/logger"
)

// CreateOrder initializes the order in PENDING state.
func (m *Module) CreateOrder(ctx context.Context, input any) (any, error) {
	data, ok := input.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid input type")
	}

	workflowInput, ok := data["input"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("missing workflow input")
	}

	customerID, _ := workflowInput["customer_id"].(string)
	cartID, _ := workflowInput["cart_id"].(string)
	itemsRaw, ok := workflowInput["items"].([]any)
	if !ok {
		return nil, fmt.Errorf("missing items in input")
	}

	orderID := fmt.Sprintf("ord_%d", time.Now().UnixNano())
	o := &Order{
		ID:         orderID,
		CustomerID: customerID,
		Status:     OrderPending,
	}

	var totalPrice float64
	for _, itemRaw := range itemsRaw {
		itemMap := itemRaw.(map[string]any)
		quantity := int(itemMap["quantity"].(float64))
		price := itemMap["price"].(float64)
		
		o.Items = append(o.Items, OrderItem{
			ID:        fmt.Sprintf("oi_%d_%s", time.Now().UnixNano(), itemMap["product_id"].(string)),
			OrderID:   orderID,
			ProductID: itemMap["product_id"].(string),
			Quantity:  quantity,
			UnitPrice: price,
		})
		totalPrice += price * float64(quantity)
	}
	o.TotalPrice = totalPrice

	if err := m.repo.Save(ctx, o); err != nil {
		return nil, fmt.Errorf("failed to save order: %w", err)
	}

	logger.Info("Order created (Pending)", "order_id", orderID, "cart_id", cartID)
	return o, nil
}

// ProcessPayment simulates payment processing.
func (m *Module) ProcessPayment(ctx context.Context, input any) (any, error) {
	data := input.(map[string]any)
	
	// We need the order created in the previous step
	oRaw, ok := data["order.create"]
	if !ok {
		return nil, fmt.Errorf("missing order from create step")
	}
	o := oRaw.(*Order)

	workflowInput := data["input"].(map[string]any)
	forceFail, _ := workflowInput["fail_payment"].(bool)

	if forceFail {
		return nil, fmt.Errorf("payment gateway rejected transaction")
	}

	logger.Info("Payment processed successfully", "order_id", o.ID, "amount", o.TotalPrice)
	return map[string]any{
		"transaction_id": fmt.Sprintf("tx_%d", time.Now().UnixNano()),
		"status":         "SUCCESS",
	}, nil
}

// FinalizeOrder updates status to PAID.
func (m *Module) FinalizeOrder(ctx context.Context, input any) (any, error) {
	data := input.(map[string]any)
	o := data["order.create"].(*Order)

	o.Status = OrderPaid
	if err := m.repo.Save(ctx, o); err != nil {
		return nil, err
	}

	logger.Info("Order finalized (Paid)", "order_id", o.ID)
	return o, nil
}

// CompensatePayment handles payment failure by cancelling the order.
func (m *Module) CompensatePayment(ctx context.Context, input any) (any, error) {
	data := input.(map[string]any)
	
	// Check if order was created
	oRaw, ok := data["order.create"]
	if !ok {
		return nil, nil // Nothing to compensate if order wasn't even created
	}
	o := oRaw.(*Order)

	o.Status = OrderCancelled
	if err := m.repo.Save(ctx, o); err != nil {
		return nil, err
	}

	logger.Warn("Saga Compensation: Order cancelled due to payment failure", "order_id", o.ID)
	return nil, nil
}
