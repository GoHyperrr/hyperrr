package support

import (
	"time"

	"gorm.io/gorm"
)

type TicketStatus string
type SenderType string

const (
	TicketOpen     TicketStatus = "OPEN"
	TicketResolved TicketStatus = "RESOLVED"
	TicketClosed   TicketStatus = "CLOSED"

	SenderHuman SenderType = "HUMAN"
	SenderAI    SenderType = "AI"
)

// Ticket represents a customer support request.
type Ticket struct {
	ID         string         `gorm:"primaryKey" json:"id"`
	CustomerID string         `gorm:"index;not null" json:"customer_id"`
	Subject    string         `gorm:"not null" json:"subject"`
	Status     TicketStatus   `gorm:"not null" json:"status"`
	Messages   []Message      `gorm:"foreignKey:TicketID" json:"messages"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
}

// Message is a single entry in a support ticket conversation.
type Message struct {
	ID        string         `gorm:"primaryKey" json:"id"`
	TicketID  string         `gorm:"index;not null" json:"ticket_id"`
	Sender    SenderType     `gorm:"not null" json:"sender"`
	Content   string         `gorm:"not null" json:"content"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}
