package finance

import (
	"context"
	"github.com/GoHyperrr/hyperrr/pkg/db"
)

// Repository handles data access for finance models.
type Repository struct {
	db *db.DB
}

// NewRepository creates a new Repository.
func NewRepository(database *db.DB) *Repository {
	return &Repository{db: database}
}

// Save persists a payment to the database.
func (r *Repository) Save(ctx context.Context, p *Payment) error {
	return r.db.WithContext(ctx).Save(p).Error
}

// GetByID retrieves a payment by its ID.
func (r *Repository) GetByID(ctx context.Context, id string) (*Payment, error) {
	var p Payment
	err := r.db.WithContext(ctx).First(&p, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// ListByOrderID retrieves all payments for an order.
func (r *Repository) ListByOrderID(ctx context.Context, orderID string) ([]*Payment, error) {
	var payments []*Payment
	err := r.db.WithContext(ctx).Where("order_id = ?", orderID).Find(&payments).Error
	return payments, err
}
