package cart

import (
	"time"

	"gorm.io/gorm"
)

type CartStatus string

const (
	CartActive    CartStatus = "ACTIVE"
	CartCompleted CartStatus = "COMPLETED"
	CartAbandoned CartStatus = "ABANDONED"
)

// Cart represents a temporary shopping session.
type Cart struct {
	ID         string         `gorm:"primaryKey" json:"id"`
	CustomerID string         `gorm:"index" json:"customer_id"`
	Status     CartStatus     `gorm:"not null" json:"status"`
	Items      []CartItem     `gorm:"foreignKey:CartID" json:"items"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
}

// CartItem represents an item within a cart.
type CartItem struct {
	ID        string         `gorm:"primaryKey" json:"id"`
	CartID    string         `gorm:"index" json:"cart_id"`
	ProductID string         `json:"product_id"`
	Quantity  int            `json:"quantity"`
	Price     float64        `json:"price"` // Price at the time of adding to cart
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}
