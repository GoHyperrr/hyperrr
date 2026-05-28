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

func (s *AuthStore) SaveRefreshToken(ctx context.Context, t *RefreshToken) error {
	return s.db.WithContext(ctx).Save(t).Error
}

func (s *AuthStore) GetRefreshToken(ctx context.Context, token string) (*RefreshToken, error) {
	var t RefreshToken
	err := s.db.WithContext(ctx).Where("token = ?", token).First(&t).Error
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *AuthStore) RevokeRefreshToken(ctx context.Context, token string) error {
	now := time.Now()
	return s.db.WithContext(ctx).Model(&RefreshToken{}).Where("token = ?", token).Update("revoked_at", &now).Error
}

func (s *AuthStore) DeleteExpiredTokens(ctx context.Context, now time.Time) error {
	return s.db.WithContext(ctx).Where("expires_at < ? OR revoked_at IS NOT NULL", now).Delete(&RefreshToken{}).Error
}
