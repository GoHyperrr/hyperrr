package product

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

	// Result from previous step "product.validate_product"
	validatedData, ok := data["product.validate_product"].(map[string]any)
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

// UpdateProductDetails updates an existing product's information.
func (m *Module) UpdateProductDetails(ctx context.Context, input any) (any, error) {
	data, ok := input.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid input type")
	}

	workflowInput, ok := data["input"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("missing workflow input")
	}

	productID, _ := workflowInput["id"].(string)
	p, err := m.repo.GetByID(ctx, productID)
	if err != nil {
		return nil, fmt.Errorf("product not found: %w", err)
	}

	if name, ok := workflowInput["name"].(string); ok && name != "" {
		p.Name = name
	}
	if desc, ok := workflowInput["description"].(string); ok && desc != "" {
		p.Description = desc
	}
	if price, ok := workflowInput["price"].(float64); ok && price > 0 {
		p.Price = price
	}

	if err := m.repo.Save(ctx, p); err != nil {
		return nil, fmt.Errorf("failed to update product: %w", err)
	}

	return p, nil
}
