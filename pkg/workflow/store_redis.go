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
	if stepID == "" {
		key := fmt.Sprintf("wf:%s:overall", execID)
		return s.client.Set(ctx, key, state, 0).Err()
	}
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

func (s *RedisStore) InitializeExecution(ctx context.Context, execID string, input []byte) error {
	pipe := s.client.Pipeline()
	pipe.Set(ctx, fmt.Sprintf("wf:%s:input", execID), input, 0)
	pipe.Set(ctx, fmt.Sprintf("wf:%s:overall", execID), StateRunning, 0)
	_, err := pipe.Exec(ctx)
	return err
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
	keys := []string{
		fmt.Sprintf("wf:%s:state", execID),
		fmt.Sprintf("wf:%s:input", execID),
		fmt.Sprintf("wf:%s:outputs", execID),
		fmt.Sprintf("wf:%s:overall", execID),
		fmt.Sprintf("wf:%s:emitted", execID),
	}
	
	pipe := s.client.Pipeline()
	for _, k := range keys {
		pipe.Expire(ctx, k, ttl)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (s *RedisStore) SaveStepOutput(ctx context.Context, execID string, stepID string, output []byte) error {
	key := fmt.Sprintf("wf:%s:outputs", execID)
	return s.client.HSet(ctx, key, stepID, output).Err()
}

func (s *RedisStore) GetStepOutput(ctx context.Context, execID string, stepID string) ([]byte, error) {
	key := fmt.Sprintf("wf:%s:outputs", execID)
	res, err := s.client.HGet(ctx, key, stepID).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("step output not found for step: %s in execution: %s", stepID, execID)
		}
		return nil, err
	}
	return res, nil
}

func (s *RedisStore) ListExecutions(ctx context.Context, state string) ([]string, error) {
	// Scan for keys with pattern wf:*:overall and check value
	var ids []string
	var cursor uint64
	for {
		keys, nextCursor, err := s.client.Scan(ctx, cursor, "wf:*:overall", 100).Result()
		if err != nil {
			return nil, err
		}
		
		for _, k := range keys {
			val, err := s.client.Get(ctx, k).Result()
			if err == nil && val == state {
				id := k[3 : len(k)-8] // Extract ID from wf:{id}:overall
				ids = append(ids, id)
			}
		}
		
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	return ids, nil
}

func (s *RedisStore) RecordEventEmitted(ctx context.Context, execID string, eventType string) error {
	key := fmt.Sprintf("wf:%s:emitted", execID)
	return s.client.HSet(ctx, key, eventType, "true").Err()
}

func (s *RedisStore) IsEventEmitted(ctx context.Context, execID string, eventType string) (bool, error) {
	key := fmt.Sprintf("wf:%s:emitted", execID)
	exists, err := s.client.HExists(ctx, key, eventType).Result()
	return exists, err
}
