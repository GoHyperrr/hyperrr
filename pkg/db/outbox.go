package db

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// OutboxEvent represents an event that needs to be published after a transaction commits.
type OutboxEvent struct {
	ID        string    `gorm:"primaryKey"`
	Type      string    `gorm:"not null"`
	Payload   string    `gorm:"type:text;not null"`
	CreatedAt time.Time `gorm:"index"`
	Processed bool      `gorm:"default:false;index"`
}

// SaveToOutbox saves an event to the outbox table within the given transaction.
func (db *DB) SaveToOutbox(ctx context.Context, eventType string, payload any) error {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal outbox payload: %w", err)
	}

	outboxEvent := &OutboxEvent{
		ID:        "out_" + uuid.New().String(),
		Type:      eventType,
		Payload:   string(payloadJSON),
		CreatedAt: time.Now(),
	}

	return db.WithContext(ctx).Create(outboxEvent).Error
}
