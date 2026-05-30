package identity

import (
	"time"

	"github.com/GoHyperrr/hyperrr/pkg/db"
)

// ActorType represents the type of entity performing an action.
type ActorType string

const (
	ActorHuman   ActorType = "HUMAN"
	ActorAIAgent ActorType = "AI_AGENT"
	ActorSystem  ActorType = "SYSTEM"
)

// Actor represents a generic identity in the system.
type Actor struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	Type      ActorType `gorm:"index" json:"type"`
	Name      string    `json:"name"`
	Metadata  db.JSONMap `gorm:"type:text" json:"metadata,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
