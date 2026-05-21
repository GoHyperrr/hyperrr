package graph

import (
	"encoding/json"
	"github.com/GoHyperrr/hyperrr/internal/context"
	"github.com/GoHyperrr/hyperrr/commerce/cart"
	"github.com/GoHyperrr/hyperrr/api/graph/model"
)

func mapToModel(l *context.Lineage) *model.WorkflowLineage {
	res := &model.WorkflowLineage{
		ID:        l.ID,
		Name:      l.Name,
		Version:   l.Version,
		State:     l.State,
		StartedAt: l.StartedAt,
		EndedAt:   l.EndedAt,
	}

	if l.Error != "" {
		res.Error = &l.Error
	}

	for _, s := range l.Steps {
		resStep := &model.StepExecution{
			StepID:    s.StepID,
			State:     s.State,
			StartedAt: s.StartedAt,
			EndedAt:   s.EndedAt,
			Attempts:  s.Attempts,
		}
		if s.Error != "" {
			resStep.Error = &s.Error
		}
		res.Steps = append(res.Steps, resStep)
	}

	for _, e := range l.Events {
		payloadJSON, _ := json.Marshal(e.Payload)
		payloadStr := string(payloadJSON)
		res.Events = append(res.Events, &model.Event{
			ID:        e.ID,
			Type:      e.Type,
			Timestamp: e.Timestamp,
			Payload:   &payloadStr,
		})
	}

	return res
}

func mapCartToModel(c *cart.Cart) *model.Cart {
	res := &model.Cart{
		ID:         c.ID,
		CustomerID: c.CustomerID,
		Status:     string(c.Status),
	}
	for _, item := range c.Items {
		res.Items = append(res.Items, &model.CartItem{
			ID:        item.ID,
			ProductID: item.ProductID,
			Quantity:  item.Quantity,
			Price:     item.Price,
		})
	}
	return res
}
