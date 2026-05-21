package identity

import (
	"context"
	"os"
	"testing"

	"github.com/GoHyperrr/hyperrr/pkg/config"
	"github.com/GoHyperrr/hyperrr/pkg/db"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
)

func TestIdentityModule(t *testing.T) {
	dbFile := "identity_test.db"
	defer os.Remove(dbFile)

	cfg := &config.Config{
		DBDriver: "sqlite",
		DBDSN:    dbFile,
	}

	database, err := db.Connect(cfg)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}

	mod := NewModule()
	deps := &registry.Dependencies{
		DB: database,
	}

	if err := mod.Init(context.Background(), deps); err != nil {
		t.Fatalf("failed to init: %v", err)
	}

	// Register and migrate
	db.Register(mod.Models()...)
	if err := database.AutoMigrateAll(); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	t.Run("Validate Actor Success", func(t *testing.T) {
		actor := Actor{ID: "actor_1", Type: ActorHuman, Name: "Test User"}
		database.Create(&actor)

		input := map[string]any{
			"input": map[string]any{
				"actor_id": "actor_1",
			},
		}

		res, err := mod.ValidateActor(context.Background(), input)
		if err != nil {
			t.Fatalf("failed to validate actor: %v", err)
		}

		resMap := res.(map[string]any)
		if resMap["id"] != "actor_1" {
			t.Errorf("expected actor_1, got %v", resMap["id"])
		}
	})

	t.Run("Get Actor by API Key", func(t *testing.T) {
		actor := Actor{ID: "actor_2", Type: ActorAIAgent, Name: "AI Bot"}
		database.Create(&actor)
		database.Create(&APIKey{ID: "key_1", Key: "secret", ActorID: "actor_2"})

		got, err := mod.GetActorByAPIKey(context.Background(), "secret")
		if err != nil {
			t.Fatalf("failed to get actor: %v", err)
		}

		if got.ID != "actor_2" {
			t.Errorf("expected actor_2, got %s", got.ID)
		}
	})

	t.Run("Actor Not Found", func(t *testing.T) {
		input := map[string]any{"input": map[string]any{"actor_id": "ghost"}}
		_, err := mod.ValidateActor(context.Background(), input)
		if err == nil {
			t.Error("expected error for non-existent actor")
		}
	})

	t.Run("Invalid Inputs", func(t *testing.T) {
		_, err := mod.ValidateActor(context.Background(), "not a map")
		if err == nil {
			t.Error("expected error for invalid input type")
		}

		_, err = mod.ValidateActor(context.Background(), map[string]any{"wrong": "key"})
		if err == nil {
			t.Error("expected error for missing 'input' key")
		}
		
		_, err = mod.ValidateActor(context.Background(), map[string]any{"input": map[string]any{"wrong": "key"}})
		if err == nil {
			t.Error("expected error for missing actor_id")
		}
	})
	
	t.Run("Handlers Map", func(t *testing.T) {
		h := mod.Handlers()
		if _, ok := h["identity.validate_actor"]; !ok {
			t.Error("missing validate_actor handler")
		}
	})
}
