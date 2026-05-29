package eventbus

import (
	"context"
	"sync"

	"github.com/GoHyperrr/hyperrr/pkg/logger"
)

// InMemBus is a thread-safe in-memory implementation of EventBus.
type InMemBus struct {
	mu          sync.RWMutex
	handlers    map[string][]EventHandler
	closeOnce   sync.Once
	closed      chan struct{}
}

// NewInMemBus creates a new InMemBus.
func NewInMemBus() *InMemBus {
	return &InMemBus{
		handlers: make(map[string][]EventHandler),
		closed:   make(chan struct{}),
	}
}

// Publish sends an event to all registered handlers for the event type.
func (b *InMemBus) Publish(ctx context.Context, event Event) error {
	select {
	case <-b.closed:
		return context.Canceled
	default:
	}

	b.mu.RLock()
	handlers, ok := b.handlers[event.Type]
	b.mu.RUnlock()

	if !ok {
		return nil
	}

	for _, handler := range handlers {
		// Execute synchronously for test stability and deterministic ordering.
		if err := handler(ctx, event); err != nil {
			logger.Error("event handler failed", "type", event.Type, "id", event.ID, "error", err)
		}
	}

	return nil
}

// Subscribe registers a handler for a specific event type.
func (b *InMemBus) Subscribe(ctx context.Context, eventType string, handler EventHandler) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.handlers[eventType] = append(b.handlers[eventType], handler)
	return nil
}

// Close shuts down the event bus.
func (b *InMemBus) Close() error {
	b.closeOnce.Do(func() {
		close(b.closed)
	})
	return nil
}
