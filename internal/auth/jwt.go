package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/GoHyperrr/hyperrr/internal/identity"
	"github.com/golang-jwt/jwt/v5"
)

var (
	signingKey      = []byte("hyperrr-secret-key") // Default, should be overwritten via SetSigningKey
	errInvalidToken = errors.New("invalid token")
)

// SetSigningKey sets the secret key used for JWT signing and validation.
func SetSigningKey(key string) {
	signingKey = []byte(key)
}

type Claims struct {
	ActorID   string             `json:"actor_id"`
	ActorType identity.ActorType `json:"actor_type"`
	jwt.RegisteredClaims
}

var store *AuthStore

// SetStore sets the persistence store for auth tokens.
func SetStore(s *AuthStore) {
	store = s
}

// GenerateToken creates a new JWT for an actor.
func GenerateToken(actor identity.Actor) (string, error) {
	jti := fmt.Sprintf("jti_%d", time.Now().UnixNano())
	claims := Claims{
		ActorID:   actor.ID,
		ActorType: actor.Type,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(signingKey)
}

// ValidateToken parses and validates a JWT string.
func ValidateToken(ctx context.Context, tokenString string) (*identity.Actor, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return signingKey, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		// Check blacklist
		if store != nil && store.IsBlacklisted(ctx, claims.ID) {
			return nil, errors.New("token is revoked")
		}

		return &identity.Actor{
			ID:   claims.ActorID,
			Type: claims.ActorType,
		}, nil
	}

	return nil, errInvalidToken
}
