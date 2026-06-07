package workflow

import (
	"github.com/GoHyperrr/mdk"
)

type StoreProvider = mdk.StoreProvider

func RegisterStore(name string, provider StoreProvider) {
	mdk.RegisterStateStore(name, provider)
}

func GetStore(name string) (StoreProvider, bool) {
	return mdk.GetStateStore(name)
}

// StateStore defines the interface for checkpointing workflow states.
type StateStore = mdk.StateStore
