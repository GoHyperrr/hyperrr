package customer

import (
	"context"

	ctxEngine "github.com/GoHyperrr/hyperrr/internal/context"
	"github.com/GoHyperrr/hyperrr/pkg/logger"
)

// MLBrainV2 analyzes a customer's history via the Context Engine to assign a persona.
type MLBrainV2 struct {
	projector *ctxEngine.Projector
}

func NewMLBrainV2(p *ctxEngine.Projector) *MLBrainV2 {
	return &MLBrainV2{projector: p}
}

func (m *MLBrainV2) Analyze(ctx context.Context, customerID string) (string, error) {
	if m.projector == nil {
		return "NEWBIE", nil
	}

	lineages := m.projector.ListLineages()
	
	orderCount := 0
	failureCount := 0

	for _, l := range lineages {
		// Filter by customer if we had that metadata in lineage (future improvement)
		// For now, we analyze all lineages as a proxy for 'system activity' 
		// but in a real system we'd filter by actor/customer.
		
		if l.Name == "fulfillment.v1" && l.State == "COMPLETED" {
			orderCount++
		}
		if l.State == "FAILED" {
			failureCount++
		}
	}

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
