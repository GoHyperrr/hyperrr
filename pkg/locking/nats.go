package locking

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// NATSLocker implements Locker using NATS JetStream KeyValue.
type NATSLocker struct {
	kv      jetstream.KeyValue
	ownerID string
}

// NewNATSLocker creates a new NATSLocker.
func NewNATSLocker(ctx context.Context, nc *nats.Conn, bucketName string) (*NATSLocker, error) {
	if nc == nil {
		return nil, fmt.Errorf("nats connection is nil")
	}
	js, err := jetstream.New(nc)
	if err != nil {
		return nil, fmt.Errorf("failed to get jetstream context: %w", err)
	}

	kv, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket:      bucketName,
		Description: "Distributed locks",
		TTL:         10 * time.Minute, // Maximum safety TTL for any lock
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create or update kv bucket: %w", err)
	}

	return &NATSLocker{
		kv:      kv,
		ownerID: "owner_" + uuid.New().String(),
	}, nil
}

func (l *NATSLocker) getOwner(ctx context.Context) string {
	if owner, ok := ctx.Value(LockOwnerKey).(string); ok && owner != "" {
		return owner
	}
	return l.ownerID
}

func (l *NATSLocker) Acquire(ctx context.Context, key string, ttl time.Duration, timeout time.Duration) (bool, error) {
	start := time.Now()
	lockKey := fmt.Sprintf("lock.%s", key)
	owner := l.getOwner(ctx)

	for {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		expiresAt := time.Now().Add(ttl)
		valStr := fmt.Sprintf("%s:%s", owner, expiresAt.Format(time.RFC3339Nano))

		// Use Put with Create revision check for atomicity
		_, err := l.kv.Create(ctx, lockKey, []byte(valStr))
		if err == nil {
			return true, nil
		}

		// Check if it's already locked and if it's expired
		entry, err := l.kv.Get(ctx, lockKey)
		if err == nil {
			val := string(entry.Value())
			parts := strings.SplitN(val, ":", 2)
			if len(parts) == 2 {
				exp, parseErr := time.Parse(time.RFC3339Nano, parts[1])
				if parseErr == nil && time.Now().After(exp) {
					// Atomic update to take the expired lock
					_, err = l.kv.Update(ctx, lockKey, []byte(valStr), entry.Revision())
					if err == nil {
						return true, nil
					}
				}
			}
		}

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

func (l *NATSLocker) Release(ctx context.Context, key string) error {
	lockKey := fmt.Sprintf("lock.%s", key)
	owner := l.getOwner(ctx)

	entry, err := l.kv.Get(ctx, lockKey)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			return nil // Already released or expired
		}
		return err
	}

	val := string(entry.Value())
	parts := strings.SplitN(val, ":", 2)
	if len(parts) > 0 && parts[0] == owner {
		// Update value to indicate it's released, using the revision check
		_, err = l.kv.Update(ctx, lockKey, []byte("released:"), entry.Revision())
		if err != nil {
			// Revision mismatch: lock was already overwritten / re-acquired by someone else. Do nothing.
			return nil
		}
		// Soft-delete the lock
		_ = l.kv.Delete(ctx, lockKey)
	}

	return nil
}

func (l *NATSLocker) Close() error {
	return nil
}
