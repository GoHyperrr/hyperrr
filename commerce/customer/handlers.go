package customer

import (
	"context"
	"fmt"
)

// CalculatePersona determines a customer's persona using the MLBrainV2.
func (m *Module) CalculatePersona(ctx context.Context, input any) (any, error) {
	data, ok := input.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid input type")
	}

	workflowInput, ok := data["input"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("missing workflow input")
	}

	customerID, _ := workflowInput["customer_id"].(string)
	if customerID == "" {
		return nil, fmt.Errorf("customer_id is required")
	}

	if m.brain == nil {
		return nil, fmt.Errorf("ML brain not initialized")
	}

	persona, err := m.brain.Analyze(ctx, customerID)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"customer_id": customerID,
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

// UpdateCustomerDetails updates the customer's profile information.
func (m *Module) UpdateCustomerDetails(ctx context.Context, input any) (any, error) {
	data, ok := input.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid input type")
	}

	workflowInput, ok := data["input"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("missing workflow input")
	}

	customerID, _ := workflowInput["id"].(string)
	c, err := m.repo.GetByID(ctx, customerID)
	if err != nil {
		return nil, fmt.Errorf("customer not found: %w", err)
	}

	if name, ok := workflowInput["name"].(string); ok && name != "" {
		c.Name = name
	}
	if email, ok := workflowInput["email"].(string); ok && email != "" {
		c.Email = email
	}

	if err := m.repo.Save(ctx, c); err != nil {
		return nil, fmt.Errorf("failed to update customer: %w", err)
	}

	return c, nil
}
