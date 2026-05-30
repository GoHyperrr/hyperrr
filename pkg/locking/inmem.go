package locking

import (
	"context"
	"sync"
	"time"
)

type lockEntry struct {
	expiresAt time.Time
}

// InMemLocker is a simple thread-safe in-memory locker.
type InMemLocker struct {
	mu    sync.Mutex
	locks map[string]*lockEntry
}

// NewInMemLocker creates a new InMemLocker.
func NewInMemLocker() *InMemLocker {
	l := &InMemLocker{
		locks: make(map[string]*lockEntry),
	}
	go l.cleanupRoutine()
	return l
}

func (l *InMemLocker) Acquire(ctx context.Context, key string, ttl time.Duration, timeout time.Duration) (bool, error) {
	start := time.Now()
	for {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		default:
		}

		l.mu.Lock()
		entry, exists := l.locks[key]
		now := time.Now()

		if !exists || now.After(entry.expiresAt) {
			l.locks[key] = &lockEntry{
				expiresAt: now.Add(ttl),
			}
			l.mu.Unlock()
			return true, nil
		}
		l.mu.Unlock()

		if time.Since(start) >= timeout {
			return false, ErrLockAcquisitionTimeout
		}

		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-time.After(50 * time.Millisecond):
			// Retry
		}
	}
}

func (l *InMemLocker) Release(ctx context.Context, key string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.locks, key)
	return nil
}

func (l *InMemLocker) Close() error {
	return nil
}

func (l *InMemLocker) cleanupRoutine() {
	ticker := time.NewTicker(1 * time.Minute)
	for range ticker.C {
		l.mu.Lock()
		now := time.Now()
		for k, v := range l.locks {
			if now.After(v.expiresAt) {
				delete(l.locks, k)
			}
		}
		l.mu.Unlock()
	}
}
