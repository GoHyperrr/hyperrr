package customer

import (
	"context"
	"fmt"

	"github.com/GoHyperrr/hyperrr/pkg/utils"
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

	customerID := utils.GetString(workflowInput, "customer_id")
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

	personaData, ok := data["calculate"].(map[string]any)
	if !ok {
		// Fallback to older step name if needed, but the current DAG uses 'calculate'
		personaData, ok = data["customer.calculate_persona"].(map[string]any)
	}
	if !ok {
		return nil, fmt.Errorf("missing persona data")
	}

	customerID := utils.GetString(personaData, "customer_id")
	persona := utils.GetString(personaData, "persona")

	c, err := m.repo.GetByID(ctx, customerID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch customer: %w", err)
	}

	c.Persona = persona
	if err := m.repo.Save(ctx, c); err != nil {
		return nil, fmt.Errorf("failed to update persona: %w", err)
	}

	return map[string]any{"customer": c}, nil
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

	customerID := utils.GetString(workflowInput, "id")
	c, err := m.repo.GetByID(ctx, customerID)
	if err != nil {
		return nil, fmt.Errorf("customer not found: %w", err)
	}

	if name := utils.GetString(workflowInput, "name"); name != "" {
		c.Name = name
	}
	if email := utils.GetString(workflowInput, "email"); email != "" {
		c.Email = email
	}

	if err := m.repo.Save(ctx, c); err != nil {
		return nil, fmt.Errorf("failed to update customer: %w", err)
	}

	return map[string]any{"customer": c}, nil
}
