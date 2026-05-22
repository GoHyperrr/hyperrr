package search

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/GoHyperrr/hyperrr/commerce/product"
	"github.com/GoHyperrr/hyperrr/pkg/logger"
)

// SearchProducts simulates a semantic/vector search across the product catalog.
func (m *Module) SearchProducts(ctx context.Context, input any) (any, error) {
	data, ok := input.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid input type")
	}

	workflowInput, ok := data["input"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("missing workflow input")
	}

	query, _ := workflowInput["query"].(string)
	limit, _ := workflowInput["limit"].(float64)
	if limit <= 0 {
		limit = 10
	}

	actorID, _ := workflowInput["actor_id"].(string)

	if m.prodMod == nil {
		return nil, fmt.Errorf("product module not linked to search")
	}

	// Fetch all products (simplified mock for large catalog)
	allProds, err := m.prodMod.Repo().List(ctx)
	if err != nil {
		return nil, err
	}

	// Simulate semantic search by keyword matching and scoring
	var results []*product.Product
	for _, p := range allProds {
		if strings.Contains(strings.ToLower(p.Name), strings.ToLower(query)) ||
			strings.Contains(strings.ToLower(p.ID), strings.ToLower(query)) {
			results = append(results, p)
		}
		if len(results) >= int(limit) {
			break
		}
	}

	// Track search history
	history := &SearchHistory{
		ID:        fmt.Sprintf("sh_%d", time.Now().UnixNano()),
		Query:     query,
		ActorID:   actorID,
		ResultCount: len(results),
	}
	m.db.WithContext(ctx).Create(history)

	logger.Info("Product search performed", "query", query, "results", len(results))
	return results, nil
}
