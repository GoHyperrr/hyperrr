package identity

import (
	"time"

	ident "github.com/GoHyperrr/hyperrr/pkg/identity"
	"gorm.io/gorm"
)

// User represents a human entity in the system.
type User struct {
	ID           string         `gorm:"primaryKey" json:"id"`
	Email        string         `gorm:"uniqueIndex;not null" json:"email"`
	PasswordHash string         `gorm:"not null" json:"-"`
	ActorID      string         `gorm:"not null" json:"actor_id"`
	Actor        ident.Actor    `gorm:"foreignKey:ActorID" json:"actor"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

// APIKey represents a secret key used for authentication.
type APIKey struct {
	ID        string         `gorm:"primaryKey" json:"id"`
	Key       string         `gorm:"uniqueIndex;not null" json:"key"`
	ActorID   string         `gorm:"not null" json:"actor_id"`
	Actor     ident.Actor    `gorm:"foreignKey:ActorID" json:"actor"`
	ExpiresAt *time.Time     `json:"expires_at,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}
