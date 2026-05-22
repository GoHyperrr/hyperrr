package graph

import (
	"encoding/json"
	"github.com/GoHyperrr/hyperrr/internal/context"
	"github.com/GoHyperrr/hyperrr/commerce/cart"
	"github.com/GoHyperrr/hyperrr/commerce/order"
	"github.com/GoHyperrr/hyperrr/commerce/finance"
	"github.com/GoHyperrr/hyperrr/commerce/notification"
	"github.com/GoHyperrr/hyperrr/commerce/fulfillment"
	"github.com/GoHyperrr/hyperrr/commerce/support"
	"github.com/GoHyperrr/hyperrr/commerce/marketing"
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

func mapOrderToModel(o *order.Order) *model.Order {
	res := &model.Order{
		ID:         o.ID,
		CustomerID: o.CustomerID,
		Status:     string(o.Status),
		TotalPrice: o.TotalPrice,
	}
	for _, item := range o.Items {
		res.Items = append(res.Items, &model.OrderItem{
			ID:        item.ID,
			ProductID: item.ProductID,
			Quantity:  item.Quantity,
			UnitPrice: item.UnitPrice,
		})
	}
	return res
}

func mapNotificationToModel(n *notification.Notification) *model.Notification {
	return &model.Notification{
		ID:        n.ID,
		Recipient: n.Recipient,
		Channel:   string(n.Channel),
		Subject:   n.Subject,
		Body:      n.Body,
		Status:    string(n.Status),
		CreatedAt: n.CreatedAt,
	}
}

func mapInventoryToModel(inv *fulfillment.Inventory) *model.Inventory {
	return &model.Inventory{
		ID:                inv.ID,
		ProductID:         inv.ProductID,
		AvailableQuantity: inv.AvailableQuantity,
	}
}

func mapShipmentToModel(s *fulfillment.Shipment) *model.Shipment {
	var trackingNumber *string
	if s.TrackingNumber != "" {
		trackingNumber = &s.TrackingNumber
	}
	var carrier *string
	if s.Carrier != "" {
		carrier = &s.Carrier
	}
	return &model.Shipment{
		ID:             s.ID,
		OrderID:        s.OrderID,
		Status:         string(s.Status),
		TrackingNumber: trackingNumber,
		Carrier:        carrier,
	}
}

func mapPaymentToModel(p *finance.Payment) *model.Payment {
	return &model.Payment{
		ID:      p.ID,
		OrderID: p.OrderID,
		Amount:  p.Amount,
		Status:  string(p.Status),
	}
}

func mapTicketToModel(t *support.Ticket) *model.Ticket {
	res := &model.Ticket{
		ID:         t.ID,
		CustomerID: t.CustomerID,
		Subject:    t.Subject,
		Status:     string(t.Status),
		CreatedAt:  t.CreatedAt,
	}
	for _, m := range t.Messages {
		res.Messages = append(res.Messages, mapMessageToModel(&m))
	}
	return res
}

func mapMessageToModel(m *support.Message) *model.Message {
	return &model.Message{
		ID:        m.ID,
		TicketID:  m.TicketID,
		Sender:    string(m.Sender),
		Content:   m.Content,
		CreatedAt: m.CreatedAt,
	}
}

func mapCouponToModel(c *marketing.Coupon) *model.Coupon {
	return &model.Coupon{
		ID:                 c.ID,
		Code:               c.Code,
		DiscountPercentage: c.DiscountPercentage,
		Active:             c.Active,
	}
}
