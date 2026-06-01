package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// NATSStore implements StateStore using NATS JetStream KeyValue.
type NATSStore struct {
	kv jetstream.KeyValue
}

type natsWorkflowState struct {
	Input         []byte            `json:"input,omitempty"`
	Steps         map[string]string `json:"steps,omitempty"`
	Outputs       map[string][]byte `json:"outputs,omitempty"`
	OverallState  string            `json:"overall_state,omitempty"`
	EmittedEvents map[string]bool   `json:"emitted_events,omitempty"`
}

// NewNATSStore creates a new NATSStore.
func NewNATSStore(ctx context.Context, nc *nats.Conn, bucketName string) (*NATSStore, error) {
	if nc == nil {
		return nil, fmt.Errorf("nats connection is nil")
	}
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
				wfState := natsWorkflowState{Steps: make(map[string]string)}
				if stepID == "" {
					wfState.OverallState = state
				} else {
					wfState.Steps[stepID] = state
				}
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
		
		if stepID == "" {
			wfState.OverallState = state
		} else {
			if wfState.Steps == nil {
				wfState.Steps = make(map[string]string)
			}
			wfState.Steps[stepID] = state
		}
		
		data, _ := json.Marshal(wfState)
		_, err = s.kv.Update(ctx, key, data, entry.Revision())
		if err == nil {
			return nil
		}
		if errors.Is(err, jetstream.ErrKeyExists) || strings.Contains(err.Error(), "revision") || strings.Contains(err.Error(), "mismatch") {
			continue
		}
		return err
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

func (s *NATSStore) InitializeExecution(ctx context.Context, execID string, input []byte) error {
	key := fmt.Sprintf("wf.%s", execID)
	wfState := natsWorkflowState{
		Input:        input,
		Steps:        make(map[string]string),
		OverallState: StateRunning,
	}
	data, _ := json.Marshal(wfState)
	_, err := s.kv.Create(ctx, key, data)
	return err
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
		if errors.Is(err, jetstream.ErrKeyExists) || strings.Contains(err.Error(), "revision") || strings.Contains(err.Error(), "mismatch") {
			continue
		}
		return err
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
	return nil
}

func (s *NATSStore) SaveStepOutput(ctx context.Context, execID string, stepID string, output []byte) error {
	key := fmt.Sprintf("wf.%s", execID)
	for {
		entry, err := s.kv.Get(ctx, key)
		if err != nil {
			return err
		}

		var wfState natsWorkflowState
		if err := json.Unmarshal(entry.Value(), &wfState); err != nil {
			return err
		}
		if wfState.Outputs == nil {
			wfState.Outputs = make(map[string][]byte)
		}
		wfState.Outputs[stepID] = output
		
		data, _ := json.Marshal(wfState)
		_, err = s.kv.Update(ctx, key, data, entry.Revision())
		if err == nil {
			return nil
		}
		if errors.Is(err, jetstream.ErrKeyExists) || strings.Contains(err.Error(), "revision") || strings.Contains(err.Error(), "mismatch") {
			continue
		}
		return err
	}
}

func (s *NATSStore) GetStepOutput(ctx context.Context, execID string, stepID string) ([]byte, error) {
	key := fmt.Sprintf("wf.%s", execID)
	entry, err := s.kv.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	var wfState natsWorkflowState
	if err := json.Unmarshal(entry.Value(), &wfState); err != nil {
		return nil, err
	}
	if wfState.Outputs == nil {
		return nil, fmt.Errorf("step output not found for execution: %s", execID)
	}
	output, ok := wfState.Outputs[stepID]
	if !ok {
		return nil, fmt.Errorf("step output not found for step: %s in execution: %s", stepID, execID)
	}
	return output, nil
}

func (s *NATSStore) ListExecutions(ctx context.Context, state string) ([]string, error) {
	keys, err := s.kv.Keys(ctx)
	if err != nil {
		if errors.Is(err, jetstream.ErrNoKeysFound) {
			return nil, nil
		}
		return nil, err
	}

	var ids []string
	for _, key := range keys {
		if !strings.HasPrefix(key, "wf.") {
			continue
		}
		
		entry, err := s.kv.Get(ctx, key)
		if err != nil {
			continue
		}
		
		var wfState natsWorkflowState
		if err := json.Unmarshal(entry.Value(), &wfState); err == nil {
			if wfState.OverallState == state {
				ids = append(ids, strings.TrimPrefix(key, "wf."))
			}
		}
	}
	return ids, nil
}

func (s *NATSStore) RecordEventEmitted(ctx context.Context, execID string, eventType string) error {
	key := fmt.Sprintf("wf.%s", execID)
	for {
		entry, err := s.kv.Get(ctx, key)
		if err != nil {
			return err
		}

		var wfState natsWorkflowState
		if err := json.Unmarshal(entry.Value(), &wfState); err != nil {
			return err
		}
		if wfState.EmittedEvents == nil {
			wfState.EmittedEvents = make(map[string]bool)
		}
		wfState.EmittedEvents[eventType] = true
		
		data, _ := json.Marshal(wfState)
		_, err = s.kv.Update(ctx, key, data, entry.Revision())
		if err == nil {
			return nil
		}
		if errors.Is(err, jetstream.ErrKeyExists) || strings.Contains(err.Error(), "revision") || strings.Contains(err.Error(), "mismatch") {
			continue
		}
		return err
	}
}

func (s *NATSStore) IsEventEmitted(ctx context.Context, execID string, eventType string) (bool, error) {
	key := fmt.Sprintf("wf.%s", execID)
	entry, err := s.kv.Get(ctx, key)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			return false, nil
		}
		return false, err
	}

	var wfState natsWorkflowState
	if err := json.Unmarshal(entry.Value(), &wfState); err != nil {
		return false, err
	}
	if wfState.EmittedEvents == nil {
		return false, nil
	}
	return wfState.EmittedEvents[eventType], nil
}
