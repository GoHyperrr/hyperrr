package auth

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/GoHyperrr/hyperrr/internal/identity"
	"github.com/GoHyperrr/hyperrr/pkg/config"
	"github.com/GoHyperrr/hyperrr/pkg/db"
	"github.com/golang-jwt/jwt/v5"
)

func TestJWT(t *testing.T) {
	actor := identity.Actor{
		ID:   "act_123",
		Type: identity.ActorHuman,
	}

	t.Run("Generate and Validate", func(t *testing.T) {
		token, err := GenerateToken(actor)
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		got, err := ValidateToken(context.Background(), token)
		if err != nil {
			t.Fatalf("failed to validate token: %v", err)
		}

		if got.ID != actor.ID || got.Type != actor.Type {
			t.Errorf("expected %v, got %v", actor, got)
		}
	})

	t.Run("Invalid Token", func(t *testing.T) {
		_, err := ValidateToken(context.Background(), "invalid.token.string")
		if err == nil {
			t.Error("expected error for invalid token, got nil")
		}
	})

	t.Run("Revoked Token", func(t *testing.T) {
		// Mock DB for store
		cfg := &config.Config{DBDriver: "sqlite", DBDSN: "auth_test_bl.db"}
		database, _ := db.Connect(cfg)
		defer os.Remove("auth_test_bl.db")
		database.AutoMigrate(&Blacklist{})
		
		s := NewAuthStore(database)
		SetStore(s)
		defer SetStore(nil)

		actor := identity.Actor{ID: "revoked-user", Type: identity.ActorHuman}
		tokenString, _ := GenerateToken(actor)
		
		// Get JTI from token
		token, _ := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (interface{}, error) {
			return signingKey, nil
		})
		claims := token.Claims.(*Claims)
		
		// Blacklist it
		s.Blacklist(context.Background(), claims.ID, time.Now().Add(time.Hour))
		
		_, err := ValidateToken(context.Background(), tokenString)
		if err == nil || !strings.Contains(err.Error(), "token is revoked") {
			t.Errorf("expected revoked error, got %v", err)
		}
	})
}
