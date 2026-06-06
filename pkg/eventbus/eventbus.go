package eventbus

import (
	"context"
	"sync"

	"github.com/GoHyperrr/mdk"
)

// Re-export/alias mdk event types
type Event = mdk.Event
type EventHandler = mdk.EventHandler
type EventBus interface {
	mdk.EventBus
	Close() error
}

type ContextTiedBus interface {
	EventBus
	SetContext(ctx context.Context)
}

type BusProvider func(url string) (EventBus, error)

var (
	providersMu sync.RWMutex
	providers   = make(map[string]BusProvider)
)

func RegisterProvider(name string, provider BusProvider) {
	providersMu.Lock()
	defer providersMu.Unlock()
	providers[name] = provider
}

func GetProvider(name string) (BusProvider, bool) {
	providersMu.RLock()
	defer providersMu.RUnlock()
	p, ok := providers[name]
	return p, ok
}
