package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/GoHyperrr/hyperrr/pkg/db"
)

// RefreshToken represents a long-lived token used to generate new JWTs.
type RefreshToken struct {
	ID        string    `gorm:"primaryKey"`
	ActorID   string    `gorm:"index;not null"`
	Token     string    `gorm:"uniqueIndex;not null"`
	ExpiresAt time.Time `gorm:"not null"`
	RevokedAt *time.Time
	CreatedAt time.Time
}

// Blacklist tracks revoked JWT IDs.
type Blacklist struct {
	ID        string    `gorm:"primaryKey"`
	JTI       string    `gorm:"uniqueIndex;not null"`
	ExpiresAt time.Time `gorm:"index;not null"`
}

// AuthStore handles persistence for authentication tokens.
type AuthStore struct {
	db *db.DB
}

func NewAuthStore(database *db.DB) *AuthStore {
	return &AuthStore{db: database}
}

func (s *AuthStore) Blacklist(ctx context.Context, jti string, expiresAt time.Time) error {
	return s.db.WithContext(ctx).Create(&Blacklist{
		ID:        fmt.Sprintf("bl_%d", time.Now().UnixNano()),
		JTI:       jti,
		ExpiresAt: expiresAt,
	}).Error
}

func (s *AuthStore) IsBlacklisted(ctx context.Context, jti string) bool {
	var b Blacklist
	err := s.db.WithContext(ctx).First(&b, "jti = ?", jti).Error
	return err == nil
}
