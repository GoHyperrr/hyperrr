package finance

import (
	"context"
	"fmt"
	"time"

	"github.com/GoHyperrr/hyperrr/commerce/order"
	"github.com/GoHyperrr/hyperrr/pkg/logger"
)

// ProcessPayment processes the payment for an order and creates a Payment record.
func (m *Module) ProcessPayment(ctx context.Context, input any) (any, error) {
	data, ok := input.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid input type")
	}

	workflowInput, ok := data["input"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("missing workflow input")
	}

	// We need the order created in the previous step
	oRaw, ok := data["order.create"]
	if !ok {
		return nil, fmt.Errorf("missing order from create step")
	}
	o := oRaw.(*order.Order)

	forceFail, _ := workflowInput["fail_payment"].(bool)

	paymentID := fmt.Sprintf("pay_%d", time.Now().UnixNano())
	
	p := &Payment{
		ID:      paymentID,
		OrderID: o.ID,
		Amount:  o.TotalPrice,
		Status:  PaymentPending,
	}
	
	if err := m.repo.Save(ctx, p); err != nil {
		return nil, fmt.Errorf("failed to save pending payment: %w", err)
	}

	if forceFail {
		p.Status = PaymentFailed
		m.repo.Save(ctx, p)
		return nil, fmt.Errorf("payment gateway rejected transaction")
	}

	p.Status = PaymentSuccess
	if err := m.repo.Save(ctx, p); err != nil {
		return nil, fmt.Errorf("failed to save successful payment: %w", err)
	}

	logger.Info("Payment processed successfully", "payment_id", p.ID, "order_id", o.ID, "amount", o.TotalPrice)
	
	return p, nil
}

// CompensatePayment handles refunding a payment if a subsequent step fails.
func (m *Module) CompensatePayment(ctx context.Context, input any) (any, error) {
	data, ok := input.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid input type")
	}

	// Check if payment was processed
	pRaw, ok := data["finance.process_payment"]
	if !ok || pRaw == nil {
		return nil, nil // Nothing to compensate if payment wasn't processed
	}
	p := pRaw.(*Payment)

	p.Status = PaymentRefunded
	if err := m.repo.Save(ctx, p); err != nil {
		return nil, err
	}

	logger.Warn("Saga Compensation: Payment refunded", "payment_id", p.ID)
	return p, nil
}
