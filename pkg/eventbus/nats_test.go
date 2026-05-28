package eventbus

import (
	"context"
	"testing"
)

func TestNATSBusErrorPaths(t *testing.T) {
	t.Run("Connect Error", func(t *testing.T) {
		// Invalid URL should cause connection failure
		_, err := NewNATSBus("nats://invalid-host-that-does-not-exist:4222")
		if err == nil {
			t.Error("expected error for invalid NATS host")
		}
	})

	t.Run("Publish Marshal Error", func(t *testing.T) {
		bus := &NATSBus{} // nil connection
		event := Event{
			Payload: make(chan int), // Cannot be marshaled
		}
		err := bus.Publish(context.Background(), event)
		if err == nil {
			t.Error("expected error for non-marshallable payload")
		}
	})
}
