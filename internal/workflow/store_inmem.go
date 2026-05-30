package workflow

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// InMemStore is an in-memory implementation of StateStore for local testing.
type InMemStore struct {
	mu            sync.RWMutex
	states        map[string]map[string]string // execID -> stepID -> state
	overallStates map[string]string            // execID -> overall state (RUNNING, COMPLETED, etc)
	inputs        map[string][]byte            // execID -> input payload
	outputs       map[string]map[string][]byte // execID -> stepID -> output
	ttls          map[string]time.Time         // execID -> expiration time
	emitted       map[string]map[string]bool   // execID -> eventType -> true
}

// NewInMemStore creates a new InMemStore.
func NewInMemStore() *InMemStore {
	store := &InMemStore{
		states:        make(map[string]map[string]string),
		overallStates: make(map[string]string),
		inputs:        make(map[string][]byte),
		outputs:       make(map[string]map[string][]byte),
		ttls:          make(map[string]time.Time),
		emitted:       make(map[string]map[string]bool),
	}
	// Start a simple cleanup routine
	go store.cleanupRoutine()
	return store
}

func (s *InMemStore) SaveState(ctx context.Context, execID string, stepID string, state string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Special case: if stepID is empty, it's the overall workflow state
	if stepID == "" {
		s.overallStates[execID] = state
		return nil
	}

	if s.states[execID] == nil {
		s.states[execID] = make(map[string]string)
	}
	s.states[execID][stepID] = state
	return nil
}

func (s *InMemStore) GetState(ctx context.Context, execID string) (map[string]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if exp, ok := s.ttls[execID]; ok && time.Now().After(exp) {
		return nil, fmt.Errorf("state expired for execution: %s", execID)
	}

	states, ok := s.states[execID]
	if !ok {
		return nil, fmt.Errorf("state not found for execution: %s", execID)
	}

	res := make(map[string]string, len(states))
	for k, v := range states {
		res[k] = v
	}
	return res, nil
}

func (s *InMemStore) InitializeExecution(ctx context.Context, execID string, input []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.inputs[execID] = input
	s.states[execID] = make(map[string]string)
	s.overallStates[execID] = StateRunning
	return nil
}

func (s *InMemStore) SaveInput(ctx context.Context, execID string, input []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.inputs[execID] = input
	return nil
}

func (s *InMemStore) GetInput(ctx context.Context, execID string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if exp, ok := s.ttls[execID]; ok && time.Now().After(exp) {
		return nil, fmt.Errorf("input expired for execution: %s", execID)
	}

	input, ok := s.inputs[execID]
	if !ok {
		return nil, fmt.Errorf("input not found for execution: %s", execID)
	}
	
	res := make([]byte, len(input))
	copy(res, input)
	return res, nil
}

func (s *InMemStore) SetTTL(ctx context.Context, execID string, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ttls[execID] = time.Now().Add(ttl)
	return nil
}

func (s *InMemStore) SaveStepOutput(ctx context.Context, execID string, stepID string, output []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.outputs[execID] == nil {
		s.outputs[execID] = make(map[string][]byte)
	}
	s.outputs[execID][stepID] = output
	return nil
}

func (s *InMemStore) GetStepOutput(ctx context.Context, execID string, stepID string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if outs, ok := s.outputs[execID]; ok {
		if out, ok := outs[stepID]; ok {
			res := make([]byte, len(out))
			copy(res, out)
			return res, nil
		}
	}
	return nil, fmt.Errorf("output not found for step: %s", stepID)
}

func (s *InMemStore) ListExecutions(ctx context.Context, state string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var ids []string
	for id, s := range s.overallStates {
		if s == state {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func (s *InMemStore) RecordEventEmitted(ctx context.Context, execID string, eventType string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.emitted[execID] == nil {
		s.emitted[execID] = make(map[string]bool)
	}
	s.emitted[execID][eventType] = true
	return nil
}

func (s *InMemStore) IsEventEmitted(ctx context.Context, execID string, eventType string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if events, ok := s.emitted[execID]; ok {
		return events[eventType], nil
	}
	return false, nil
}

func (s *InMemStore) cleanupRoutine() {
	ticker := time.NewTicker(1 * time.Minute)
	for range ticker.C {
		s.mu.Lock()
		now := time.Now()
		for execID, exp := range s.ttls {
			if now.After(exp) {
				delete(s.ttls, execID)
				delete(s.states, execID)
				delete(s.overallStates, execID)
				delete(s.inputs, execID)
				delete(s.outputs, execID)
				delete(s.emitted, execID)
			}
		}
		s.mu.Unlock()
	}
}
