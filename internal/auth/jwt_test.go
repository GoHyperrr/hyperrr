package auth

import (
	"strings"
	"testing"

	"github.com/GoHyperrr/hyperrr/internal/identity"
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

		got, err := ValidateToken(token)
		if err != nil {
			t.Fatalf("failed to validate token: %v", err)
		}

		if got.ID != actor.ID || got.Type != actor.Type {
			t.Errorf("expected %v, got %v", actor, got)
		}
	})

	t.Run("Invalid Token", func(t *testing.T) {
		_, err := ValidateToken("invalid.token.string")
		if err == nil {
			t.Error("expected error for invalid token, got nil")
		}
	})

	t.Run("Unexpected Signing Method", func(t *testing.T) {
		// Create a token with a different signing method (e.g., None)
		token := jwt.NewWithClaims(jwt.SigningMethodNone, &Claims{ActorID: "test"})
		tokenString, _ := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
		
		_, err := ValidateToken(tokenString)
		if err == nil || !strings.Contains(err.Error(), "unexpected signing method") {
			t.Errorf("expected error for unexpected signing method, got %v", err)
		}
	})
}
