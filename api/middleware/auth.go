package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/GoHyperrr/hyperrr/internal/auth"
	"github.com/GoHyperrr/hyperrr/internal/identity"
)

type contextKey string

const actorCtxKey contextKey = "actor"

// AuthMiddleware extracts the JWT from the Authorization header and injects the Actor into the context.
func AuthMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header == "" {
				next.ServeHTTP(w, r)
				return
			}

			parts := strings.Split(header, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				next.ServeHTTP(w, r)
				return
			}

			token := parts[1]
			actor, err := auth.ValidateToken(token)
			if err != nil {
				// We don't necessarily want to fail the request here, 
				// resolvers will check if actor is required.
				next.ServeHTTP(w, r)
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

// ForContext retrieves the Actor from the context.
func ForContext(ctx context.Context) *identity.Actor {
	raw, _ := ctx.Value(actorCtxKey).(*identity.Actor)
	return raw
}
