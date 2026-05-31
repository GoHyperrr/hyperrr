package customer

import (
	"context"
	"github.com/GoHyperrr/hyperrr/pkg/db"
)

// Repository handles data access for customers.
type Repository struct {
	db *db.DB
}

// NewRepository creates a new Repository.
func NewRepository(database *db.DB) *Repository {
	return &Repository{db: database}
}

// Save persists a customer to the database.
func (r *Repository) Save(ctx context.Context, c *Customer) error {
	return r.db.WithContext(ctx).Save(c).Error
}

// GetByID retrieves a customer by its ID.
func (r *Repository) GetByID(ctx context.Context, id string) (*Customer, error) {
	var c Customer
	err := r.db.WithContext(ctx).Preload("Addresses").First(&c, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// GetByUserID retrieves a customer by its OS-level UserID.
func (r *Repository) GetByUserID(ctx context.Context, userID string) (*Customer, error) {
	var c Customer
	err := r.db.WithContext(ctx).Preload("Addresses").First(&c, "user_id = ?", userID).Error
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// List retrieves all customers.
func (r *Repository) List(ctx context.Context) ([]*Customer, error) {
	var list []*Customer
	err := r.db.WithContext(ctx).Preload("Addresses").Find(&list).Error
	return list, err
}

