package eventbus

import (
	"context"

	"github.com/GoHyperrr/mdk"
)

// Re-export/alias mdk event types
type Event = mdk.Event
type EventHandler = mdk.EventHandler
type EventBus = mdk.EventBusCloser

type ContextTiedBus interface {
	EventBus
	SetContext(ctx context.Context)
}

type BusProvider = mdk.BusProvider

func RegisterProvider(name string, provider BusProvider) {
	mdk.RegisterEventBusProvider(name, provider)
}

func GetProvider(name string) (BusProvider, bool) {
	return mdk.GetEventBusProvider(name)
}
