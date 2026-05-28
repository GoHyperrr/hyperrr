package auth

import (
	"context"
	"os"
	"testing"

	"github.com/GoHyperrr/hyperrr/pkg/config"
	"github.com/GoHyperrr/hyperrr/pkg/db"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
)

func TestModule(t *testing.T) {
	dbFile := "auth_module_test.db"
	defer os.Remove(dbFile)

	m := NewModule()
	if m.ID() != "internal.auth" {
		t.Errorf("expected internal.auth, got %s", m.ID())
	}

	if len(m.Models()) != 2 {
		t.Errorf("expected 2 models, got %d", len(m.Models()))
	}

	if m.Handlers() != nil {
		t.Error("expected nil handlers")
	}

	cfg := &config.Config{DBDriver: "sqlite", DBDSN: dbFile}
	database, _ := db.Connect(cfg)
	
	deps := &registry.Dependencies{
		DB: database,
	}

	err := m.Init(context.Background(), deps)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	if m.store == nil {
		t.Error("store was not initialized")
	}
}
