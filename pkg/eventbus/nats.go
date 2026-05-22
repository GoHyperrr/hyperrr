package eventbus

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nats-io/nats.go"
)

// NATSBus is a NATS-based event bus.
type NATSBus struct {
	conn *nats.Conn
}

// NewNATSBus creates a new NATSBus.
func NewNATSBus(url string) (*NATSBus, error) {
	nc, err := nats.Connect(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}
	return &NATSBus{conn: nc}, nil
}

func (b *NATSBus) Publish(ctx context.Context, event Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}
	return b.conn.Publish(event.Type, data)
}

func (b *NATSBus) Subscribe(ctx context.Context, eventType string, handler EventHandler) error {
	_, err := b.conn.Subscribe(eventType, func(m *nats.Msg) {
		var event Event
		if err := json.Unmarshal(m.Data, &event); err != nil {
			return // Log error
		}
		handler(context.Background(), event)
	})
	return err
}

func (b *NATSBus) Close() error {
	b.conn.Close()
	return nil
}
