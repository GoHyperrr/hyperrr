package customer

import (
	"context"
	"fmt"
)

// CalculatePersona is a mock ML handler that determines a customer's persona.
func (m *Module) CalculatePersona(ctx context.Context, input any) (any, error) {
	data, ok := input.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid input type")
	}

	workflowInput, ok := data["input"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("missing workflow input")
	}

	orderTotal, _ := workflowInput["order_total"].(float64)
	
	// Mock ML logic
	persona := "BRONZE"
	if orderTotal > 1000 {
		persona = "WHALE"
	} else if orderTotal > 500 {
		persona = "GOLD"
	}

	return map[string]any{
		"customer_id": workflowInput["customer_id"],
		"persona":     persona,
	}, nil
}

// UpdatePersona saves the calculated persona to the customer record.
func (m *Module) UpdatePersona(ctx context.Context, input any) (any, error) {
	data, ok := input.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid input type")
	}

	personaData, ok := data["customer.calculate_persona"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("missing persona data")
	}

	customerID := personaData["customer_id"].(string)
	persona := personaData["persona"].(string)

	c, err := m.repo.GetByID(ctx, customerID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch customer: %w", err)
	}

	c.Persona = persona
	if err := m.repo.Save(ctx, c); err != nil {
		return nil, fmt.Errorf("failed to update persona: %w", err)
	}

	return c, nil
}
