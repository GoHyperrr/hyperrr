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

	if customerID == "" || cartID == "" || len(itemsRaw) == 0 {
		return nil, fmt.Errorf("invalid or missing input fields")
	}

	orderID := fmt.Sprintf("ord_%d", time.Now().UnixNano())
	o := &Order{
		ID:         orderID,
		CustomerID: customerID,
		Status:     OrderPending,
	}

	var totalPrice float64
	for _, itemRaw := range itemsRaw {
		itemMap, ok := itemRaw.(map[string]any)
		if !ok {
			continue
		}

		var quantity int
		if q, ok := itemMap["quantity"].(int); ok {
			quantity = q
		} else if qf, ok := itemMap["quantity"].(float64); ok {
			quantity = int(qf)
		}

		var price float64
		if p, ok := itemMap["price"].(float64); ok {
			price = p
		}

		productID, _ := itemMap["product_id"].(string)
		if productID == "" || quantity <= 0 {
			continue
		}
		
		o.Items = append(o.Items, OrderItem{
			ID:        fmt.Sprintf("oi_%d_%s", time.Now().UnixNano(), productID),
			OrderID:   orderID,
			ProductID: productID,
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

// FinalizeOrder updates status to PAID.
func (m *Module) FinalizeOrder(ctx context.Context, input any) (any, error) {
	data, ok := input.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid input type")
	}

	oRaw, ok := data["order.create"]
	if !ok || oRaw == nil {
		return nil, fmt.Errorf("missing order from create step")
	}
	o, ok := oRaw.(*Order)
	if !ok {
		return nil, fmt.Errorf("invalid order type in create step")
	}

	o.Status = OrderPaid
	if err := m.repo.Save(ctx, o); err != nil {
		return nil, err
	}

	logger.Info("Order finalized (Paid)", "order_id", o.ID)
	return o, nil
}

// CompensatePayment handles payment failure by cancelling the order.
func (m *Module) CompensatePayment(ctx context.Context, input any) (any, error) {
	data, ok := input.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid input type")
	}

	// Check if order was created
	oRaw, ok := data["order.create"]
	if !ok || oRaw == nil {
		return nil, nil // Nothing to compensate if order wasn't even created
	}
	o, ok := oRaw.(*Order)
	if !ok {
		return nil, fmt.Errorf("invalid order type in create step")
	}

	o.Status = OrderCancelled
	if err := m.repo.Save(ctx, o); err != nil {
		return nil, err
	}

	logger.Warn("Saga Compensation: Order cancelled due to payment failure", "order_id", o.ID)
	return nil, nil
}
