package order

import (
	"time"

	"gorm.io/gorm"
)

type OrderStatus string

const (
	OrderPending   OrderStatus = "PENDING"
	OrderPaid      OrderStatus = "PAID"
	OrderFulfilled OrderStatus = "FULFILLED"
	OrderCancelled OrderStatus = "CANCELLED"
)

// Order represents a finalized commerce transaction.
type Order struct {
	ID         string         `gorm:"primaryKey" json:"id"`
	CustomerID string         `gorm:"index" json:"customer_id"`
	Status     OrderStatus    `gorm:"not null" json:"status"`
	TotalPrice float64        `json:"total_price"`
	Items      []OrderItem    `gorm:"foreignKey:OrderID" json:"items"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
}

// OrderItem represents a line item in an order.
type OrderItem struct {
	ID        string         `gorm:"primaryKey" json:"id"`
	OrderID   string         `gorm:"index" json:"order_id"`
	ProductID string         `json:"product_id"`
	Quantity  int            `json:"quantity"`
	UnitPrice float64        `json:"unit_price"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}
