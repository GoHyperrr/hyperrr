package eventbus

import (
	"context"
	"time"
)

// Event represents a system event.
type Event struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Payload   any       `json:"payload"`
	Timestamp time.Time `json:"timestamp"`
	Metadata  Metadata  `json:"metadata"`
}

// Metadata represents event metadata.
type Metadata map[string]string

// EventHandler is a function that handles an event.
type EventHandler func(ctx context.Context, event Event) error

// EventBus defines the interface for publishing and subscribing to events.
type EventBus interface {
	// Publish sends an event to the bus.
	Publish(ctx context.Context, event Event) error
	// Subscribe registers a handler for a specific event type.
	Subscribe(ctx context.Context, eventType string, handler EventHandler) error
	// Close shuts down the event bus.
	Close() error
}
