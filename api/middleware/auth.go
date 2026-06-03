package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/GoHyperrr/hyperrr/pkg/identity"
)

type contextKey string

const actorCtxKey contextKey = "actor"

// TokenValidator defines an interface for validating authentication tokens.
type TokenValidator interface {
	ValidateToken(ctx context.Context, token string) (*identity.Actor, error)
}

// AuthMiddleware extracts the JWT from the Authorization header and injects the Actor into the context.
func AuthMiddleware(validator TokenValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header == "" {
				next.ServeHTTP(w, r)
				return
			}

			parts := strings.Split(header, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				http.Error(w, "Malformed Authorization header (expected 'Bearer <token>')", http.StatusBadRequest)
				return
			}

			token := parts[1]
			actor, err := validator.ValidateToken(r.Context(), token)
			if err != nil {
				http.Error(w, "Invalid token", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), actorCtxKey, actor)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// WithActor returns a new context with the given actor.
func WithActor(ctx context.Context, actor *identity.Actor) context.Context {
	return context.WithValue(ctx, actorCtxKey, actor)
}

// ForContext retrieves the Actor from the context and a boolean indicating success.
func ForContext(ctx context.Context) (*identity.Actor, bool) {
	raw, ok := ctx.Value(actorCtxKey).(*identity.Actor)
	return raw, ok
}
