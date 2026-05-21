package tests

import (
	"context"
	"os"
	"testing"

	"github.com/GoHyperrr/hyperrr/api/graph"
	"github.com/GoHyperrr/hyperrr/api/middleware"
	"github.com/GoHyperrr/hyperrr/commerce/customer"
	"github.com/GoHyperrr/hyperrr/internal/identity"
	"github.com/GoHyperrr/hyperrr/pkg/config"
	"github.com/GoHyperrr/hyperrr/pkg/db"
	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
)

func TestAuthFlow(t *testing.T) {
	ctx := context.Background()
	bus := eventbus.NewInMemBus()
	
	// Setup DB
	cfg := &config.Config{DBDriver: "sqlite", DBDSN: "auth_test.db"}
	database, _ := db.Connect(cfg)
	defer func() {
		d, _ := database.DB.DB()
		d.Close()
		os.Remove("auth_test.db")
	}()

	// Setup Identity
	identMod := identity.NewModule()
	identMod.Init(ctx, &registry.Dependencies{DB: database, EventBus: bus})
	db.Register(identMod.Models()...)

	// Setup Customer
	custMod := customer.NewModule()
	custMod.Init(ctx, &registry.Dependencies{DB: database, EventBus: bus})
	db.Register(custMod.Models()...)

	database.AutoMigrateAll()

	resolver := &graph.Resolver{
		IdentityModule: identMod,
		CustomerModule: custMod,
	}

	t.Run("Register and Login", func(t *testing.T) {
		// 1. Register
		regRes, err := resolver.Mutation().Register(ctx, "test@example.com", "password123", "Test User")
		if err != nil {
			t.Fatalf("registration failed: %v", err)
		}
		if regRes.Token == "" {
			t.Fatal("expected token, got empty")
		}

		// 2. Login
		loginRes, err := resolver.Mutation().Login(ctx, "test@example.com", "password123")
		if err != nil {
			t.Fatalf("login failed: %v", err)
		}
		if loginRes.Token == "" {
			t.Fatal("expected token, got empty")
		}

		// 3. Verify Customer created via event
		c, err := custMod.Repo().GetByUserID(ctx, loginRes.Actor.ID)
		if err != nil {
			t.Fatalf("customer not found: %v", err)
		}
		if c.Email != "test@example.com" {
			t.Errorf("expected test@example.com, got %s", c.Email)
		}

		// 4. Test 'me' query
		actor := &identity.Actor{ID: loginRes.Actor.ID, Type: identity.ActorType(loginRes.Actor.Type), Name: loginRes.Actor.Name}
		meCtx := middleware.WithActor(ctx, actor)
		meRes, err := resolver.Query().Me(meCtx)
		if err != nil {
			t.Fatalf("me query failed: %v", err)
		}
		if meRes.ID != loginRes.Actor.ID {
			t.Errorf("expected ID %s, got %s", loginRes.Actor.ID, meRes.ID)
		}
	})

	t.Run("Login Failure", func(t *testing.T) {
		_, err := resolver.Mutation().Login(ctx, "test@example.com", "wrong-password")
		if err == nil {
			t.Fatal("expected login failure for wrong password")
		}
	})

	t.Run("Me Unauthorized", func(t *testing.T) {
		_, err := resolver.Query().Me(ctx)
		if err == nil {
			t.Fatal("expected error for unauthorized me query")
		}
	})
}
