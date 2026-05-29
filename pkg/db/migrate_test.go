package db

import (
	"path/filepath"
	"testing"

	"github.com/GoHyperrr/hyperrr/pkg/config"
)

func TestAutoMigrateAllEdgeCases(t *testing.T) {
	t.Run("No registered models", func(t *testing.T) {
		// Save and clear Registry using the lock if possible, or just direct for test
		registryMu.Lock()
		oldRegistry := Registry
		Registry = nil
		registryMu.Unlock()
		
		defer func() {
			registryMu.Lock()
			Registry = oldRegistry
			registryMu.Unlock()
		}()
		
		dbFile := filepath.Join(t.TempDir(), "empty_migrate.db")
		cfg := &config.Config{DBDriver: "sqlite", DBDSN: dbFile}
		database, _ := Connect(cfg)
		sqlDB, _ := database.DB.DB()
		defer sqlDB.Close()
		
		err := database.AutoMigrateAll()
		if err != nil {
			t.Errorf("expected no error with empty registry, got %v", err)
		}
	})

	t.Run("AutoMigrateAll Failure", func(t *testing.T) {
		dbFile := filepath.Join(t.TempDir(), "migrate_fail.db")
		cfg := &config.Config{DBDriver: "sqlite", DBDSN: dbFile}
		database, _ := Connect(cfg)
		sqlDB, _ := database.DB.DB()
		defer sqlDB.Close()
		
		sqlDB.Close() // Close underlying DB to force failure
		
		err := database.AutoMigrateAll()
		if err == nil {
			t.Error("expected error for closed DB")
		}
	})
}
