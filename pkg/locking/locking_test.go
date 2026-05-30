package locking

import (
	"context"
	"testing"
	"time"
)

func TestInMemLocker(t *testing.T) {
	ctx := context.Background()
	locker := NewInMemLocker()
	defer locker.Close()

	t.Run("Basic Acquire/Release", func(t *testing.T) {
		key := "test-lock-1"
		ok, err := locker.Acquire(ctx, key, 1*time.Second, 100*time.Millisecond)
		if err != nil {
			t.Fatalf("Acquire failed: %v", err)
		}
		if !ok {
			t.Fatal("expected to acquire lock")
		}

		err = locker.Release(ctx, key)
		if err != nil {
			t.Fatalf("Release failed: %v", err)
		}
		
		// Verify re-acquisition after manual release
		ok, err = locker.Acquire(ctx, key, 1*time.Second, 10*time.Millisecond)
		if err != nil || !ok { t.Fatal("re-acquisition failed") }
	})

	t.Run("Acquire Timeout", func(t *testing.T) {
		key := "test-lock-2"
		_, _ = locker.Acquire(ctx, key, 1*time.Second, 100*time.Millisecond)

		ok, err := locker.Acquire(ctx, key, 1*time.Second, 50*time.Millisecond)
		if err != ErrLockAcquisitionTimeout {
			t.Fatalf("expected ErrLockAcquisitionTimeout, got %v", err)
		}
		if ok {
			t.Fatal("expected to not acquire lock")
		}
	})

	t.Run("Auto-expiry (TTL)", func(t *testing.T) {
		key := "test-lock-3"
		_, _ = locker.Acquire(ctx, key, 100*time.Millisecond, 10*time.Millisecond)

		time.Sleep(200 * time.Millisecond)

		ok, err := locker.Acquire(ctx, key, 1*time.Second, 10*time.Millisecond)
		if err != nil {
			t.Fatalf("Acquire failed after expiry: %v", err)
		}
		if !ok {
			t.Fatal("expected to acquire lock after expiry")
		}
	})
	
	t.Run("Context Cancellation", func(t *testing.T) {
		key := "test-lock-ctx"
		cCtx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately
		
		_, err := locker.Acquire(cCtx, key, 1*time.Second, 1*time.Second)
		if err != context.Canceled {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	})
}
