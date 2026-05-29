package analytics

import (
	"context"
	"testing"

	"github.com/GoHyperrr/hyperrr/internal/workflow"
	"github.com/GoHyperrr/hyperrr/pkg/config"
	"github.com/GoHyperrr/hyperrr/pkg/db"
	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
	ctxEngine "github.com/GoHyperrr/hyperrr/internal/context"
)

func TestAnalyticsModule(t *testing.T) {
	cfg := &config.Config{DBDriver: "sqlite", DBDSN: ":memory:"}
	database, _ := db.Connect(cfg)
	bus := eventbus.NewInMemBus()
	runner := workflow.NewRunner(bus)
	projector := ctxEngine.NewProjector(bus)
	projector.Start(context.Background())

	mod := NewModule()
	mod.Init(context.Background(), &registry.Dependencies{DB: database, EventBus: bus, Runner: runner})
	mod.SetProjector(projector)
	database.AutoMigrateAll()

	t.Run("System Stats", func(t *testing.T) {
		// Emit some events to seed lineages
		bus.Publish(context.Background(), eventbus.Event{
			Type: "workflow.started",
			Payload: map[string]any{"id": "wf1", "name": "test", "version": "v1"},
		})
		
		stats := mod.Projector().ListLineages()
		if len(stats) == 0 {
			t.Error("expected at least 1 lineage")
		}
	})

	t.Run("Module API", func(t *testing.T) {
		if mod.ID() != "commerce.analytics" {
			t.Error("invalid ID")
		}
		if mod.Repo() != nil {
			t.Error("repo should be nil")
		}
		if len(mod.Models()) != 0 {
			t.Error("models should be empty")
		}
		if len(mod.Handlers()) != 0 {
			t.Error("handlers should be empty")
		}
	})
}
