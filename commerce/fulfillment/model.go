package fulfillment

import (
	"time"

	"gorm.io/gorm"
)

type ShipmentStatus string

const (
	ShipmentPending   ShipmentStatus = "PENDING"
	ShipmentShipped   ShipmentStatus = "SHIPPED"
	ShipmentDelivered ShipmentStatus = "DELIVERED"
	ShipmentCancelled ShipmentStatus = "CANCELLED"
)

// Inventory represents the available stock for a product.
type Inventory struct {
	ID                string         `gorm:"primaryKey" json:"id"`
	ProductID         string         `gorm:"uniqueIndex;not null" json:"product_id"`
	AvailableQuantity int            `gorm:"not null" json:"available_quantity"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
	DeletedAt         gorm.DeletedAt `gorm:"index" json:"-"`
}

// Shipment represents the logistics fulfillment of an order.
type Shipment struct {
	ID             string         `gorm:"primaryKey" json:"id"`
	OrderID        string         `gorm:"uniqueIndex;not null" json:"order_id"`
	Status         ShipmentStatus `gorm:"not null" json:"status"`
	TrackingNumber string         `json:"tracking_number"`
	Carrier        string         `json:"carrier"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
}
