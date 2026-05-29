package registry

import (
	"fmt"
	"sync"
)

// Registry manages the set of available hyperrr modules.
type Registry struct {
	mu      sync.RWMutex
	modules map[string]Module
}

// NewRegistry creates a new, independent module registry.
func NewRegistry() *Registry {
	return &Registry{
		modules: make(map[string]Module),
	}
}

var globalRegistry = NewRegistry()

// Register adds a module to the global registry.
func Register(m Module) {
	globalRegistry.Register(m)
}

// List returns all registered modules from the global registry.
func List() []Module {
	return globalRegistry.List()
}

// Get returns a module by its ID from the global registry.
func Get(id string) (Module, bool) {
	return globalRegistry.Get(id)
}

// Register adds a module to the registry. It logs a warning if the ID is already taken.
func (r *Registry) Register(m Module) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.modules[m.ID()]; exists {
		fmt.Printf("WARNING: overwriting module registration for ID: %s\n", m.ID())
	}
	r.modules[m.ID()] = m
}

// List returns all registered modules.
func (r *Registry) List() []Module {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	res := make([]Module, 0, len(r.modules))
	for _, m := range r.modules {
		res = append(res, m)
	}
	return res
}

// Get returns a module by its ID.
func (r *Registry) Get(id string) (Module, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	m, ok := r.modules[id]
	return m, ok
}
