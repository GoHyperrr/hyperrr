package locking

import (
	"github.com/GoHyperrr/mdk"
)

var (
	ErrLockAcquisitionTimeout = mdk.ErrLockAcquisitionTimeout
	ErrLockNotHeld           = mdk.ErrLockNotHeld
)

type LockerProvider = mdk.LockerProvider

func RegisterLocker(name string, provider LockerProvider) {
	mdk.RegisterLocker(name, provider)
}

func GetLocker(name string) (LockerProvider, bool) {
	return mdk.GetLocker(name)
}

var LockOwnerKey = mdk.LockOwnerKey

// Locker defines the interface for distributed locking.
type Locker = mdk.Locker
