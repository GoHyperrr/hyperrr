package eventbus

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/GoHyperrr/hyperrr/pkg/logger"
	"github.com/nats-io/nats.go"
)

// NATSBus is a NATS-based event bus.
type NATSBus struct {
	conn   *nats.Conn
	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.Mutex
	subs   []*nats.Subscription
}

// NewNATSBus creates a new NATSBus.
func NewNATSBus(url string) (*NATSBus, error) {
	nc, err := nats.Connect(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &NATSBus{
		conn:   nc,
		ctx:    ctx,
		cancel: cancel,
	}, nil
}

// SetContext sets the base context for event handlers and ties the bus lifecycle to it.
func (b *NATSBus) SetContext(ctx context.Context) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.cancel != nil {
		b.cancel()
	}
	b.ctx, b.cancel = context.WithCancel(ctx)
}

// Publish publishes an event to NATS.
func (b *NATSBus) Publish(ctx context.Context, event Event) error {
	if b.conn == nil {
		return fmt.Errorf("nats connection is nil")
	}
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}
	subject := topicKey(event.Namespace, event.Type)
	return b.conn.Publish(subject, data)
}

// Subscribe subscribes to a topic.
func (b *NATSBus) Subscribe(namespace, eventType string, handler EventHandler) (func(), error) {
	if b.conn == nil {
		return nil, fmt.Errorf("nats connection is nil")
	}
	subject := topicKey(namespace, eventType)
	sub, err := b.conn.Subscribe(subject, func(m *nats.Msg) {
		var event Event
		if err := json.Unmarshal(m.Data, &event); err != nil {
			logger.Error("failed to unmarshal NATS event", "subject", subject, "error", err)
			return
		}
		
		b.mu.Lock()
		activeCtx := b.ctx
		b.mu.Unlock()
		
		if err := handler(activeCtx, event); err != nil {
			logger.Error("nats event handler failed", "subject", subject, "id", event.ID, "error", err)
		}
	})
	if err != nil {
		return nil, err
	}

	b.mu.Lock()
	b.subs = append(b.subs, sub)
	b.mu.Unlock()

	unsubscribe := func() {
		_ = sub.Unsubscribe()
		b.mu.Lock()
		defer b.mu.Unlock()
		for i, s := range b.subs {
			if s == sub {
				b.subs = append(b.subs[:i], b.subs[i+1:]...)
				break
			}
		}
	}

	return unsubscribe, nil
}

// Close closes the NATS event bus.
func (b *NATSBus) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.cancel != nil {
		b.cancel()
		b.cancel = nil
	}

	for _, sub := range b.subs {
		_ = sub.Unsubscribe()
	}
	b.subs = nil

	if b.conn != nil {
		b.conn.Close()
		b.conn = nil
	}
	return nil
}

// Conn returns the underlying NATS connection.
func (b *NATSBus) Conn() *nats.Conn {
	return b.conn
}
