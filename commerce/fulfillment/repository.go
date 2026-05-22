package fulfillment

import (
	"context"

	"github.com/GoHyperrr/hyperrr/pkg/db"
)

type Repository struct {
	db *db.DB
}

func NewRepository(database *db.DB) *Repository {
	return &Repository{db: database}
}

// SaveInventory saves an inventory record.
func (r *Repository) SaveInventory(ctx context.Context, inv *Inventory) error {
	return r.db.WithContext(ctx).Save(inv).Error
}

// GetInventoryByProductID retrieves inventory for a product.
func (r *Repository) GetInventoryByProductID(ctx context.Context, productID string) (*Inventory, error) {
	var inv Inventory
	err := r.db.WithContext(ctx).First(&inv, "product_id = ?", productID).Error
	if err != nil {
		return nil, err
	}
	return &inv, nil
}

// SaveShipment saves a shipment record.
func (r *Repository) SaveShipment(ctx context.Context, s *Shipment) error {
	return r.db.WithContext(ctx).Save(s).Error
}

// GetShipment retrieves a shipment by ID.
func (r *Repository) GetShipment(ctx context.Context, id string) (*Shipment, error) {
	var s Shipment
	err := r.db.WithContext(ctx).First(&s, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// GetShipmentByOrderID retrieves a shipment by its order ID.
func (r *Repository) GetShipmentByOrderID(ctx context.Context, orderID string) (*Shipment, error) {
	var s Shipment
	err := r.db.WithContext(ctx).First(&s, "order_id = ?", orderID).Error
	if err != nil {
		return nil, err
	}
	return &s, nil
}
