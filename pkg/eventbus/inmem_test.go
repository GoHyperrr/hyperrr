package eventbus

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestInMemBus(t *testing.T) {
	ctx := context.Background()

	t.Run("Publish and Subscribe", func(t *testing.T) {
		bus := NewInMemBus()
		defer bus.Close()

		received := make(chan Event, 1)
		err := bus.Subscribe(ctx, "test.event", func(ctx context.Context, event Event) error {
			received <- event
			return nil
		})
		if err != nil {
			t.Fatalf("failed to subscribe: %v", err)
		}

		event := Event{ID: "1", Type: "test.event", Payload: "hello"}
		err = bus.Publish(ctx, event)
		if err != nil {
			t.Fatalf("failed to publish: %v", err)
		}

		select {
		case ev := <-received:
			if ev.ID != "1" {
				t.Errorf("expected ID 1, got %s", ev.ID)
			}
		case <-time.After(100 * time.Millisecond):
			t.Error("timed out waiting for event")
		}
	})

	t.Run("Multiple Subscribers", func(t *testing.T) {
		bus := NewInMemBus()
		defer bus.Close()

		var wg sync.WaitGroup
		wg.Add(2)

		handler := func(ctx context.Context, event Event) error {
			wg.Done()
			return nil
		}

		bus.Subscribe(ctx, "multi.event", handler)
		bus.Subscribe(ctx, "multi.event", handler)

		bus.Publish(ctx, Event{Type: "multi.event"})

		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(100 * time.Millisecond):
			t.Error("timed out waiting for multiple subscribers")
		}
	})

	t.Run("Handler Error", func(t *testing.T) {
		bus := NewInMemBus()
		defer bus.Close()

		bus.Subscribe(ctx, "error.event", func(ctx context.Context, event Event) error {
			return errors.New("forced error")
		})

		// This should not crash and should log (we can't easily check logs here without capturing stderr)
		err := bus.Publish(ctx, Event{Type: "error.event"})
		if err != nil {
			t.Errorf("expected nil error from Publish even if handler fails, got %v", err)
		}
	})

	t.Run("Publish to Closed Bus", func(t *testing.T) {
		bus := NewInMemBus()
		bus.Close()

		err := bus.Publish(ctx, Event{Type: "closed.event"})
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	})

	t.Run("Multiple Close Calls", func(t *testing.T) {
		bus := NewInMemBus()
		bus.Close()
		// Should not panic
		bus.Close()
	})

	t.Run("Wait for handlers", func(t *testing.T) {
		bus := NewInMemBus()
		defer bus.Close()

		processed := false
		bus.Subscribe(ctx, "wait.event", func(ctx context.Context, event Event) error {
			processed = true
			return nil
		})

		bus.Publish(ctx, Event{Type: "wait.event"})

		if !processed {
			t.Error("expected event to be processed")
		}
	})
}
