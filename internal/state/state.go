package state

import "sync/atomic"

var shuttingDown atomic.Bool

func IsShuttingDown() bool {
	return shuttingDown.Load()
}

func StartShutdown() {
	shuttingDown.Store(true)
}
