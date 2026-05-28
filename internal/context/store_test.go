package context

import (
	"context"
	"os"
	"testing"

	"github.com/GoHyperrr/hyperrr/pkg/config"
	"github.com/GoHyperrr/hyperrr/pkg/db"
)

func TestLineageStore(t *testing.T) {
	dbFile := "context_store_test.db"
	defer os.Remove(dbFile)

	cfg := &config.Config{DBDriver: "sqlite", DBDSN: dbFile}
	database, _ := db.Connect(cfg)
	database.AutoMigrate(&LineageModel{})

	store := NewLineageStore(database)
	ctx := context.Background()

	t.Run("SaveAndGet", func(t *testing.T) {
		l := &Lineage{ID: "w1", Name: "test"}
		store.Save(ctx, l)
		got, _ := store.Get(ctx, "w1")
		if got.ID != "w1" { t.Error("Save/Get failed") }
		store.List(ctx)
	})
}
