package locking

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

type mockRedisHook struct {
	store      map[string]string
	forceError error
}

func (h *mockRedisHook) DialHook(next redis.DialHook) redis.DialHook {
	return next
}

func (h *mockRedisHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}

func (h *mockRedisHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		if h.forceError != nil {
			return h.forceError
		}

		name := strings.ToLower(cmd.Name())
		args := cmd.Args()

		if name == "setnx" {
			if len(args) < 3 {
				return nil
			}
			key := args[1].(string)
			val := args[2].(string)

			if _, exists := h.store[key]; exists {
				if boolCmd, ok := cmd.(*redis.BoolCmd); ok {
					boolCmd.SetVal(false)
				}
				return nil
			}
			h.store[key] = val
			if boolCmd, ok := cmd.(*redis.BoolCmd); ok {
				boolCmd.SetVal(true)
			}
			return nil
		} else if name == "set" {
			if len(args) < 3 {
				return nil
			}
			key := args[1].(string)
			val := args[2].(string)

			// Check if NX option is specified in SET command
			isNX := false
			for _, arg := range args {
				if s, ok := arg.(string); ok && strings.ToLower(s) == "nx" {
					isNX = true
				}
			}

			if isNX {
				if _, exists := h.store[key]; exists {
					if boolCmd, ok := cmd.(*redis.BoolCmd); ok {
						boolCmd.SetVal(false)
					}
					return nil
				}
				h.store[key] = val
				if boolCmd, ok := cmd.(*redis.BoolCmd); ok {
					boolCmd.SetVal(true)
				}
				return nil
			}
		} else if name == "eval" {
			if len(args) < 5 {
				return nil
			}
			key := args[3].(string)
			val := args[4].(string)

			if currentVal, exists := h.store[key]; exists && currentVal == val {
				delete(h.store, key)
				if cmdCmd, ok := cmd.(*redis.Cmd); ok {
					cmdCmd.SetVal(int64(1))
				}
				return nil
			} else {
				if cmdCmd, ok := cmd.(*redis.Cmd); ok {
					cmdCmd.SetVal(int64(0))
				}
				return nil
			}
		}

		return nil
	}
}

