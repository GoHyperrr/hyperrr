package eventbus

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/GoHyperrr/hyperrr/pkg/logger"
	"github.com/nats-io/nats.go"
)

// NATSSubscription implements the Subscription interface for NATS.
type NATSSubscription struct {
	sub *nats.Subscription
	bus *NATSBus
}

func (s *NATSSubscription) Unsubscribe() error {
	if s.bus != nil {
		s.bus.removeSubscription(s)
	}
	return s.sub.Unsubscribe()
}

// NATSBus is a NATS-based event bus.
type NATSBus struct {
	conn      *nats.Conn
	ctx       context.Context
	cancel    context.CancelFunc
	mu        sync.Mutex
	subs      []*NATSSubscription
}

// NewNATSBus creates a new NATSBus.
func NewNATSBus(url string) (*NATSBus, error) {
	nc, err := nats.Connect(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	bus := &NATSBus{
		conn:   nc,
		ctx:    ctx,
		cancel: cancel,
		subs:   make([]*NATSSubscription, 0),
	}
	
	// Automatically clean up on context cancellation
	go func() {
		<-ctx.Done()
		bus.Close()
	}()
	
	return bus, nil
}

// SetContext sets the base context for event handlers and ties the bus lifecycle to it.
func (b *NATSBus) SetContext(ctx context.Context) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.cancel != nil {
		b.cancel() // Cancel the old background context
	}
	
	b.ctx, b.cancel = context.WithCancel(ctx)
	go func(c context.Context) {
		<-c.Done()
		b.Close()
	}(b.ctx)
}

func (b *NATSBus) Publish(ctx context.Context, event Event) error {
	if b.conn == nil {
		return fmt.Errorf("nats connection is nil")
	}
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}
	return b.conn.Publish(event.Type, data)
}

func (b *NATSBus) Subscribe(ctx context.Context, eventType string, handler EventHandler) (Subscription, error) {
	if b.conn == nil {
		return nil, fmt.Errorf("nats connection is nil")
	}
	sub, err := b.conn.Subscribe(eventType, func(m *nats.Msg) {
		var event Event
		if err := json.Unmarshal(m.Data, &event); err != nil {
			logger.Error("failed to unmarshal NATS event", "type", eventType, "error", err)
			return
		}
		if err := handler(b.ctx, event); err != nil {
			logger.Error("nats event handler failed", "type", event.Type, "id", event.ID, "error", err)
		}
	})
	if err != nil {
		return nil, err
	}
	
	natsSub := &NATSSubscription{sub: sub, bus: b}
	
	b.mu.Lock()
	b.subs = append(b.subs, natsSub)
	b.mu.Unlock()
	
	return natsSub, nil
}

func (b *NATSBus) removeSubscription(s *NATSSubscription) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for i, sub := range b.subs {
		if sub == s {
			b.subs = append(b.subs[:i], b.subs[i+1:]...)
			break
		}
	}
}

func (b *NATSBus) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	if b.cancel != nil {
		b.cancel()
		b.cancel = nil
	}

	for _, sub := range b.subs {
		sub.sub.Unsubscribe()
	}
	b.subs = nil

	if b.conn != nil {
		b.conn.Close()
		b.conn = nil
	}
	return nil
}
