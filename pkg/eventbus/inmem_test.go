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
		unsub, err := bus.Subscribe("test", "event", func(ctx context.Context, event Event) error {
			received <- event
			return nil
		})
		if err != nil {
			t.Fatalf("failed to subscribe: %v", err)
		}
		defer unsub()

		event := Event{ID: "1", Namespace: "test", Type: "event", Payload: map[string]any{"msg": "hello"}}
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

		unsub1, _ := bus.Subscribe("multi", "event", handler)
		unsub2, _ := bus.Subscribe("multi", "event", handler)
		defer unsub1()
		defer unsub2()

		_ = bus.Publish(ctx, Event{Namespace: "multi", Type: "event"})

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

		unsub, _ := bus.Subscribe("error", "event", func(ctx context.Context, event Event) error {
			return errors.New("forced error")
		})
		defer unsub()

		err := bus.Publish(ctx, Event{Namespace: "error", Type: "event"})
		if err != nil {
			t.Errorf("expected nil error from Publish even if handler fails, got %v", err)
		}
	})

	t.Run("Publish to Closed Bus", func(t *testing.T) {
		bus := NewInMemBus()
		bus.Close()

		err := bus.Publish(ctx, Event{Namespace: "closed", Type: "event"})
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	})

	t.Run("Multiple Close Calls", func(t *testing.T) {
		bus := NewInMemBus()
		_ = bus.Close()
		// Should not panic
		_ = bus.Close()
	})

	t.Run("Unsubscribe", func(t *testing.T) {
		bus := NewInMemBus()
		count := 0
		unsub, _ := bus.Subscribe("unsub", "event", func(ctx context.Context, event Event) error {
			count++
			return nil
		})

		_ = bus.Publish(ctx, Event{Namespace: "unsub", Type: "event"})
		if count != 1 {
			t.Errorf("expected 1 event, got %d", count)
		}

		unsub()
		_ = bus.Publish(ctx, Event{Namespace: "unsub", Type: "event"})
		if count != 1 {
			t.Errorf("expected still 1 event after unsubscribe, got %d", count)
		}
	})

	t.Run("Wait for handlers", func(t *testing.T) {
		bus := NewInMemBus()
		defer bus.Close()

		processed := false
		unsub, _ := bus.Subscribe("wait", "event", func(ctx context.Context, event Event) error {
			processed = true
			return nil
		})
		defer unsub()

		_ = bus.Publish(ctx, Event{Namespace: "wait", Type: "event"})

		if !processed {
			t.Error("expected event to be processed")
		}
	})

	t.Run("Async Mode", func(t *testing.T) {
		bus := NewInMemBus()
		bus.SetAsync(true)
		defer bus.Close()

		start := time.Now()
		handler := func(ctx context.Context, event Event) error {
			time.Sleep(50 * time.Millisecond)
			return nil
		}

		unsub1, _ := bus.Subscribe("async", "event", handler)
		unsub2, _ := bus.Subscribe("async", "event", handler)
		defer unsub1()
		defer unsub2()

		_ = bus.Publish(ctx, Event{Namespace: "async", Type: "event"})
		
		elapsed := time.Since(start)
		if elapsed >= 100*time.Millisecond {
			t.Errorf("expected async execution to be fast, took %v", elapsed)
		}
	})
}
