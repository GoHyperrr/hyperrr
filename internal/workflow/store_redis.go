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

func (s *RedisStore) InitializeExecution(ctx context.Context, execID string, input []byte) error {
	stateKey := fmt.Sprintf("wf:%s:state", execID)
	inputKey := fmt.Sprintf("wf:%s:input", execID)

	pipe := s.client.Pipeline()
	// Set initial status in state hash
	pipe.HSet(ctx, stateKey, "_status", "STARTED")
	// Save the input
	pipe.Set(ctx, inputKey, input, 0)

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
	stateKey := fmt.Sprintf("wf:%s:state", execID)
	inputKey := fmt.Sprintf("wf:%s:input", execID)
	outputsKey := fmt.Sprintf("wf:%s:outputs", execID)
	
	if err := s.client.Expire(ctx, stateKey, ttl).Err(); err != nil {
		return err
	}
	if err := s.client.Expire(ctx, outputsKey, ttl).Err(); err != nil {
		return err
	}
	return s.client.Expire(ctx, inputKey, ttl).Err()
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
