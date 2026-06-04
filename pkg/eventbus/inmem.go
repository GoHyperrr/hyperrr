package eventbus

import (
	"context"
	"sync"

	"github.com/GoHyperrr/hyperrr/pkg/logger"
	"github.com/google/uuid"
)

// InMemBus is a thread-safe in-memory implementation of EventBus.
type InMemBus struct {
	mu        sync.RWMutex
	handlers  map[string]map[string]EventHandler // topic (namespace.type) -> subID -> handler
	closeOnce sync.Once
	closed    chan struct{}
	async     bool
}

// NewInMemBus creates a new InMemBus.
func NewInMemBus() *InMemBus {
	return &InMemBus{
		handlers: make(map[string]map[string]EventHandler),
		closed:   make(chan struct{}),
	}
}

// SetAsync enables or disables asynchronous handler execution.
func (b *InMemBus) SetAsync(async bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.async = async
}

func topicKey(namespace, eventType string) string {
	if namespace == "" {
		return eventType
	}
	return namespace + "." + eventType
}

// Publish sends an event to all registered handlers for the event type.
func (b *InMemBus) Publish(ctx context.Context, event Event) error {
	select {
	case <-b.closed:
		return context.Canceled
	default:
	}

	key := topicKey(event.Namespace, event.Type)
	b.mu.RLock()
	typeHandlers, ok := b.handlers[key]
	if !ok {
		b.mu.RUnlock()
		return nil
	}
	// Copy handlers to slice under RLock to prevent concurrent map read/write race conditions
	handlers := make([]EventHandler, 0, len(typeHandlers))
	for _, h := range typeHandlers {
		handlers = append(handlers, h)
	}
	async := b.async
	b.mu.RUnlock()

	for _, handler := range handlers {
		if async {
			go func(h EventHandler) {
				detachedCtx := context.WithoutCancel(ctx)
				if err := h(detachedCtx, event); err != nil {
					logger.Error("event handler failed", "topic", key, "id", event.ID, "error", err)
				}
			}(handler)
		} else {
			// Execute synchronously for test stability and deterministic ordering.
			if err := handler(ctx, event); err != nil {
				logger.Error("event handler failed", "topic", key, "id", event.ID, "error", err)
			}
		}
	}

	return nil
}

// Subscribe registers a handler for a specific event type.
func (b *InMemBus) Subscribe(namespace, eventType string, handler EventHandler) (func(), error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	key := topicKey(namespace, eventType)
	if b.handlers[key] == nil {
		b.handlers[key] = make(map[string]EventHandler)
	}

	id := uuid.New().String()
	b.handlers[key][id] = handler

	unsubscribe := func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		if subMap, ok := b.handlers[key]; ok {
			delete(subMap, id)
			if len(subMap) == 0 {
				delete(b.handlers, key)
			}
		}
	}

	return unsubscribe, nil
}

// Close closes the event bus.
func (b *InMemBus) Close() error {
	b.closeOnce.Do(func() {
		close(b.closed)
	})
	return nil
}
