package locking

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisLocker implements Locker using Redis.
type RedisLocker struct {
	client *redis.Client
}

// NewRedisLocker creates a new RedisLocker.
func NewRedisLocker(client *redis.Client) *RedisLocker {
	return &RedisLocker{client: client}
}

func (l *RedisLocker) Acquire(ctx context.Context, key string, ttl time.Duration, timeout time.Duration) (bool, error) {
	start := time.Now()
	lockKey := "lock:" + key

	for {
		// SetNX (Set if Not eXists) with TTL
		ok, err := l.client.SetNX(ctx, lockKey, "locked", ttl).Result()
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
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

func (l *RedisLocker) Release(ctx context.Context, key string) error {
	lockKey := "lock:" + key
	return l.client.Del(ctx, lockKey).Err()
}

func (l *RedisLocker) Close() error {
	return nil
}
