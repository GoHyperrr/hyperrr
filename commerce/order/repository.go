package order

import (
	"context"
	"github.com/GoHyperrr/hyperrr/pkg/db"
)

// Repository handles data access for orders.
type Repository struct {
	db *db.DB
}

// NewRepository creates a new Repository.
func NewRepository(database *db.DB) *Repository {
	return &Repository{db: database}
}

// Save persists an order to the database.
func (r *Repository) Save(ctx context.Context, o *Order) error {
	return r.db.WithContext(ctx).Save(o).Error
}

// GetByID retrieves an order by its ID.
func (r *Repository) GetByID(ctx context.Context, id string) (*Order, error) {
	var o Order
	err := r.db.WithContext(ctx).Preload("Items").First(&o, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &o, nil
}

// ListByCustomerID retrieves all orders for a customer.
func (r *Repository) ListByCustomerID(ctx context.Context, customerID string) ([]*Order, error) {
	var orders []*Order
	err := r.db.WithContext(ctx).Preload("Items").Where("customer_id = ?", customerID).Find(&orders).Error
	return orders, err
}

// List retrieves all orders.
func (r *Repository) List(ctx context.Context) ([]*Order, error) {
	var orders []*Order
	err := r.db.WithContext(ctx).Preload("Items").Find(&orders).Error
	return orders, err
}
