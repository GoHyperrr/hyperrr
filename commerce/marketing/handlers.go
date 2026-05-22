package marketing

import (
	"context"
	"fmt"
	"time"

	"github.com/GoHyperrr/hyperrr/commerce/order"
	"github.com/GoHyperrr/hyperrr/pkg/logger"
	"github.com/GoHyperrr/hyperrr/pkg/utils"
)

// ValidateCoupon checks if a coupon is valid and returns it.
func (m *Module) ValidateCoupon(ctx context.Context, input any) (any, error) {
	data, ok := input.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid input type")
	}

	workflowInput, ok := data["input"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("missing workflow input")
	}

	code, _ := workflowInput["coupon_code"].(string)
	if code == "" {
		return nil, fmt.Errorf("coupon code is required")
	}

	coupon, err := m.repo.GetCouponByCode(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("invalid or inactive coupon")
	}

	logger.Info("Coupon validated", "code", code, "discount", coupon.DiscountPercentage)
	return map[string]any{"coupon": coupon}, nil
}

// AddLoyaltyPoints adds points based on the order total.
func (m *Module) AddLoyaltyPoints(ctx context.Context, input any) (any, error) {
	data, ok := input.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid input type")
	}

	// We need the order created in the previous step
	oRaw, ok := data["order.finalize"]
	if !ok {
		// Fallback to order.create if finalize didn't happen yet (unlikely in success path)
		oRaw, ok = data["order.create"]
	}
	if !ok {
		return nil, fmt.Errorf("missing order from previous step")
	}
	
	// Handle both possible structures (direct order or map wrapping it)
	var o *order.Order
	if orderDirect, ok := oRaw.(*order.Order); ok {
		o = orderDirect
	} else if resMap, ok := oRaw.(map[string]any); ok {
		o = resMap["order"].(*order.Order)
	} else {
		return nil, fmt.Errorf("invalid order type in results")
	}

	// Calculate points: 1 point per 10 currency units
	pointsToAdd := int(o.TotalPrice / 10)

	lp, err := m.repo.GetLoyaltyPointsByCustomerID(ctx, o.CustomerID)
	if err != nil {
		// Auto-create loyalty account if it doesn't exist
		lp = &LoyaltyPoints{
			ID:         fmt.Sprintf("lp_%d", time.Now().UnixNano()),
			CustomerID: o.CustomerID,
			Balance:    0,
		}
	}

	// Idempotency check
	wfID := utils.GetString(data, "_workflow_id")
	if wfID != "" {
		if m.repo.db.IsProcessed(ctx, "marketing.add_loyalty_points", wfID) {
			logger.Info("Loyalty points already added for this workflow, skipping", "wf_id", wfID)
			return lp, nil
		}
	}

	lp.Balance += pointsToAdd
	if err := m.repo.SaveLoyaltyPoints(ctx, lp); err != nil {
		return nil, err
	}

	if wfID != "" {
		m.repo.db.MarkProcessed(ctx, "marketing.add_loyalty_points", wfID)
	}

	logger.Info("Loyalty points added", "customer_id", o.CustomerID, "points", pointsToAdd, "new_balance", lp.Balance)
	return lp, nil
}
