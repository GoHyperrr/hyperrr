package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/GoHyperrr/hyperrr/internal/auth"
	"github.com/GoHyperrr/hyperrr/internal/identity"
)

func TestAuthMiddleware(t *testing.T) {
	t.Run("No Authorization Header", func(t *testing.T) {
		mw := AuthMiddleware()
		handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			actor, ok := ForContext(r.Context())
			if ok || actor != nil {
				t.Error("expected nil actor")
			}
		}))

		req := httptest.NewRequest("GET", "/", nil)
		mw(handler).ServeHTTP(httptest.NewRecorder(), req)
	})

	t.Run("Valid Token", func(t *testing.T) {
		actor := identity.Actor{ID: "act_1", Type: identity.ActorHuman}
		token, _ := auth.GenerateToken(actor)

		mw := AuthMiddleware()
		handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got, ok := ForContext(r.Context())
			if !ok || got == nil || got.ID != actor.ID {
				t.Errorf("expected actor %s, got %v", actor.ID, got)
			}
		}))

		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		mw(handler).ServeHTTP(httptest.NewRecorder(), req)
	})

	t.Run("Invalid Token Format", func(t *testing.T) {
		mw := AuthMiddleware()
		handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, ok := ForContext(r.Context())
			if ok {
				t.Error("expected ok false")
			}
		}))

		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Basic 123")
		mw(handler).ServeHTTP(httptest.NewRecorder(), req)
	})

	t.Run("Invalid JWT", func(t *testing.T) {
		mw := AuthMiddleware()
		handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, ok := ForContext(r.Context())
			if ok {
				t.Error("expected ok false")
			}
		}))

		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Bearer invalid-token")
		mw(handler).ServeHTTP(httptest.NewRecorder(), req)
	})
	
	t.Run("WithActor helper", func(t *testing.T) {
		actor := &identity.Actor{ID: "test"}
		ctx := WithActor(context.Background(), actor)
		got, ok := ForContext(ctx)
		if !ok || got != actor {
			t.Error("WithActor failed")
		}
	})
}
