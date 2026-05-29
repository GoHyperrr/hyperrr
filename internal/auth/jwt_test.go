package auth

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/GoHyperrr/hyperrr/internal/identity"
	"github.com/GoHyperrr/hyperrr/pkg/config"
	"github.com/GoHyperrr/hyperrr/pkg/db"
	"github.com/golang-jwt/jwt/v5"
)

func TestJWT(t *testing.T) {
	// Setup a store for testing
	cfg := &config.Config{DBDriver: "sqlite", DBDSN: ":memory:"}
	database, _ := db.Connect(cfg)
	database.AutoMigrate(&Blacklist{})
	store := NewAuthStore(database, "secret", 24*time.Hour)

	actor := identity.Actor{
		ID:   "act_123",
		Type: identity.ActorHuman,
	}

	t.Run("Generate and Validate", func(t *testing.T) {
		token, err := store.GenerateToken(actor)
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		got, err := store.ValidateToken(context.Background(), token)
		if err != nil {
			t.Fatalf("failed to validate token: %v", err)
		}

		if got.ID != actor.ID || got.Type != actor.Type {
			t.Errorf("expected %v, got %v", actor, got)
		}
	})

	t.Run("Invalid Token", func(t *testing.T) {
		_, err := store.ValidateToken(context.Background(), "invalid.token.string")
		if err == nil {
			t.Error("expected error for invalid token, got nil")
		}
	})

	t.Run("Expired Token", func(t *testing.T) {
		// Use a store with zero expiration
		shortStore := NewAuthStore(database, "secret", -time.Hour)
		tokenString, _ := shortStore.GenerateToken(actor)

		_, err := shortStore.ValidateToken(context.Background(), tokenString)
		if err == nil {
			t.Error("expected error for expired token, got nil")
		}
	})

	t.Run("Wrong Signing Method", func(t *testing.T) {
		// Manually create a token string with a different alg in header (RS256 instead of HS256).
		tokenString := "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJhY3Rvcl9pZCI6Indyb25nIn0.sig"

		_, err := store.ValidateToken(context.Background(), tokenString)
		if err == nil || !strings.Contains(err.Error(), "unexpected signing method") {
			t.Errorf("expected unexpected signing method error, got %v", err)
		}
	})

	t.Run("Revoked Token", func(t *testing.T) {
		actor := identity.Actor{ID: "revoked-user", Type: identity.ActorHuman}
		tokenString, _ := store.GenerateToken(actor)
		
		// Get JTI from token
		token, _ := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (interface{}, error) {
			return store.signingKey, nil
		})
		claims := token.Claims.(*Claims)
		
		// Blacklist it
		store.Blacklist(context.Background(), claims.ID, time.Now().Add(time.Hour))
		
		_, err := store.ValidateToken(context.Background(), tokenString)
		if err == nil || !strings.Contains(err.Error(), "token is revoked") {
			t.Errorf("expected revoked error, got %v", err)
		}
	})

	t.Run("Invalid Algorithm", func(t *testing.T) {
		tokenString := "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0.e30."
		_, err := store.ValidateToken(context.Background(), tokenString)
		if err == nil {
			t.Error("expected error for 'none' algorithm")
		}
	})

	t.Run("Malformed Token", func(t *testing.T) {
		_, err := store.ValidateToken(context.Background(), "not.a.token")
		if err == nil {
			t.Error("expected error for malformed token")
		}
	})
}
