package support

import (
	"context"
	"fmt"
	"time"

	"github.com/GoHyperrr/hyperrr/pkg/logger"
	"github.com/google/uuid"
)

func (m *Module) CreateTicket(ctx context.Context, input any) (any, error) {
	data, ok := input.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid input type")
	}

	workflowInput, ok := data["input"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("missing workflow input")
	}

	customerID, _ := workflowInput["customer_id"].(string)
	subject, _ := workflowInput["subject"].(string)
	initialMessage, _ := workflowInput["message"].(string)

	ticketID := "tkt_" + uuid.New().String()
	t := &Ticket{
		ID:         ticketID,
		CustomerID: customerID,
		Subject:    subject,
		Status:     TicketOpen,
		Messages: []Message{
			{
				ID:        "msg_" + uuid.New().String(),
				TicketID:  ticketID,
				Sender:    SenderHuman,
				Content:   initialMessage,
				CreatedAt: time.Now(),
			},
		},
	}

	if err := m.repo.SaveTicket(ctx, t); err != nil {
		return nil, err
	}

	logger.Info("Support ticket created", "ticket_id", ticketID, "customer_id", customerID)
	// Must return map for other steps to find "ticket" key in "support.create_ticket" result
	return map[string]any{"ticket": t}, nil
}

func (m *Module) DispatchAIResponse(ctx context.Context, input any) (any, error) {
	data, ok := input.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid input type")
	}

	resRaw, ok := data["ticket"]
	if !ok {
		return nil, fmt.Errorf("missing result from ticket step")
	}
	resMap, ok := resRaw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid result format from ticket step")
	}
	t, ok := resMap["ticket"].(*Ticket)
	if !ok {
		return nil, fmt.Errorf("missing ticket from ticket step result")
	}

	// AI Logic: Placeholder for future Context Engine integration via decoupled registry
	aiContent := "Hello! I am your AI assistant. How can I help you today?"

	msg := &Message{
		ID:        "msg_ai_" + uuid.New().String(),
		TicketID:  t.ID,
		Sender:    SenderAI,
		Content:   aiContent,
		CreatedAt: time.Now(),
	}

	if err := m.repo.SaveMessage(ctx, msg); err != nil {
		return nil, err
	}

	logger.Info("AI response dispatched", "ticket_id", t.ID)
	return map[string]any{"message": msg}, nil
}
