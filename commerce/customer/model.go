package customer

import (
	"time"

	"gorm.io/gorm"
)

// Customer represents a business-level customer profile.
type Customer struct {
	ID        string         `gorm:"primaryKey" json:"id"`
	UserID    string         `gorm:"index" json:"user_id"` // OS-level user identity
	Name      string         `json:"name"`
	Email     string         `json:"email"`
	Persona   string         `json:"persona"` // ML-calculated persona
	Addresses []Address      `gorm:"foreignKey:CustomerID" json:"addresses"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// Address represents a physical location associated with a customer.
type Address struct {
	ID         string         `gorm:"primaryKey" json:"id"`
	CustomerID string         `gorm:"index" json:"customer_id"`
	Line1      string         `json:"line1"`
	Line2      string         `json:"line2"`
	City       string         `json:"city"`
	State      string         `json:"state"`
	Zip        string         `json:"zip"`
	Country    string         `json:"country"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
}
