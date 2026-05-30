package context

import (
	"context"
	"testing"
	"time"

	"github.com/GoHyperrr/hyperrr/pkg/config"
	"github.com/GoHyperrr/hyperrr/pkg/db"
	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
)

func TestContextModule(t *testing.T) {
	m := NewModule()

	// 1. ID
	if m.ID() != "core.context" {
		t.Errorf("expected module ID core.context, got %s", m.ID())
	}

	// 2. Models
	models := m.Models()
	if len(models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(models))
	}
	if _, ok := models[0].(*LineageModel); !ok {
		t.Error("expected LineageModel in Models()")
	}

	// 3. Handlers
	if m.Handlers() != nil {
		t.Error("expected Handlers() to return nil")
	}

	// 4. Shutdown
	if err := m.Shutdown(context.Background()); err != nil {
		t.Errorf("expected nil error on Shutdown, got %v", err)
	}

	// 5. Init and Projector
	bus := eventbus.NewInMemBus()
	cfg := &config.Config{DBDriver: "sqlite", DBDSN: ":memory:"}
	database, _ := db.Connect(cfg)
	_ = database.AutoMigrate(&LineageModel{})

	deps := &registry.Dependencies{
		EventBus: bus,
		DB:       database,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := m.Init(ctx, deps)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	proj := m.Projector()
	if proj == nil {
		t.Fatal("expected Projector() to not return nil after Init")
	}
}

func TestLineageGetters(t *testing.T) {
	now := time.Now()
	ended := now.Add(5 * time.Second)
	l := &Lineage{
		ID:        "lin-1",
		Name:      "flow-1",
		State:     "COMPLETED",
		Error:     "none",
		StartedAt: now,
		EndedAt:   &ended,
	}

	if l.GetID() != "lin-1" {
		t.Errorf("GetID mismatch: %s", l.GetID())
	}
	if l.GetName() != "flow-1" {
		t.Errorf("GetName mismatch: %s", l.GetName())
	}
	if l.GetState() != "COMPLETED" {
		t.Errorf("GetState mismatch: %s", l.GetState())
	}
	if l.GetError() != "none" {
		t.Errorf("GetError mismatch: %s", l.GetError())
	}
	if !l.GetStartedAt().Equal(now) {
		t.Error("GetStartedAt mismatch")
	}
	if l.GetEndedAt() == nil || !l.GetEndedAt().Equal(ended) {
		t.Error("GetEndedAt mismatch")
	}
}
