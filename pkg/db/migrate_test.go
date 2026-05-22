package db

import (
	"os"
	"testing"

	"github.com/GoHyperrr/hyperrr/pkg/config"
)

func TestAutoMigrateAllEdgeCases(t *testing.T) {
	t.Run("No registered models", func(t *testing.T) {
		// Save and clear Registry
		oldRegistry := Registry
		Registry = nil
		defer func() { Registry = oldRegistry }()
		
		dbFile := "empty_migrate_edge.db"
		defer os.Remove(dbFile)
		cfg := &config.Config{DBDriver: "sqlite", DBDSN: dbFile}
		database, _ := Connect(cfg)
		
		err := database.AutoMigrateAll()
		if err != nil {
			t.Errorf("expected no error with empty registry, got %v", err)
		}
	})

	t.Run("AutoMigrateAll Failure", func(t *testing.T) {
		cfg := &config.Config{DBDriver: "sqlite", DBDSN: "migrate_fail.db"}
		database, _ := Connect(cfg)
		d, _ := database.DB.DB()
		d.Close() // Close underlying DB to force failure
		
		err := database.AutoMigrateAll()
		if err == nil {
			t.Error("expected error for closed DB")
		}
		os.Remove("migrate_fail.db")
	})
}
