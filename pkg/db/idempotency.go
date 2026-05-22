package db

import (
	"context"
	"fmt"
	"time"
)

// IdempotencyKey prevents duplicate processing of the same operation.
type IdempotencyKey struct {
	ID        string    `gorm:"primaryKey"`
	Scope     string    `gorm:"index:idx_scope_key,unique"` // e.g. "workflow_step"
	Key       string    `gorm:"index:idx_scope_key,unique"` // e.g. "wfID_stepID"
	CreatedAt time.Time
}

// IsProcessed checks if an operation has already been completed.
func (db *DB) IsProcessed(ctx context.Context, scope, key string) bool {
	var ik IdempotencyKey
	err := db.WithContext(ctx).Where("scope = ? AND key = ?", scope, key).First(&ik).Error
	return err == nil
}

// MarkProcessed records that an operation has been completed.
func (db *DB) MarkProcessed(ctx context.Context, scope, key string) error {
	ik := &IdempotencyKey{
		ID:        fmt.Sprintf("ik_%d", time.Now().UnixNano()),
		Scope:     scope,
		Key:       key,
		CreatedAt: time.Now(),
	}
	return db.WithContext(ctx).Create(ik).Error
}
