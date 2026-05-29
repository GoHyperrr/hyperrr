package tests

import (
	"context"
	"testing"

	"github.com/GoHyperrr/hyperrr/api/graph"
	"github.com/GoHyperrr/hyperrr/api/middleware"
	"github.com/GoHyperrr/hyperrr/commerce/customer"
	"github.com/GoHyperrr/hyperrr/internal/auth"
	"github.com/GoHyperrr/hyperrr/internal/identity"
	"github.com/GoHyperrr/hyperrr/internal/workflow"
	"github.com/GoHyperrr/hyperrr/pkg/config"
	"github.com/GoHyperrr/hyperrr/pkg/db"
	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
)

func TestAuthFlow(t *testing.T) {
	ctx := context.Background()
	bus := eventbus.NewInMemBus()
	
	// Setup DB
	cfg := &config.Config{DBDriver: "sqlite", DBDSN: ":memory:"}
	database, _ := db.Connect(cfg)
	defer func() {
		d, _ := database.DB.DB()
		d.Close()
	}()

	// Setup Identity
	identMod := identity.NewModule()
	identMod.Init(ctx, &registry.Dependencies{DB: database, EventBus: bus})
	db.Register(identMod.Models()...)

	// Setup Customer
	custMod := customer.NewModule()
	custMod.Init(ctx, &registry.Dependencies{
		DB:       database,
		EventBus: bus,
		Runner:   workflow.NewRunner(bus),
		Registry: workflow.NewRegistry(),
	})
	db.Register(custMod.Models()...)

	// Setup Auth
	authMod := auth.NewModule()
	authMod.Init(ctx, &registry.Dependencies{
		Config: &config.Config{JWTSecret: "secret", JWTExpiration: "24h"},
		DB:     database,
	})
	db.Register(authMod.Models()...)

	database.AutoMigrateAll()

	resolver := &graph.Resolver{
		IdentityModule: identMod,
		CustomerModule: custMod,
		AuthModule:     authMod,
	}

	t.Run("Register and Login", func(t *testing.T) {
		// 1. Register
		regRes, err := resolver.Mutation().Register(ctx, "test_auth@example.com", "password123", "Test User")
		if err != nil {
			t.Fatalf("registration failed: %v", err)
		}
		if regRes.Token == "" {
			t.Fatal("expected token, got empty")
		}

		// 2. Login
		loginRes, err := resolver.Mutation().Login(ctx, "test_auth@example.com", "password123")
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
		if c.Email != "test_auth@example.com" {
			t.Errorf("expected test_auth@example.com, got %s", c.Email)
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
		_, err := resolver.Mutation().Login(ctx, "test_auth@example.com", "wrong-password")
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
