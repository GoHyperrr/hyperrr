package customer

import (
	"context"

	"github.com/GoHyperrr/hyperrr/internal/workflow"
	"github.com/GoHyperrr/hyperrr/pkg/logger"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
)

// MLBrainV2 analyzes a customer's history via the Context Engine to assign a persona.
type MLBrainV2 struct {
	projector registry.Projector
}

func NewMLBrainV2(p registry.Projector) *MLBrainV2 {
	return &MLBrainV2{projector: p}
}

func (m *MLBrainV2) Analyze(ctx context.Context, customerID string) (string, error) {
	if m.projector == nil {
		return "NEWBIE", nil
	}

	// Query for successful orders
	orders := m.projector.QueryLineages(func(l registry.LineageData) bool {
		return l.GetName() == "fulfillment.v1" && l.GetState() == workflow.StateCompleted
	})
	
	// Query for failures
	failures := m.projector.QueryLineages(func(l registry.LineageData) bool {
		return l.GetState() == workflow.StateFailed
	})
	
	orderCount := len(orders)
	failureCount := len(failures)

	logger.Info("AI Brain analyzing customer", "customer_id", customerID, "orders", orderCount, "failures", failureCount)

	if orderCount > 5 {
		return "WHALE", nil
	}
	if orderCount > 2 {
		return "GOLD", nil
	}
	if failureCount > 2 {
		return "FRUSTRATED", nil
	}

	return "REGULAR", nil
}
