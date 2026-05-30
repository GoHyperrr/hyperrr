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

	t.Run("Ownership Validation", func(t *testing.T) {
		key := "test-lock-owner"
		ctx1 := context.WithValue(context.Background(), LockOwnerKey, "owner-1")
		ctx2 := context.WithValue(context.Background(), LockOwnerKey, "owner-2")

		// Acquire under owner-1
		_, err := locker.Acquire(ctx1, key, 1*time.Second, 100*time.Millisecond)
		if err != nil {
			t.Fatalf("Acquire failed: %v", err)
		}

		// Try to release under owner-2 (should not release)
		_ = locker.Release(ctx2, key)

		// Try to acquire under owner-2 (should timeout because it's still locked by owner-1)
		ok, err := locker.Acquire(ctx2, key, 1*time.Second, 10*time.Millisecond)
		if err != ErrLockAcquisitionTimeout {
			t.Errorf("expected ErrLockAcquisitionTimeout, got %v (ok=%v)", err, ok)
		}

		// Release under owner-1 (should release)
		err = locker.Release(ctx1, key)
		if err != nil {
			t.Fatalf("Release failed: %v", err)
		}

		// Acquire under owner-2 (should succeed now)
		ok, err = locker.Acquire(ctx2, key, 1*time.Second, 10*time.Millisecond)
		if err != nil || !ok {
			t.Errorf("Acquire by owner-2 failed after release: %v", err)
		}
	})

	t.Run("Background Ticker Cleanup", func(t *testing.T) {
		// Temporarily speed up cleanup interval
		oldInterval := cleanupInterval
		cleanupInterval = 10 * time.Millisecond
		defer func() { cleanupInterval = oldInterval }()

		locker := NewInMemLocker()
		defer locker.Close()

		key := "test-cleanup-ticker"
		
		// Acquire lock with 5ms TTL (should expire almost immediately)
		_, err := locker.Acquire(ctx, key, 5*time.Millisecond, 100*time.Millisecond)
		if err != nil {
			t.Fatalf("Acquire failed: %v", err)
		}

		// Wait 30ms for the cleanup ticker to run
		time.Sleep(30 * time.Millisecond)

		// Check if we can acquire the lock immediately (should succeed since ticker should have cleaned it up)
		ok, err := locker.Acquire(ctx, key, 1*time.Second, 10*time.Millisecond)
		if err != nil {
			t.Fatalf("Acquire failed: %v", err)
		}
		if !ok {
			t.Fatal("expected lock to be cleaned up by background ticker and re-acquired")
		}
	})

	t.Run("Release non-existent key", func(t *testing.T) {
		err := locker.Release(ctx, "non-existent-key-12345")
		if err != nil {
			t.Errorf("expected no error releasing non-existent key, got %v", err)
		}
	})

	t.Run("Mid-loop Context Cancellation", func(t *testing.T) {
		key := "test-lock-mid-cancel"
		_, err := locker.Acquire(ctx, key, 1*time.Second, 100*time.Millisecond)
		if err != nil {
			t.Fatalf("Acquire failed: %v", err)
		}

		cCtx, cancel := context.WithCancel(context.Background())
		errChan := make(chan error, 1)
		go func() {
			_, err := locker.Acquire(cCtx, key, 1*time.Second, 1*time.Second)
			errChan <- err
		}()

		time.Sleep(10 * time.Millisecond)
		cancel()

		select {
		case err := <-errChan:
			if err != context.Canceled {
				t.Errorf("expected context.Canceled, got %v", err)
			}
		case <-time.After(200 * time.Millisecond):
			t.Error("timeout waiting for acquire to return after cancel")
		}
	})
}
