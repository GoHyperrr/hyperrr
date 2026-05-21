package auth

import (
	"testing"

	"github.com/GoHyperrr/hyperrr/internal/identity"
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
}
