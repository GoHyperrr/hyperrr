package tests

import (
	"context"
	"testing"

	"github.com/GoHyperrr/hyperrr/api/graph"
	"github.com/GoHyperrr/hyperrr/api/middleware"
	"github.com/GoHyperrr/commerce/customer"
	"github.com/GoHyperrr/auth/emailpass"
	"github.com/GoHyperrr/auth/apikey"
	ident "github.com/GoHyperrr/hyperrr/pkg/identity"
	"github.com/GoHyperrr/hyperrr/pkg/workflow"
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

	// Setup EmailPass Auth
	emailpassMod := emailpass.NewModule()
	emailpassMod.Init(ctx, &registry.Dependencies{
		Config:   &config.Config{JWTSecret: "secret", JWTExpiration: "24h"},
		DB:       database,
		EventBus: bus,
	})
	db.Register(emailpassMod.Models()...)

	// Setup Customer
	custMod := customer.NewModule()
	custMod.Init(ctx, &registry.Dependencies{
		DB:       database,
		EventBus: bus,
		Runner:   workflow.NewRunner(bus, nil, nil),
		Registry: workflow.NewRegistry(),
	})
	db.Register(custMod.Models()...)

	// Setup APIKey Auth
	apikeyMod := apikey.NewModule()
	apikeyMod.Init(ctx, &registry.Dependencies{
		DB:       database,
		EventBus: bus,
	})
	db.Register(apikeyMod.Models()...)

	database.AutoMigrateAll()

	resolver := &graph.Resolver{
		EmailpassModule: emailpassMod,
		CustomerModule:  custMod,
		ApikeyModule:    apikeyMod,
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
		actor := &ident.Actor{ID: loginRes.Actor.ID, Type: ident.ActorType(loginRes.Actor.Type), Name: loginRes.Actor.Name}
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

	t.Run("API Key CRUD Operations", func(t *testing.T) {
		actor := &ident.Actor{ID: "act_test_key_owner", Type: ident.ActorHuman, Name: "Key Owner"}
		if err := database.Create(actor).Error; err != nil {
			t.Fatalf("failed to seed test actor: %v", err)
		}
		authCtx := middleware.WithActor(ctx, actor)

		// 1. Unauthorized create
		_, err := resolver.Mutation().CreateAPIKey(ctx, "Unauthorized Key", nil)
		if err == nil {
			t.Fatal("expected error creating API key without actor context")
		}

		// 2. Authorized create
		keyInfo, err := resolver.Mutation().CreateAPIKey(authCtx, "My Dev Key", nil)
		if err != nil {
			t.Fatalf("failed to create API key: %v", err)
		}
		if keyInfo.Name != "My Dev Key" {
			t.Errorf("expected key name 'My Dev Key', got '%s'", keyInfo.Name)
		}
		if len(keyInfo.Key) < 5 || keyInfo.Key[:3] != "hk_" {
			t.Errorf("expected secure key prefix 'hk_', got '%s'", keyInfo.Key)
		}

		// 3. Authorized list
		keyList, err := resolver.Query().ListAPIKeys(authCtx)
		if err != nil {
			t.Fatalf("failed to list API keys: %v", err)
		}
		if len(keyList) != 1 {
			t.Errorf("expected 1 API key, got %d", len(keyList))
		}
		if keyList[0].ID != keyInfo.ID || keyList[0].Name != "My Dev Key" {
			t.Errorf("listed key does not match created key")
		}

		// 4. Resolve actor by key
		resolvedActor, err := apikeyMod.GetActorByAPIKey(ctx, keyInfo.Key)
		if err != nil {
			t.Fatalf("failed to resolve actor by key: %v", err)
		}
		if resolvedActor.ID != actor.ID {
			t.Errorf("expected resolved actor ID %s, got %s", actor.ID, resolvedActor.ID)
		}

		// 5. Authorized revoke
		revoked, err := resolver.Mutation().RevokeAPIKey(authCtx, keyInfo.ID)
		if err != nil {
			t.Fatalf("failed to revoke API key: %v", err)
		}
		if !revoked {
			t.Fatal("expected revoke to return true")
		}

		// 6. List after revoke
		keyList2, err := resolver.Query().ListAPIKeys(authCtx)
		if err != nil {
			t.Fatalf("failed to list API keys post-revoke: %v", err)
		}
		if len(keyList2) != 0 {
			t.Errorf("expected 0 API keys after revoke, got %d", len(keyList2))
		}

		// 7. Resolve revoked key
		_, err = apikeyMod.GetActorByAPIKey(ctx, keyInfo.Key)
		if err == nil {
			t.Fatal("expected resolution to fail for a revoked key")
		}
	})
}
