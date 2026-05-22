package finance

import (
	"time"

	"gorm.io/gorm"
)

type PaymentStatus string

const (
	PaymentPending  PaymentStatus = "PENDING"
	PaymentSuccess  PaymentStatus = "SUCCESS"
	PaymentFailed   PaymentStatus = "FAILED"
	PaymentRefunded PaymentStatus = "REFUNDED"
)

// Payment represents a financial transaction.
type Payment struct {
	ID        string         `gorm:"primaryKey" json:"id"`
	OrderID   string         `gorm:"index" json:"order_id"`
	Amount    float64        `json:"amount"`
	Status    PaymentStatus  `gorm:"not null" json:"status"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}
