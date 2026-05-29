package eventbus

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/GoHyperrr/hyperrr/pkg/logger"
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
	if b.conn == nil {
		return fmt.Errorf("nats connection is nil")
	}
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}
	return b.conn.Publish(event.Type, data)
}

func (b *NATSBus) Subscribe(ctx context.Context, eventType string, handler EventHandler) error {
	if b.conn == nil {
		return fmt.Errorf("nats connection is nil")
	}
	_, err := b.conn.Subscribe(eventType, func(m *nats.Msg) {
		var event Event
		if err := json.Unmarshal(m.Data, &event); err != nil {
			logger.Error("failed to unmarshal NATS event", "type", eventType, "error", err)
			return
		}
		if err := handler(context.Background(), event); err != nil {
			logger.Error("nats event handler failed", "type", event.Type, "id", event.ID, "error", err)
		}
	})
	return err
}

func (b *NATSBus) Close() error {
	if b.conn != nil {
		b.conn.Close()
	}
	return nil
}
