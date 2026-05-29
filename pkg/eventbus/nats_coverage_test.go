package eventbus

import (
	"context"
	"testing"
)

func TestNATSBus_Coverage(t *testing.T) {
	ctx := context.Background()
	bus := &NATSBus{conn: nil}

	t.Run("Publish with nil conn", func(t *testing.T) {
		err := bus.Publish(ctx, Event{Type: "test"})
		if err == nil {
			t.Error("expected error when publishing with nil conn")
		}
	})

	t.Run("Subscribe with nil conn", func(t *testing.T) {
		_, err := bus.Subscribe(ctx, "test", func(ctx context.Context, e Event) error { return nil })
		if err == nil {
			t.Error("expected error when subscribing with nil conn")
		}
	})

	t.Run("Close with nil conn", func(t *testing.T) {
		err := bus.Close()
		if err != nil {
			t.Errorf("unexpected error when closing with nil conn: %v", err)
		}
	})
}
