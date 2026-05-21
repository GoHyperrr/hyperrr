package product

import (
	"context"
	"github.com/GoHyperrr/hyperrr/pkg/db"
)

// Repository handles data access for products.
type Repository struct {
	db *db.DB
}

// NewRepository creates a new Repository.
func NewRepository(database *db.DB) *Repository {
	return &Repository{db: database}
}

// Save persists a product to the database.
func (r *Repository) Save(ctx context.Context, p *Product) error {
	return r.db.WithContext(ctx).Save(p).Error
}

// GetByID retrieves a product by its ID.
func (r *Repository) GetByID(ctx context.Context, id string) (*Product, error) {
	var p Product
	err := r.db.WithContext(ctx).First(&p, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// List returns all products.
func (r *Repository) List(ctx context.Context) ([]*Product, error) {
	var products []*Product
	err := r.db.WithContext(ctx).Find(&products).Error
	return products, err
}

// Delete removes a product from the database.
func (r *Repository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&Product{}, "id = ?", id).Error
}
