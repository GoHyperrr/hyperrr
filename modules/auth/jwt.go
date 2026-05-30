package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/GoHyperrr/hyperrr/pkg/identity"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var (
	errInvalidToken = errors.New("invalid token")
)

type Claims struct {
	ActorID   string             `json:"actor_id"`
	ActorType identity.ActorType `json:"actor_type"`
	jwt.RegisteredClaims
}

// GenerateToken creates a new JWT for an actor.
func (s *AuthStore) GenerateToken(actor identity.Actor) (string, error) {
	jti := uuid.New().String()
	claims := Claims{
		ActorID:   actor.ID,
		ActorType: actor.Type,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.jwtExpiration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.signingKey)
}

// ValidateToken parses and validates a JWT string.
func (s *AuthStore) ValidateToken(ctx context.Context, tokenString string) (*identity.Actor, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.signingKey, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		// Check blacklist
		revoked, err := s.IsBlacklisted(ctx, claims.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to check token revocation: %w", err)
		}
		if revoked {
			return nil, errors.New("token is revoked")
		}

		return &identity.Actor{
			ID:   claims.ActorID,
			Type: claims.ActorType,
		}, nil
	}

	return nil, errInvalidToken
}
