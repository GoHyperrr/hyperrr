package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/GoHyperrr/hyperrr/internal/identity"
	"github.com/golang-jwt/jwt/v5"
)

var (
	signingKey = []byte("hyperrr-secret-key") // In a real app, this would be in config
	errInvalidToken = errors.New("invalid token")
)

type Claims struct {
	ActorID   string             `json:"actor_id"`
	ActorType identity.ActorType `json:"actor_type"`
	jwt.RegisteredClaims
}

// GenerateToken creates a new JWT for an actor.
func GenerateToken(actor identity.Actor) (string, error) {
	claims := Claims{
		ActorID:   actor.ID,
		ActorType: actor.Type,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(signingKey)
}

// ValidateToken parses and validates a JWT string.
func ValidateToken(tokenString string) (*identity.Actor, error) {
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
		return &identity.Actor{
			ID:   claims.ActorID,
			Type: claims.ActorType,
		}, nil
	}

	return nil, errInvalidToken
}
