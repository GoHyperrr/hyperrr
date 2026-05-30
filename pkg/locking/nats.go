package locking

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// NATSLocker implements Locker using NATS JetStream KeyValue.
type NATSLocker struct {
	kv jetstream.KeyValue
}

// NewNATSLocker creates a new NATSLocker.
func NewNATSLocker(ctx context.Context, nc *nats.Conn, bucketName string) (*NATSLocker, error) {
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

	return &NATSLocker{kv: kv}, nil
}

func (l *NATSLocker) Acquire(ctx context.Context, key string, ttl time.Duration, timeout time.Duration) (bool, error) {
	start := time.Now()
	lockKey := fmt.Sprintf("lock.%s", key)

	for {
		// Use Put with Create revision check for atomicity
		_, err := l.kv.Create(ctx, lockKey, []byte(time.Now().Add(ttl).Format(time.RFC3339)))
		if err == nil {
			return true, nil
		}

		// Check if it's already locked and if it's expired
		entry, err := l.kv.Get(ctx, lockKey)
		if err == nil {
			expiresAt, parseErr := time.Parse(time.RFC3339, string(entry.Value()))
			if parseErr == nil && time.Now().After(expiresAt) {
				// Atomic update to take the expired lock
				_, err = l.kv.Update(ctx, lockKey, []byte(time.Now().Add(ttl).Format(time.RFC3339)), entry.Revision())
				if err == nil {
					return true, nil
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
	return l.kv.Delete(ctx, lockKey)
}

func (l *NATSLocker) Close() error {
	return nil
}
