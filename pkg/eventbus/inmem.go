package eventbus

import (
	"context"
	"sync"

	"github.com/GoHyperrr/hyperrr/pkg/logger"
	"github.com/google/uuid"
)

// InMemSubscription implements the Subscription interface for InMemBus.
type InMemSubscription struct {
	bus       *InMemBus
	eventType string
	id        string
}

func (s *InMemSubscription) Unsubscribe() error {
	return s.bus.unsubscribe(s.eventType, s.id)
}

// InMemBus is a thread-safe in-memory implementation of EventBus.
type InMemBus struct {
	mu        sync.RWMutex
	handlers  map[string]map[string]EventHandler // type -> subID -> handler
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

// Publish sends an event to all registered handlers for the event type.
func (b *InMemBus) Publish(ctx context.Context, event Event) error {
	select {
	case <-b.closed:
		return context.Canceled
	default:
	}

	b.mu.RLock()
	typeHandlers, ok := b.handlers[event.Type]
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
					logger.Error("event handler failed", "type", event.Type, "id", event.ID, "error", err)
				}
			}(handler)
		} else {
			// Execute synchronously for test stability and deterministic ordering.
			if err := handler(ctx, event); err != nil {
				logger.Error("event handler failed", "type", event.Type, "id", event.ID, "error", err)
			}
		}
	}

	return nil
}

// Subscribe registers a handler for a specific event type.
func (b *InMemBus) Subscribe(ctx context.Context, eventType string, handler EventHandler) (Subscription, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.handlers[eventType] == nil {
		b.handlers[eventType] = make(map[string]EventHandler)
	}

	id := uuid.New().String()
	b.handlers[eventType][id] = handler

	return &InMemSubscription{
		bus:       b,
		eventType: eventType,
		id:        id,
	}, nil
}

func (b *InMemBus) unsubscribe(eventType string, id string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if subs, ok := b.handlers[eventType]; ok {
		delete(subs, id)
		if len(subs) == 0 {
			delete(b.handlers, eventType)
		}
	}
	return nil
}

// Close shuts down the event bus.
func (b *InMemBus) Close() error {
	b.closeOnce.Do(func() {
		close(b.closed)
	})
	return nil
}
