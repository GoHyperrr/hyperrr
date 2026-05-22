package marketing

import (
	"time"

	"gorm.io/gorm"
)

// Coupon represents a discount code.
type Coupon struct {
	ID                 string         `gorm:"primaryKey" json:"id"`
	Code               string         `gorm:"uniqueIndex;not null" json:"code"`
	DiscountPercentage float64        `gorm:"not null" json:"discount_percentage"`
	Active             bool           `gorm:"default:true" json:"active"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
	DeletedAt          gorm.DeletedAt `gorm:"index" json:"-"`
}

// LoyaltyPoints represents a customer's rewards balance.
type LoyaltyPoints struct {
	ID         string         `gorm:"primaryKey" json:"id"`
	CustomerID string         `gorm:"uniqueIndex;not null" json:"customer_id"`
	Balance    int            `gorm:"default:0" json:"balance"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
}
