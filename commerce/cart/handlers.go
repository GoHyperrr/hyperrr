package cart

import (
	"context"
	"fmt"
	"time"

	"github.com/GoHyperrr/hyperrr/pkg/logger"
)

// AddItem handles adding an item to a cart via workflow.
func (m *Module) AddItem(ctx context.Context, input any) (any, error) {
	data, ok := input.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid input type")
	}

	workflowInput, ok := data["input"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("missing workflow input")
	}

	cartID, _ := workflowInput["cart_id"].(string)
	productID, _ := workflowInput["product_id"].(string)

	var quantity int
	if q, ok := workflowInput["quantity"].(int); ok {
		quantity = q
	} else if qf, ok := workflowInput["quantity"].(float64); ok {
		quantity = int(qf)
	}

	var price float64
	if p, ok := workflowInput["price"].(float64); ok {
		price = p
	}

	if cartID == "" || productID == "" || quantity <= 0 {
		return nil, fmt.Errorf("invalid or missing input fields")
	}

	c, err := m.repo.GetByID(ctx, cartID)
	if err != nil {
		return nil, fmt.Errorf("cart not found: %w", err)
	}

	if c.Status != CartActive {
		return nil, fmt.Errorf("cart is not active")
	}

	// Check if item already exists
	found := false
	for i, item := range c.Items {
		if item.ProductID == productID {
			c.Items[i].Quantity += quantity
			found = true
			break
		}
	}

	if !found {
		c.Items = append(c.Items, CartItem{
			ID:        fmt.Sprintf("ci_%d", time.Now().UnixNano()),
			CartID:    cartID,
			ProductID: productID,
			Quantity:  quantity,
			Price:     price,
		})
	}

	if err := m.repo.Save(ctx, c); err != nil {
		return nil, fmt.Errorf("failed to save cart: %w", err)
	}

	return c, nil
}

// RemoveItem handles removing an item from a cart.
func (m *Module) RemoveItem(ctx context.Context, input any) (any, error) {
	data, ok := input.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid input type")
	}

	workflowInput, ok := data["input"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("missing workflow input")
	}

	cartID, _ := workflowInput["cart_id"].(string)
	itemID, _ := workflowInput["item_id"].(string)

	if err := m.repo.DeleteItem(ctx, cartID, itemID); err != nil {
		return nil, fmt.Errorf("failed to delete item: %w", err)
	}

	return m.repo.GetByID(ctx, cartID)
}

// Checkout handles finalizing the cart.
func (m *Module) Checkout(ctx context.Context, input any) (any, error) {
	data, ok := input.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid input type")
	}

	workflowInput, ok := data["input"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("missing workflow input")
	}

	cartID, _ := workflowInput["cart_id"].(string)
	c, err := m.repo.GetByID(ctx, cartID)
	if err != nil {
		return nil, fmt.Errorf("cart not found: %w", err)
	}

	if len(c.Items) == 0 {
		return nil, fmt.Errorf("cart is empty")
	}

	c.Status = CartCompleted
	if err := m.repo.Save(ctx, c); err != nil {
		return nil, fmt.Errorf("failed to complete cart: %w", err)
	}

	logger.Info("Cart checkout completed", "cart_id", cartID)
	return true, nil
}
