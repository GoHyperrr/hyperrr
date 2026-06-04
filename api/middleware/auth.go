package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/GoHyperrr/hyperrr/pkg/identity"
	"github.com/GoHyperrr/mdk"
)

// TokenValidator defines an interface for validating authentication tokens.
type TokenValidator interface {
	ValidateToken(ctx context.Context, token string) (*identity.Actor, error)
}

// ActorResolver defines the interface for resolving API Keys.
type ActorResolver interface {
	GetActorByAPIKey(ctx context.Context, key string) (*identity.Actor, error)
}

// AuthMiddleware extracts the credentials (JWT or API Key) and injects the Actor into the context.
func AuthMiddleware(providers []string, validator TokenValidator, resolver ActorResolver) func(http.Handler) http.Handler {
	if len(providers) == 0 {
		providers = []string{"jwt"} // default
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var actor *identity.Actor
			var authErr error
			authenticated := false
			authAttempted := false

			for _, p := range providers {
				switch p {
				case "none":
					actor = &identity.Actor{
						ID:   "act_http_developer",
						Type: identity.ActorAIAgent,
						Name: "Developer User (No Auth)",
					}
					authenticated = true
				case "jwt", "emailpass":
					header := r.Header.Get("Authorization")
					if header == "" {
						continue
					}
					authAttempted = true
					parts := strings.Split(header, " ")
					if len(parts) != 2 || parts[0] != "Bearer" {
						authErr = fmt.Errorf("Malformed Authorization header (expected 'Bearer <token>')")
						continue
					}
					if validator == nil {
						authErr = fmt.Errorf("JWT token validator not configured")
						continue
					}
					resActor, err := validator.ValidateToken(r.Context(), parts[1])
					if err != nil {
						authErr = err
						continue
					}
					actor = resActor
					authenticated = true
				case "apikey":
					apiKey := r.Header.Get("X-API-Key")
					if apiKey == "" {
						// Also try Authorization header Bearer <key>
						header := r.Header.Get("Authorization")
						if strings.HasPrefix(header, "Bearer ") {
							apiKey = strings.TrimPrefix(header, "Bearer ")
						}
					}
					if apiKey == "" {
						continue
					}
					authAttempted = true
					if resolver == nil {
						authErr = fmt.Errorf("API key resolver not configured")
						continue
					}
					resActor, err := resolver.GetActorByAPIKey(r.Context(), apiKey)
					if err != nil {
						authErr = err
						continue
					}
					actor = resActor
					authenticated = true
				}

				if authenticated {
					break
				}
			}

			// If no credentials were provided and we didn't attempt auth, allow the request to pass as anonymous.
			if !authenticated && !authAttempted {
				next.ServeHTTP(w, r)
				return
			}

			if !authenticated {
				errMsg := "Unauthorized"
				if authErr != nil {
					errMsg = "Unauthorized: " + authErr.Error()
				}
				http.Error(w, errMsg, http.StatusUnauthorized)
				return
			}

			ctx := mdk.WithActor(r.Context(), actor)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// WithActor returns a new context with the given actor.
func WithActor(ctx context.Context, actor *identity.Actor) context.Context {
	return mdk.WithActor(ctx, actor)
}

// ForContext retrieves the Actor from the context and a boolean indicating success.
func ForContext(ctx context.Context) (*identity.Actor, bool) {
	return mdk.ActorFromContext(ctx)
}

