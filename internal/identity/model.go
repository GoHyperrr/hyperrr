package identity

import (
	"time"

	"gorm.io/gorm"
)

// ActorType represents the type of entity performing an action.
type ActorType string

const (
	ActorHuman   ActorType = "HUMAN"
	ActorAIAgent ActorType = "AI_AGENT"
	ActorSystem  ActorType = "SYSTEM"
)

// Actor represents an identity within the hyperrr OS.
type Actor struct {
	ID        string         `gorm:"primaryKey" json:"id"`
	Type      ActorType      `gorm:"not null" json:"type"`
	Name      string         `json:"name"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// User represents a human entity in the system.
type User struct {
	ID           string         `gorm:"primaryKey" json:"id"`
	Email        string         `gorm:"uniqueIndex;not null" json:"email"`
	PasswordHash string         `gorm:"not null" json:"-"`
	ActorID      string         `gorm:"not null" json:"actor_id"`
	Actor        Actor          `gorm:"foreignKey:ActorID" json:"actor"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

// APIKey represents a secret key used for authentication.
type APIKey struct {
	ID        string         `gorm:"primaryKey" json:"id"`
	Key       string         `gorm:"uniqueIndex;not null" json:"key"`
	ActorID   string         `gorm:"not null" json:"actor_id"`
	Actor     Actor          `gorm:"foreignKey:ActorID" json:"actor"`
	ExpiresAt *time.Time     `json:"expires_at,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}
