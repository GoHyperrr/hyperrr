package locking

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

var cleanupInterval = 1 * time.Minute

type lockEntry struct {
	owner     string
	expiresAt time.Time
}

// InMemLocker is a simple thread-safe in-memory locker.
type InMemLocker struct {
	mu      sync.Mutex
	locks   map[string]*lockEntry
	stop    chan struct{}
	ownerID string
}

// NewInMemLocker creates a new InMemLocker.
func NewInMemLocker() *InMemLocker {
	l := &InMemLocker{
		locks:   make(map[string]*lockEntry),
		stop:    make(chan struct{}),
		ownerID: "owner_" + uuid.New().String(),
	}
	go l.cleanupRoutine()
	return l
}

func (l *InMemLocker) getOwner(ctx context.Context) string {
	if owner, ok := ctx.Value(LockOwnerKey).(string); ok && owner != "" {
		return owner
	}
	return l.ownerID
}

func (l *InMemLocker) Acquire(ctx context.Context, key string, ttl time.Duration, timeout time.Duration) (bool, error) {
	start := time.Now()
	owner := l.getOwner(ctx)
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
				owner:     owner,
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
	
	entry, exists := l.locks[key]
	if !exists {
		return nil
	}
	
	if entry.owner == l.getOwner(ctx) {
		delete(l.locks, key)
	}
	return nil
}

func (l *InMemLocker) Close() error {
	close(l.stop)
	return nil
}

func (l *InMemLocker) cleanupRoutine() {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			l.mu.Lock()
			now := time.Now()
			for k, v := range l.locks {
				if now.After(v.expiresAt) {
					delete(l.locks, k)
				}
			}
			l.mu.Unlock()
		case <-l.stop:
			return
		}
	}
}
