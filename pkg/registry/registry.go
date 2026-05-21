package registry

import (
	"sync"
)

var (
	modules = make(map[string]Module)
	mu      sync.RWMutex
)

// Register adds a module to the global registry.
func Register(m Module) {
	mu.Lock()
	defer mu.Unlock()
	modules[m.ID()] = m
}

// List returns all registered modules.
func List() []Module {
	mu.RLock()
	defer mu.RUnlock()
	
	res := make([]Module, 0, len(modules))
	for _, m := range modules {
		res = append(res, m)
	}
	return res
}

// Get returns a module by its ID.
func Get(id string) (Module, bool) {
	mu.RLock()
	defer mu.RUnlock()
	
	m, ok := modules[id]
	return m, ok
}
