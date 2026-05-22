package identity

import (
	"context"
	"os"
	"testing"

	"github.com/GoHyperrr/hyperrr/pkg/config"
	"github.com/GoHyperrr/hyperrr/pkg/db"
	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
)

func TestIdentityHandlers(t *testing.T) {
	dbFile := "identity_h_test.db"
	defer os.Remove(dbFile)

	cfg := &config.Config{DBDriver: "sqlite", DBDSN: dbFile}
	database, _ := db.Connect(cfg)
	bus := eventbus.NewInMemBus()

	mod := NewModule()
	mod.Init(context.Background(), &registry.Dependencies{DB: database, EventBus: bus})
	db.Register(mod.Models()...)
	database.AutoMigrateAll()

	t.Run("API Key Retrieval", func(t *testing.T) {
		actor := &Actor{ID: "act1", Type: ActorHuman, Name: "Tester"}
		database.Create(actor)
		database.Create(&APIKey{Key: "secret", ActorID: "act1"})

		got, err := mod.GetActorByAPIKey(context.Background(), "secret")
		if err != nil || got.ID != "act1" {
			t.Errorf("GetActorByAPIKey failed: %v", err)
		}

		_, err = mod.GetActorByAPIKey(context.Background(), "ghost")
		if err == nil {
			t.Error("expected error for invalid key")
		}
	})

	t.Run("Login Error Cases", func(t *testing.T) {
		_, err := mod.Login(context.Background(), "ghost@example.com", "password")
		if err == nil || err.Error() != "invalid credentials" {
			t.Error("expected invalid credentials error for non-existent user")
		}

		mod.Register(context.Background(), "tester@example.com", "pass", "Tester")
		_, err = mod.Login(context.Background(), "tester@example.com", "wrong")
		if err == nil || err.Error() != "invalid credentials" {
			t.Error("expected invalid credentials error for wrong password")
		}
	})
	
	t.Run("ValidateActor", func(t *testing.T) {
		// Test missing input
		_, err := mod.ValidateActor(context.Background(), "invalid")
		if err == nil {
			t.Error("expected error for invalid input type")
		}

		// Test missing workflow input
		_, err = mod.ValidateActor(context.Background(), map[string]any{"wrong": 1})
		if err == nil {
			t.Error("expected error for missing workflow input")
		}

		// Test missing actor_id
		_, err = mod.ValidateActor(context.Background(), map[string]any{"input": map[string]any{}})
		if err == nil {
			t.Error("expected error for missing actor_id")
		}

		// Test actor not found
		_, err = mod.ValidateActor(context.Background(), map[string]any{"input": map[string]any{"actor_id": "ghost"}})
		if err == nil {
			t.Error("expected error for ghost actor")
		}
	})

	t.Run("Register", func(t *testing.T) {
		actor, err := mod.Register(context.Background(), "unique@example.com", "pass", "New User")
		if err != nil {
			t.Fatalf("failed to register: %v", err)
		}
		if actor.Name != "New User" {
			t.Errorf("expected New User, got %s", actor.Name)
		}

		// Test duplicate email
		_, err = mod.Register(context.Background(), "unique@example.com", "pass", "Duplicate")
		if err == nil {
			t.Error("expected error for duplicate email")
		}
	})

	t.Run("Emit with nil bus", func(t *testing.T) {
		m := &Module{}
		m.emit(context.Background(), "test", nil) // Should not panic
	})
}
