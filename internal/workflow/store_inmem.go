package workflow

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// InMemStore is an in-memory implementation of StateStore for local testing.
type InMemStore struct {
	mu     sync.RWMutex
	states map[string]map[string]string // execID -> stepID -> state
	inputs map[string][]byte            // execID -> input payload
	ttls   map[string]time.Time         // execID -> expiration time
}

// NewInMemStore creates a new InMemStore.
func NewInMemStore() *InMemStore {
	store := &InMemStore{
		states: make(map[string]map[string]string),
		inputs: make(map[string][]byte),
		ttls:   make(map[string]time.Time),
	}
	// Start a simple cleanup routine
	go store.cleanupRoutine()
	return store
}

func (s *InMemStore) SaveState(ctx context.Context, execID string, stepID string, state string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.states[execID] == nil {
		s.states[execID] = make(map[string]string)
	}
	s.states[execID][stepID] = state
	return nil
}

func (s *InMemStore) GetState(ctx context.Context, execID string) (map[string]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Check TTL
	if exp, ok := s.ttls[execID]; ok && time.Now().After(exp) {
		return nil, fmt.Errorf("state expired for execution: %s", execID)
	}

	states, ok := s.states[execID]
	if !ok {
		return nil, fmt.Errorf("state not found for execution: %s", execID)
	}

	// Return a copy to prevent race conditions
	res := make(map[string]string, len(states))
	for k, v := range states {
		res[k] = v
	}
	return res, nil
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

	// Check TTL
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

func (s *InMemStore) cleanupRoutine() {
	ticker := time.NewTicker(1 * time.Minute)
	for range ticker.C {
		s.mu.Lock()
		now := time.Now()
		for execID, exp := range s.ttls {
			if now.After(exp) {
				delete(s.ttls, execID)
				delete(s.states, execID)
				delete(s.inputs, execID)
			}
		}
		s.mu.Unlock()
	}
}
