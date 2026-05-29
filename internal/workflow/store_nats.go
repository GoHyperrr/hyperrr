package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// NATSStore implements StateStore using NATS JetStream KeyValue.
type NATSStore struct {
	kv jetstream.KeyValue
}

type natsWorkflowState struct {
	Input []byte            `json:"input,omitempty"`
	Steps map[string]string `json:"steps,omitempty"`
}

// NewNATSStore creates a new NATSStore.
func NewNATSStore(ctx context.Context, nc *nats.Conn, bucketName string) (*NATSStore, error) {
	js, err := jetstream.New(nc)
	if err != nil {
		return nil, fmt.Errorf("failed to get jetstream context: %w", err)
	}

	// Create or update the bucket
	kv, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket:      bucketName,
		Description: "Workflow execution states",
		TTL:         48 * time.Hour,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create or update kv bucket: %w", err)
	}

	return &NATSStore{kv: kv}, nil
}

func (s *NATSStore) SaveState(ctx context.Context, execID string, stepID string, state string) error {
	key := fmt.Sprintf("wf.%s", execID)
	for {
		entry, err := s.kv.Get(ctx, key)
		if err != nil {
			if errors.Is(err, jetstream.ErrKeyNotFound) {
				wfState := natsWorkflowState{Steps: map[string]string{stepID: state}}
				data, _ := json.Marshal(wfState)
				_, err = s.kv.Create(ctx, key, data)
				if err == nil || errors.Is(err, jetstream.ErrKeyExists) {
					if err == nil {
						return nil
					}
					continue // Key was created between Get and Create, retry
				}
				return err
			}
			return err
		}

		var wfState natsWorkflowState
		if err := json.Unmarshal(entry.Value(), &wfState); err != nil {
			return err
		}
		if wfState.Steps == nil {
			wfState.Steps = make(map[string]string)
		}
		wfState.Steps[stepID] = state
		data, _ := json.Marshal(wfState)

		_, err = s.kv.Update(ctx, key, data, entry.Revision())
		if err == nil {
			return nil
		}
		// If revision mismatch, loop and retry
	}
}

func (s *NATSStore) GetState(ctx context.Context, execID string) (map[string]string, error) {
	key := fmt.Sprintf("wf.%s", execID)
	entry, err := s.kv.Get(ctx, key)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			return nil, fmt.Errorf("state not found for execution: %s", execID)
		}
		return nil, err
	}

	var wfState natsWorkflowState
	if err := json.Unmarshal(entry.Value(), &wfState); err != nil {
		return nil, err
	}
	return wfState.Steps, nil
}

func (s *NATSStore) SaveInput(ctx context.Context, execID string, input []byte) error {
	key := fmt.Sprintf("wf.%s", execID)
	for {
		entry, err := s.kv.Get(ctx, key)
		if err != nil {
			if errors.Is(err, jetstream.ErrKeyNotFound) {
				wfState := natsWorkflowState{Input: input, Steps: make(map[string]string)}
				data, _ := json.Marshal(wfState)
				_, err = s.kv.Create(ctx, key, data)
				if err == nil || errors.Is(err, jetstream.ErrKeyExists) {
					if err == nil {
						return nil
					}
					continue
				}
				return err
			}
			return err
		}

		var wfState natsWorkflowState
		if err := json.Unmarshal(entry.Value(), &wfState); err != nil {
			return err
		}
		wfState.Input = input
		data, _ := json.Marshal(wfState)

		_, err = s.kv.Update(ctx, key, data, entry.Revision())
		if err == nil {
			return nil
		}
	}
}

func (s *NATSStore) GetInput(ctx context.Context, execID string) ([]byte, error) {
	key := fmt.Sprintf("wf.%s", execID)
	entry, err := s.kv.Get(ctx, key)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			return nil, fmt.Errorf("input not found for execution: %s", execID)
		}
		return nil, err
	}

	var wfState natsWorkflowState
	if err := json.Unmarshal(entry.Value(), &wfState); err != nil {
		return nil, err
	}
	if wfState.Input == nil {
		return nil, fmt.Errorf("input not found for execution: %s", execID)
	}
	return wfState.Input, nil
}

func (s *NATSStore) SetTTL(ctx context.Context, execID string, ttl time.Duration) error {
	// NATS JetStream KV typically relies on the bucket-level TTL.
	// We can ignore per-key TTL for now as the bucket handles it.
	return nil
}
