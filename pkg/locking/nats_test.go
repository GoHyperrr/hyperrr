package locking

import (
	"bufio"
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type mockKeyValueEntry struct {
	jetstream.KeyValueEntry
	key string
	val []byte
	rev uint64
}

func (e *mockKeyValueEntry) Key() string      { return e.key }
func (e *mockKeyValueEntry) Value() []byte    { return e.val }
func (e *mockKeyValueEntry) Revision() uint64 { return e.rev }
func (e *mockKeyValueEntry) Created() time.Time { return time.Now() }

type mockKeyValue struct {
	jetstream.KeyValue
	store     map[string][]byte
	revs      map[string]uint64
	getErr    error
	updateErr error
}

func (m *mockKeyValue) Get(ctx context.Context, key string) (jetstream.KeyValueEntry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if m.getErr != nil {
		return nil, m.getErr
	}
	val, ok := m.store[key]
	if !ok {
		return nil, jetstream.ErrKeyNotFound
	}
	return &mockKeyValueEntry{key: key, val: val, rev: m.revs[key]}, nil
}

func (m *mockKeyValue) Create(ctx context.Context, key string, value []byte, opts ...jetstream.KVCreateOpt) (uint64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if _, exists := m.store[key]; exists {
		return 0, jetstream.ErrKeyExists
	}
	m.revs[key]++
	m.store[key] = value
	return m.revs[key], nil
}

func (m *mockKeyValue) Update(ctx context.Context, key string, value []byte, revision uint64) (uint64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if m.updateErr != nil {
		return 0, m.updateErr
	}
	currentRev, exists := m.revs[key]
	if !exists || currentRev != revision {
		return 0, errors.New("revision mismatch")
	}
	m.revs[key]++
	m.store[key] = value
	return m.revs[key], nil
}

func (m *mockKeyValue) Delete(ctx context.Context, key string, opts ...jetstream.KVDeleteOpt) error {
	delete(m.store, key)
	delete(m.revs, key)
	return nil
}

func TestNATSLocker_OwnershipAndRelease(t *testing.T) {
	store := make(map[string][]byte)
	revs := make(map[string]uint64)
	kv := &mockKeyValue{store: store, revs: revs}

	locker := &NATSLocker{
		kv:      kv,
		ownerID: "nats-owner-default",
	}

	t.Run("Acquire and Release by same owner", func(t *testing.T) {
		// Clear mock
		for k := range store {
			delete(store, k)
			delete(revs, k)
		}

		ctx := context.WithValue(context.Background(), LockOwnerKey, "owner-1")
		key := "resource-1"

		// 1. Acquire
		ok, err := locker.Acquire(ctx, key, 1*time.Second, 100*time.Millisecond)
		if err != nil {
			t.Fatalf("failed to acquire: %v", err)
		}
		if !ok {
			t.Fatal("expected to acquire")
		}

		// Verify stored value
		lockKey := "lock." + key
		val := string(store[lockKey])
		if !strings.HasPrefix(val, "owner-1:") {
			t.Errorf("expected stored value to start with owner-1:, got %v", val)
		}

		// 2. Release
		err = locker.Release(ctx, key)
		if err != nil {
			t.Fatalf("failed to release: %v", err)
		}

		// Verify deleted/released
		if _, exists := store[lockKey]; exists {
			t.Error("expected lock to be deleted after release")
		}
	})

	t.Run("Reject release by different owner", func(t *testing.T) {
		// Clear mock
		for k := range store {
			delete(store, k)
			delete(revs, k)
		}

		ctx1 := context.WithValue(context.Background(), LockOwnerKey, "owner-1")
		ctx2 := context.WithValue(context.Background(), LockOwnerKey, "owner-2")
		key := "resource-2"

		// Acquire under owner-1
		_, _ = locker.Acquire(ctx1, key, 1*time.Second, 100*time.Millisecond)

		// Release under owner-2
		err := locker.Release(ctx2, key)
		if err != nil {
			t.Fatalf("Release failed: %v", err)
		}

		// Check still held by owner-1
		lockKey := "lock." + key
		val := string(store[lockKey])
		if !strings.HasPrefix(val, "owner-1:") {
			t.Errorf("expected lock to still be held by owner-1, got %v", val)
		}
	})

	t.Run("Acquire expired lock", func(t *testing.T) {
		// Clear mock
		for k := range store {
			delete(store, k)
			delete(revs, k)
		}

		ctx1 := context.WithValue(context.Background(), LockOwnerKey, "owner-1")
		ctx2 := context.WithValue(context.Background(), LockOwnerKey, "owner-2")
		key := "resource-3"

		// Acquire under owner-1 with short TTL
		_, _ = locker.Acquire(ctx1, key, 10*time.Millisecond, 100*time.Millisecond)

		// Sleep to let it expire
		time.Sleep(20 * time.Millisecond)

		// Acquire under owner-2 (should succeed because it's expired)
		ok, err := locker.Acquire(ctx2, key, 1*time.Second, 100*time.Millisecond)
		if err != nil {
			t.Fatalf("failed to acquire expired lock: %v", err)
		}
		if !ok {
			t.Fatal("expected to acquire expired lock")
		}

		// Verify stored value is now owner-2
		lockKey := "lock." + key
		val := string(store[lockKey])
		if !strings.HasPrefix(val, "owner-2:") {
			t.Errorf("expected stored value to start with owner-2:, got %v", val)
		}
	})

	t.Run("Acquire Timeout", func(t *testing.T) {
		// Clear mock
		for k := range store {
			delete(store, k)
			delete(revs, k)
		}

		ctx1 := context.WithValue(context.Background(), LockOwnerKey, "owner-1")
		ctx2 := context.WithValue(context.Background(), LockOwnerKey, "owner-2")
		key := "resource-4"

		// Acquire under owner-1 with 1s TTL
		ok1, err1 := locker.Acquire(ctx1, key, 1*time.Second, 100*time.Millisecond)
		if err1 != nil || !ok1 {
			t.Fatalf("first acquire failed: err=%v, ok=%v", err1, ok1)
		}

		// Try to acquire under owner-2 with 10ms timeout (should fail with timeout)
		ok2, err2 := locker.Acquire(ctx2, key, 1*time.Second, 10*time.Millisecond)
		if err2 != ErrLockAcquisitionTimeout {
			t.Errorf("expected ErrLockAcquisitionTimeout, got %v (ok=%v), store=%s", err2, ok2, string(store["lock."+key]))
		}
	})
	
	t.Run("Default owner fallback", func(t *testing.T) {
		for k := range store {
			delete(store, k)
			delete(revs, k)
		}
		key := "resource-5"
		ok, err := locker.Acquire(context.Background(), key, 1*time.Second, 100*time.Millisecond)
		if err != nil || !ok {
			t.Fatalf("acquire with default owner failed: %v", err)
		}
		lockKey := "lock." + key
		val := string(store[lockKey])
		if !strings.HasPrefix(val, locker.ownerID+":") {
			t.Errorf("expected owner %s, got value %s", locker.ownerID, val)
		}
		err = locker.Release(context.Background(), key)
		if err != nil {
			t.Fatalf("release with default owner failed: %v", err)
		}
	})

	t.Run("Context cancelled", func(t *testing.T) {
		for k := range store {
			delete(store, k)
			delete(revs, k)
		}
		key := "resource-6"
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		ok, err := locker.Acquire(ctx, key, 1*time.Second, 100*time.Millisecond)
		if err != context.Canceled {
			t.Errorf("expected context.Canceled, got %v (ok=%v)", err, ok)
		}
	})

	t.Run("Release get error", func(t *testing.T) {
		key := "resource-7"
		expectedErr := errors.New("mock get error")
		kv.getErr = expectedErr
		defer func() { kv.getErr = nil }()

		err := locker.Release(context.Background(), key)
		if !errors.Is(err, expectedErr) {
			t.Errorf("expected mock get error, got %v", err)
		}
	})

	t.Run("Release update mismatch error", func(t *testing.T) {
		for k := range store {
			delete(store, k)
			delete(revs, k)
		}
		key := "resource-8"
		_, _ = locker.Acquire(context.Background(), key, 1*time.Second, 100*time.Millisecond)
		expectedErr := errors.New("mock update error")
		kv.updateErr = expectedErr
		defer func() { kv.updateErr = nil }()

		err := locker.Release(context.Background(), key)
		if err != nil {
			t.Errorf("expected nil error on release update failure, got %v", err)
		}
	})

	t.Run("Close", func(t *testing.T) {
		err := locker.Close()
		if err != nil {
			t.Errorf("Close failed: %v", err)
		}
	})

	t.Run("Mid-loop Context Cancellation", func(t *testing.T) {
		for k := range store {
			delete(store, k)
			delete(revs, k)
		}
		key := "mid-cancel-nats"
		_, _ = locker.Acquire(context.Background(), key, 1*time.Second, 100*time.Millisecond)

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

	t.Run("Acquire Get error", func(t *testing.T) {
		for k := range store {
			delete(store, k)
			delete(revs, k)
		}
		key := "get-err-nats"
		_, _ = locker.Acquire(context.Background(), key, 1*time.Second, 100*time.Millisecond)

		expectedErr := errors.New("mock get error")
		kv.getErr = expectedErr
		defer func() { kv.getErr = nil }()

		_, err := locker.Acquire(context.Background(), key, 1*time.Second, 10*time.Millisecond)
		if err != ErrLockAcquisitionTimeout {
			t.Errorf("expected ErrLockAcquisitionTimeout on Get error during acquire, got %v", err)
		}
	})

	t.Run("Acquire Expired Lock Parse error", func(t *testing.T) {
		for k := range store {
			delete(store, k)
			delete(revs, k)
		}
		key := "parse-err-nats"
		store["lock."+key] = []byte("owner-1:invalid-timestamp")
		revs["lock."+key] = 1

		_, err := locker.Acquire(context.Background(), key, 1*time.Second, 10*time.Millisecond)
		if err != ErrLockAcquisitionTimeout {
			t.Errorf("expected ErrLockAcquisitionTimeout, got %v", err)
		}
	})

	t.Run("Acquire Expired Lock Update mismatch", func(t *testing.T) {
		for k := range store {
			delete(store, k)
			delete(revs, k)
		}
		key := "update-mismatch-nats"
		store["lock."+key] = []byte("owner-1:" + time.Now().Add(-10*time.Second).Format(time.RFC3339Nano))
		revs["lock."+key] = 1

		expectedErr := errors.New("mock update error")
		kv.updateErr = expectedErr
		defer func() { kv.updateErr = nil }()

		_, err := locker.Acquire(context.Background(), key, 1*time.Second, 10*time.Millisecond)
		if err != ErrLockAcquisitionTimeout {
			t.Errorf("expected ErrLockAcquisitionTimeout on Update error, got %v", err)
		}
	})
}

func TestNewNATSLocker(t *testing.T) {
	t.Run("Nil connection", func(t *testing.T) {
		_, err := NewNATSLocker(context.Background(), nil, "testbucket")
		if err == nil {
			t.Error("expected error with nil connection")
		}
	})

	t.Run("Dummy NATS connection fail CreateOrUpdateKeyValue", func(t *testing.T) {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("failed to listen: %v", err)
		}
		defer ln.Close()

		go func() {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			defer conn.Close()
			_, _ = conn.Write([]byte("INFO {\"server_id\":\"mock\",\"max_payload\":1048576}\r\n"))
			reader := bufio.NewReader(conn)
			for {
				line, err := reader.ReadString('\n')
				if err != nil {
					return
				}
				if strings.Contains(line, "PING") {
					_, _ = conn.Write([]byte("PONG\r\n"))
				}
			}
		}()

		addr := "nats://" + ln.Addr().String()
		nc, err := nats.Connect(addr, nats.Timeout(2*time.Second))
		if err != nil {
			t.Fatalf("failed to connect to mock: %v", err)
		}
		defer nc.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		_, err = NewNATSLocker(ctx, nc, "testbucket")
		if err == nil {
			t.Error("expected error creating KeyValue bucket on mock server")
		}
	})
}
