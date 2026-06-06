package locking

import (
	"context"
	"errors"
	"sync"
	"time"
)

var (
	ErrLockAcquisitionTimeout = errors.New("lock acquisition timed out")
	ErrLockNotHeld           = errors.New("lock not held")
)

type LockerProvider func(url string, bucketOrPrefix string) (Locker, error)

var (
	lockersMu sync.RWMutex
	lockers   = make(map[string]LockerProvider)
)

func RegisterLocker(name string, provider LockerProvider) {
	lockersMu.Lock()
	defer lockersMu.Unlock()
	lockers[name] = provider
}

func GetLocker(name string) (LockerProvider, bool) {
	lockersMu.RLock()
	defer lockersMu.RUnlock()
	l, ok := lockers[name]
	return l, ok
}

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
