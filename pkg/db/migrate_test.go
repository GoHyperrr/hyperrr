package db

import (
	"fmt"
	"os"
	"testing"
	"time"

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
		
		dbFile := fmt.Sprintf("empty_migrate_%d.db", time.Now().UnixNano())
		defer os.Remove(dbFile)
		cfg := &config.Config{DBDriver: "sqlite", DBDSN: dbFile}
		database, _ := Connect(cfg)
		
		err := database.AutoMigrateAll()
		if err != nil {
			t.Errorf("expected no error with empty registry, got %v", err)
		}
	})

	t.Run("AutoMigrateAll Failure", func(t *testing.T) {
		dbFile := fmt.Sprintf("migrate_fail_%d.db", time.Now().UnixNano())
		defer os.Remove(dbFile)
		cfg := &config.Config{DBDriver: "sqlite", DBDSN: dbFile}
		database, _ := Connect(cfg)
		
		d, _ := database.DB.DB()
		d.Close() // Close underlying DB to force failure
		
		err := database.AutoMigrateAll()
		if err == nil {
			t.Error("expected error for closed DB")
		}
	})
}
