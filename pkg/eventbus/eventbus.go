package eventbus

import (
	"github.com/GoHyperrr/mdk"
)

// Re-export/alias mdk event types
type Event = mdk.Event
type EventHandler = mdk.EventHandler
type EventBus interface {
	mdk.EventBus
	Close() error
}
