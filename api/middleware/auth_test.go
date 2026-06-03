package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/GoHyperrr/hyperrr/pkg/identity"
)

type mockValidator struct {
	actor *identity.Actor
	err   error
}

func (m *mockValidator) ValidateToken(ctx context.Context, token string) (*identity.Actor, error) {
	return m.actor, m.err
}

func TestAuthMiddleware_Scenarios(t *testing.T) {
	t.Run("Valid Token", func(t *testing.T) {
		expectedActor := &identity.Actor{ID: "act_1", Type: identity.ActorHuman}
		validator := &mockValidator{actor: expectedActor}
		mw := AuthMiddleware(validator)

		h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			actor, ok := ForContext(r.Context())
			if !ok || actor.ID != "act_1" {
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
		mw := AuthMiddleware(&mockValidator{})
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
		mw := AuthMiddleware(&mockValidator{})
		h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("expected handler to abort, but it was executed")
		}))

		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "invalid-format")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rr.Code)
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
		actor := &identity.Actor{ID: "test"}
		ctx := WithActor(context.Background(), actor)
		got, ok := ForContext(ctx)
		if !ok || got.ID != "test" {
			t.Error("WithActor failed")
		}
	})
}
