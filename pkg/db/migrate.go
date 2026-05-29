package db

import (
	"fmt"
	"sync"
)

var (
	registryMu sync.Mutex
	Registry   = []interface{}{&OutboxEvent{}, &IdempotencyKey{}}
)

// Register adds models to the global migration registry.
func Register(models ...interface{}) {
	registryMu.Lock()
	defer registryMu.Unlock()
	Registry = append(Registry, models...)
}

// AutoMigrate runs the GORM AutoMigrate for all registered models.
func (db *DB) AutoMigrateAll() error {
	registryMu.Lock()
	models := make([]interface{}, len(Registry))
	copy(models, Registry)
	registryMu.Unlock()

	if len(models) == 0 {
		return nil
	}

	if err := db.AutoMigrate(models...); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}
