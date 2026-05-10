//go:build !windows

package main

import (
	"os"
	"syscall"
)

// shutdownSignals returns the OS signals that should trigger a graceful shutdown.
func shutdownSignals() []os.Signal {
	return []os.Signal{syscall.SIGINT, syscall.SIGTERM}
}
