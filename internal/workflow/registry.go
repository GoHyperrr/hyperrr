package workflow

import (
	"fmt"
	"sync"
)

// Registry manages the registration of workflow definitions.
type Registry struct {
	mu        sync.RWMutex
	workflows map[string]*Workflow
}

// NewRegistry creates a new Registry.
func NewRegistry() *Registry {
	return &Registry{
		workflows: make(map[string]*Workflow),
	}
}

// Register adds a workflow definition to the registry.
func (r *Registry) Register(wf *Workflow) error {
	if wf.Name == "" {
		return fmt.Errorf("workflow name cannot be empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	
	r.workflows[wf.Name] = wf
	return nil
}

// Get retrieves a workflow definition by name.
func (r *Registry) Get(name string) (*Workflow, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	wf, ok := r.workflows[name]
	if !ok {
		return nil, fmt.Errorf("workflow not found: %s", name)
	}
	return wf, nil
}
