package registry

import (
	"fmt"
	"strings"
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
	normalizedID := NormalizeModuleID(m.ID())
	if _, exists := r.modules[normalizedID]; exists {
		fmt.Printf("WARNING: overwriting module registration for ID: %s\n", normalizedID)
	}
	r.modules[normalizedID] = m
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
	
	m, ok := r.modules[NormalizeModuleID(id)]
	return m, ok
}

// ModuleFactory is a constructor function that instantiates a module with options.
// A module factory can also be auto-implemented or registered in package init().
type ModuleFactory func(options map[string]any) (Module, error)

var (
	factoryMu sync.RWMutex
	factories = make(map[string]ModuleFactory)
)

// NormalizeModuleID standardizes module IDs by removing common repo prefixes and replacing slashes with dots.
// E.g., "github.com/GoHyperrr/commerce/hotel" -> "commerce.hotel"
func NormalizeModuleID(id string) string {
	id = strings.TrimPrefix(id, "github.com/GoHyperrr/hyperrr/")
	id = strings.TrimPrefix(id, "github.com/GoHyperrr/")
	id = strings.ReplaceAll(id, "/", ".")
	return id
}

// RegisterFactory registers a module constructor in the global factory registry.
func RegisterFactory(id string, factory ModuleFactory) {
	factoryMu.Lock()
	defer factoryMu.Unlock()
	factories[NormalizeModuleID(id)] = factory
}

// GetFactory retrieves a registered module constructor by its ID.
func GetFactory(id string) (ModuleFactory, bool) {
	factoryMu.RLock()
	defer factoryMu.RUnlock()
	f, ok := factories[NormalizeModuleID(id)]
	return f, ok
}
