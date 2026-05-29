package db

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// IdempotencyKey prevents duplicate processing of the same operation.
type IdempotencyKey struct {
	ID        string    `gorm:"primaryKey"`
	Scope     string    `gorm:"index:idx_scope_key,unique"` // e.g. "workflow_step"
	Key       string    `gorm:"index:idx_scope_key,unique"` // e.g. "wfID_stepID"
	CreatedAt time.Time
}

// IsProcessed checks if an operation has already been completed.
func (db *DB) IsProcessed(ctx context.Context, scope, key string) (bool, error) {
	var ik IdempotencyKey
	err := db.WithContext(ctx).Where("scope = ? AND key = ?", scope, key).First(&ik).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// MarkProcessed records that an operation has been completed.
func (db *DB) MarkProcessed(ctx context.Context, scope, key string) error {
	ik := &IdempotencyKey{
		ID:        "ik_" + uuid.New().String(),
		Scope:     scope,
		Key:       key,
		CreatedAt: time.Now(),
	}
	return db.WithContext(ctx).Create(ik).Error
}
