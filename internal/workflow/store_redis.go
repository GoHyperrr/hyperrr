package workflow

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisStore implements StateStore using Redis.
type RedisStore struct {
	client *redis.Client
}

// NewRedisStore creates a new RedisStore.
func NewRedisStore(client *redis.Client) *RedisStore {
	return &RedisStore{client: client}
}

func (s *RedisStore) SaveState(ctx context.Context, execID string, stepID string, state string) error {
	key := fmt.Sprintf("wf:%s:state", execID)
	return s.client.HSet(ctx, key, stepID, state).Err()
}

func (s *RedisStore) GetState(ctx context.Context, execID string) (map[string]string, error) {
	key := fmt.Sprintf("wf:%s:state", execID)
	res, err := s.client.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	if len(res) == 0 {
		return nil, fmt.Errorf("state not found for execution: %s", execID)
	}
	return res, nil
}

func (s *RedisStore) SaveInput(ctx context.Context, execID string, input []byte) error {
	key := fmt.Sprintf("wf:%s:input", execID)
	return s.client.Set(ctx, key, input, 0).Err()
}

func (s *RedisStore) GetInput(ctx context.Context, execID string) ([]byte, error) {
	key := fmt.Sprintf("wf:%s:input", execID)
	res, err := s.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("input not found for execution: %s", execID)
		}
		return nil, err
	}
	return res, nil
}

func (s *RedisStore) SetTTL(ctx context.Context, execID string, ttl time.Duration) error {
	stateKey := fmt.Sprintf("wf:%s:state", execID)
	inputKey := fmt.Sprintf("wf:%s:input", execID)
	
	if err := s.client.Expire(ctx, stateKey, ttl).Err(); err != nil {
		return err
	}
	return s.client.Expire(ctx, inputKey, ttl).Err()
}
