package catalog

import (
	"context"
	"fmt"
)

// ValidateProduct checks if the product data is valid.
func (m *Module) ValidateProduct(ctx context.Context, input any) (any, error) {
	data, ok := input.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid input type")
	}

	productData, ok := data["input"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("missing product input")
	}

	name, _ := productData["name"].(string)
	if name == "" {
		return nil, fmt.Errorf("product name is required")
	}

	price, _ := productData["price"].(float64)
	if price <= 0 {
		return nil, fmt.Errorf("product price must be positive")
	}

	return productData, nil
}

// PersistProduct saves the product to the database.
func (m *Module) PersistProduct(ctx context.Context, input any) (any, error) {
	data, ok := input.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid input type")
	}

	// Result from previous step "catalog.validate_product"
	validatedData, ok := data["catalog.validate_product"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("missing validated product data")
	}

	p := &Product{
		ID:          validatedData["id"].(string),
		Name:        validatedData["name"].(string),
		Description: validatedData["description"].(string),
		Price:       validatedData["price"].(float64),
	}

	if err := m.repo.Save(ctx, p); err != nil {
		return nil, fmt.Errorf("failed to save product: %w", err)
	}

	return p, nil
}
