package finance

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/GoHyperrr/hyperrr/pkg/logger"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
	"github.com/GoHyperrr/hyperrr/pkg/utils"
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

	// Idempotency check
	wfID := utils.GetString(data, "_workflow_id")
	if wfID != "" {
		processed, err := m.repo.db.IsProcessed(ctx, "finance.process_payment", wfID)
		if err != nil {
			return nil, fmt.Errorf("failed to check idempotency: %w", err)
		}
		if processed {
			logger.Info("Payment already processed for this workflow, skipping", "wf_id", wfID)
			// Need to return the payment result to satisfy subsequent steps
			var p Payment
			m.repo.db.WithContext(ctx).Where("order_id = ? AND status = ?", utils.GetString(workflowInput, "order_id"), PaymentSuccess).First(&p)
			return map[string]any{"payment": &p}, nil
		}
	}

	// We need the order created in the previous step
	oRaw, ok := data["order.create"]
	if !ok {
		return nil, fmt.Errorf("missing result from order.create step")
	}
	resMap, ok := oRaw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid result format from order.create")
	}
	o, ok := resMap["order"].(registry.OrderResult)
	if !ok {
		return nil, fmt.Errorf("missing order from order.create result")
	}

	forceFail, _ := workflowInput["fail_payment"].(bool)

	paymentID := "pay_" + uuid.New().String()
	
	p := &Payment{
		ID:      paymentID,
		OrderID: o.GetOrderID(),
		Amount:  o.GetTotal(),
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

	if wfID != "" {
		m.repo.db.MarkProcessed(ctx, "finance.process_payment", wfID)
	}

	logger.Info("Payment processed successfully", "payment_id", p.ID, "order_id", o.GetOrderID(), "amount", o.GetTotal())
	
	return map[string]any{"payment": p}, nil
}

// CompensatePayment handles refunding a payment if a subsequent step fails.
func (m *Module) CompensatePayment(ctx context.Context, input any) (any, error) {
	data, ok := input.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid input type")
	}

	// Idempotency check
	wfID := utils.GetString(data, "_workflow_id")
	if wfID != "" {
		processed, err := m.repo.db.IsProcessed(ctx, "finance.compensate_payment", wfID)
		if err != nil {
			return nil, fmt.Errorf("failed to check idempotency: %w", err)
		}
		if processed {
			logger.Info("Payment already refunded for this workflow, skipping", "wf_id", wfID)
			return nil, nil
		}
	}

	// Check if payment was processed
	resRaw, ok := data["finance.process_payment"]
	if !ok || resRaw == nil {
		return nil, nil // Nothing to compensate if payment wasn't processed
	}
	resMap, ok := resRaw.(map[string]any)
	if !ok {
		return nil, nil
	}
	p, ok := resMap["payment"].(*Payment)
	if !ok {
		return nil, nil
	}

	p.Status = PaymentRefunded
	if err := m.repo.Save(ctx, p); err != nil {
		return nil, err
	}

	if wfID != "" {
		m.repo.db.MarkProcessed(ctx, "finance.compensate_payment", wfID)
	}

	logger.Warn("Saga Compensation: Payment refunded", "payment_id", p.ID)
	return map[string]any{"payment": p}, nil
}
