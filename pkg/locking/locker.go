package locking

import (
	"context"
	"errors"
	"time"
)

var (
	ErrLockAcquisitionTimeout = errors.New("lock acquisition timed out")
	ErrLockNotHeld           = errors.New("lock not held")
)

type contextKey string

const LockOwnerKey contextKey = "lock_owner"

// Locker defines the interface for distributed locking.
type Locker interface {
	// Acquire attempts to acquire a lock for the given key.
	// ttl is the maximum time the lock can be held before auto-expiring.
	// timeout is the maximum time to wait for the lock to become available.
	Acquire(ctx context.Context, key string, ttl time.Duration, timeout time.Duration) (bool, error)
	
	// Release releases the lock for the given key.
	Release(ctx context.Context, key string) error
	
	// Close shuts down the locker.
	Close() error
}
