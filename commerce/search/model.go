package search

import (
	"time"

	"gorm.io/gorm"
)

// SearchHistory tracks product searches for analytics and discovery.
type SearchHistory struct {
	ID        string         `gorm:"primaryKey" json:"id"`
	Query     string         `gorm:"not null" json:"query"`
	ActorID   string         `gorm:"index" json:"actor_id"`
	ResultCount int          `json:"result_count"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}
