//go:build windows

package main

import (
	"os"
	"syscall"
)

// shutdownSignals returns the OS signals that should trigger a graceful shutdown.
// Windows maps Ctrl+C/Ctrl+Break to os.Interrupt.
// Close, logoff, and shutdown events are delivered as SIGTERM.
func shutdownSignals() []os.Signal {
	return []os.Signal{os.Interrupt, syscall.SIGTERM}
}
