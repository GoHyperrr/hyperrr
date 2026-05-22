package db

import (
	"fmt"
)

// Registry maintains a list of all models to be migrated.
var Registry = []interface{}{&OutboxEvent{}, &IdempotencyKey{}}

// Register adds models to the global migration registry.
func Register(models ...interface{}) {
	Registry = append(Registry, models...)
}

// AutoMigrate runs the GORM AutoMigrate for all registered models.
func (db *DB) AutoMigrateAll() error {
	if len(Registry) == 0 {
		return nil
	}

	if err := db.AutoMigrate(Registry...); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}