func TestRedisLocker_OwnershipAndRelease(t *testing.T) {
	// Create a dummy redis client
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379", // Dummy address, won't connect
	})

	// In-memory mock store to track Redis keys
	store := make(map[string]string)
	
	// Add custom hook for mocking
	hook := &mockRedisHook{store: store}
	client.AddHook(hook)

	t.Run("Acquire and Release by same owner", func(t *testing.T) {
		// Clear mock store
		for k := range store {
			delete(store, k)
		}
		hook.forceError = nil

		locker := NewRedisLocker(client)
		ctx := context.WithValue(context.Background(), LockOwnerKey, "owner-1")
		key := "resource-1"

		// 1. Acquire lock
		ok, err := locker.Acquire(ctx, key, 1*time.Second, 100*time.Millisecond)
		if err != nil {
			t.Fatalf("failed to acquire lock: %v", err)
		}
		if !ok {
			t.Fatal("expected to acquire lock")
		}

		// Verify stored value
		if store["lock:"+key] != "owner-1" {
			t.Errorf("expected stored owner to be owner-1, got %v", store["lock:"+key])
		}

		// 2. Release lock
		err = locker.Release(ctx, key)
		if err != nil {
			t.Fatalf("failed to release lock: %v", err)
		}

		// Verify deleted from store
		if _, exists := store["lock:"+key]; exists {
			t.Error("expected lock to be deleted from store after release")
		}
	})

	t.Run("Reject release by different owner", func(t *testing.T) {
		// Clear mock store
		for k := range store {
			delete(store, k)
		}
		hook.forceError = nil

		locker := NewRedisLocker(client)
		ctx1 := context.WithValue(context.Background(), LockOwnerKey, "owner-1")
		ctx2 := context.WithValue(context.Background(), LockOwnerKey, "owner-2")
		key := "resource-2"

		// 1. Acquire lock under owner-1
		_, _ = locker.Acquire(ctx1, key, 1*time.Second, 100*time.Millisecond)

		// 2. Try to release under owner-2
		err := locker.Release(ctx2, key)
		if err != nil {
			t.Fatalf("Release errored: %v", err)
		}

		// Verify lock is STILL held by owner-1 (not released)
		if store["lock:"+key] != "owner-1" {
			t.Errorf("expected lock to still be held by owner-1, got %v", store["lock:"+key])
		}
	})

	t.Run("Fallback to instance owner ID if no context owner", func(t *testing.T) {
		// Clear mock store
		for k := range store {
			delete(store, k)
		}
		hook.forceError = nil

		locker := NewRedisLocker(client)
		ctx := context.Background() // No LockOwnerKey
		key := "resource-3"

		// 1. Acquire lock
		_, _ = locker.Acquire(ctx, key, 1*time.Second, 100*time.Millisecond)

		// Verify stored value is the instance owner ID
		expectedOwner := locker.ownerID
		if store["lock:"+key] != expectedOwner {
			t.Errorf("expected stored owner to be %s, got %v", expectedOwner, store["lock:"+key])
		}

		// 2. Try to release using a context with different owner
		ctxWrong := context.WithValue(context.Background(), LockOwnerKey, "some-other-owner")
		_ = locker.Release(ctxWrong, key)

		// Lock should still be held
		if store["lock:"+key] != expectedOwner {
			t.Error("lock was incorrectly released by wrong owner")
		}

		// 3. Release using empty context (should resolve to the same instance owner ID)
		err := locker.Release(ctx, key)
		if err != nil {
			t.Fatalf("Release failed: %v", err)
		}

		// Lock should be released
		if _, exists := store["lock:"+key]; exists {
			t.Error("lock was not released by original locker instance")
		}
	})

	t.Run("Acquire Timeout", func(t *testing.T) {
		for k := range store {
			delete(store, k)
		}
		hook.forceError = nil

		locker := NewRedisLocker(client)
		ctx := context.Background()
		key := "resource-timeout"

		// Manually occupy the lock
		store["lock:"+key] = "some-owner"

		ok, err := locker.Acquire(ctx, key, 1*time.Second, 10*time.Millisecond)
		if err != ErrLockAcquisitionTimeout {
			t.Errorf("expected ErrLockAcquisitionTimeout, got %v (ok=%v)", err, ok)
		}
	})

	t.Run("Acquire Context Cancelled", func(t *testing.T) {
		for k := range store {
			delete(store, k)
		}
		hook.forceError = nil

		locker := NewRedisLocker(client)
		cCtx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel context immediately
		key := "resource-cancelled"

		// Manually occupy to force loop retry/exit check
		store["lock:"+key] = "some-owner"

		ok, err := locker.Acquire(cCtx, key, 1*time.Second, 100*time.Millisecond)
		if err != context.Canceled {
			t.Errorf("expected context.Canceled, got %v (ok=%v)", err, ok)
		}
	})

	t.Run("Acquire Redis Error", func(t *testing.T) {
		hook.forceError = errors.New("redis connection refused")
		locker := NewRedisLocker(client)
		ok, err := locker.Acquire(context.Background(), "err-key", 1*time.Second, 100*time.Millisecond)
		if err == nil || !strings.Contains(err.Error(), "redis connection refused") {
			t.Errorf("expected Redis error, got %v (ok=%v)", err, ok)
		}
	})

	t.Run("Release Redis Error", func(t *testing.T) {
		hook.forceError = errors.New("redis connection refused")
		locker := NewRedisLocker(client)
		err := locker.Release(context.Background(), "err-key")
		if err == nil || !strings.Contains(err.Error(), "redis connection refused") {
			t.Errorf("expected Redis error, got %v", err)
		}
	})

	t.Run("Close", func(t *testing.T) {
		locker := NewRedisLocker(client)
		err := locker.Close()
		if err != nil {
			t.Errorf("expected nil error on Close, got %v", err)
		}
	})
}
