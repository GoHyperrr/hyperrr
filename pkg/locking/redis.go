package locking

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const releaseScript = `
if redis.call("get", KEYS[1]) == ARGV[1] then
    return redis.call("del", KEYS[1])
else
    return 0
end
`

// RedisLocker implements Locker using Redis.
type RedisLocker struct {
	client  *redis.Client
	ownerID string
}

// NewRedisLocker creates a new RedisLocker.
func NewRedisLocker(client *redis.Client) *RedisLocker {
	return &RedisLocker{
		client:  client,
		ownerID: "owner_" + uuid.New().String(),
	}
}

func (l *RedisLocker) getOwner(ctx context.Context) string {
	if owner, ok := ctx.Value(LockOwnerKey).(string); ok && owner != "" {
		return owner
	}
	return l.ownerID
}

func (l *RedisLocker) Acquire(ctx context.Context, key string, ttl time.Duration, timeout time.Duration) (bool, error) {
	start := time.Now()
	lockKey := "lock:" + key
	owner := l.getOwner(ctx)

	for {
		// SetNX (Set if Not eXists) with TTL and owner
		ok, err := l.client.SetNX(ctx, lockKey, owner, ttl).Result()
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
	owner := l.getOwner(ctx)
	return l.client.Eval(ctx, releaseScript, []string{lockKey}, owner).Err()
}

func (l *RedisLocker) Close() error {
	return nil
}
