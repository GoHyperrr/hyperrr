package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/GoHyperrr/hyperrr/pkg/identity"
)

type mockValidator struct {
	actor identity.Actor
	err   error
}

func (m *mockValidator) ValidateToken(ctx context.Context, token string) (identity.Actor, error) {
	return m.actor, m.err
}

type mockResolver struct {
	actor identity.Actor
	err   error
}

func (m *mockResolver) GetActorByAPIKey(ctx context.Context, key string) (identity.Actor, error) {
	return m.actor, m.err
}

func TestAuthMiddleware_Scenarios(t *testing.T) {
	t.Run("Valid JWT Token", func(t *testing.T) {
		expectedActor := &identity.BaseActor{ID: "act_1", Type: identity.ActorHuman}
		validator := &mockValidator{actor: expectedActor}
		mw := AuthMiddleware([]string{"jwt"}, validator, nil)

		h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			actor, ok := ForContext(r.Context())
			if !ok || actor.GetID() != "act_1" {
				t.Errorf("expected actor act_1, got %v", actor)
			}
		}))

		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Bearer valid-token")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
	})

	t.Run("No Token", func(t *testing.T) {
		mw := AuthMiddleware([]string{"jwt"}, &mockValidator{}, nil)
		h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, ok := ForContext(r.Context())
			if ok {
				t.Error("expected no actor in context")
			}
		}))

		req := httptest.NewRequest("GET", "/", nil)
		h.ServeHTTP(httptest.NewRecorder(), req)
	})

	t.Run("Malformed Token", func(t *testing.T) {
		mw := AuthMiddleware([]string{"jwt"}, &mockValidator{}, nil)
		h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("expected handler to abort, but it was executed")
		}))

		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "invalid-format")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rr.Code)
		}
	})

	t.Run("Valid API Key", func(t *testing.T) {
		expectedActor := &identity.BaseActor{ID: "agent_1", Type: identity.ActorAIAgent}
		resolver := &mockResolver{actor: expectedActor}
		mw := AuthMiddleware([]string{"apikey"}, nil, resolver)

		h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			actor, ok := ForContext(r.Context())
			if !ok || actor.GetID() != "agent_1" {
				t.Errorf("expected actor agent_1, got %v", actor)
			}
		}))

		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("X-API-Key", "valid-api-key")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
	})

	t.Run("Bypass 'none' provider", func(t *testing.T) {
		mw := AuthMiddleware([]string{"none"}, nil, nil)

		h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			actor, ok := ForContext(r.Context())
			if !ok || actor.GetID() != "act_http_developer" {
				t.Errorf("expected actor act_http_developer, got %v", actor)
			}
		}))

		req := httptest.NewRequest("GET", "/", nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
	})

	t.Run("No Actor in context", func(t *testing.T) {
		ctx := context.Background()
		_, ok := ForContext(ctx)
		if ok {
			t.Error("expected no actor")
		}
	})

	t.Run("WithActor helper", func(t *testing.T) {
		actor := &identity.BaseActor{ID: "test"}
		ctx := WithActor(context.Background(), actor)
		got, ok := ForContext(ctx)
		if !ok || got.GetID() != "test" {
			t.Error("WithActor failed")
		}
	})
}
