package cart

import (
	"context"
	"github.com/GoHyperrr/hyperrr/pkg/db"
)

// Repository handles data access for carts.
type Repository struct {
	db *db.DB
}

// NewRepository creates a new Repository.
func NewRepository(database *db.DB) *Repository {
	return &Repository{db: database}
}

// Save persists a cart to the database.
func (r *Repository) Save(ctx context.Context, c *Cart) error {
	return r.db.WithContext(ctx).Save(c).Error
}

// GetByID retrieves a cart by its ID.
func (r *Repository) GetByID(ctx context.Context, id string) (*Cart, error) {
	var c Cart
	err := r.db.WithContext(ctx).Preload("Items").First(&c, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// GetActiveByCustomerID retrieves the active cart for a customer.
func (r *Repository) GetActiveByCustomerID(ctx context.Context, customerID string) (*Cart, error) {
	var c Cart
	err := r.db.WithContext(ctx).Preload("Items").First(&c, "customer_id = ? AND status = ?", customerID, CartActive).Error
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// DeleteItem removes an item from a cart.
func (r *Repository) DeleteItem(ctx context.Context, cartID, itemID string) error {
	return r.db.WithContext(ctx).Delete(&CartItem{}, "cart_id = ? AND id = ?", cartID, itemID).Error
}

// ClearItems removes all items from a cart.
func (r *Repository) ClearItems(ctx context.Context, cartID string) error {
	return r.db.WithContext(ctx).Delete(&CartItem{}, "cart_id = ?", cartID).Error
}
