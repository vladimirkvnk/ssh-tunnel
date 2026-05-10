//go:build !windows

package main

import (
	"errors"
	"os"
	"syscall"
)

// terminateProcess sends SIGTERM to the process, allowing it to shut down gracefully.
func terminateProcess(proc *os.Process) error {
	return proc.Signal(syscall.SIGTERM)
}

// isProcessAlive checks if a process with the given PID is still running.
// Uses signal 0 (existence check) which is supported on Unix systems.
func isProcessAlive(pid int) (bool, error) {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false, err
	}
	err = proc.Signal(syscall.Signal(0))
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrProcessDone) || errors.Is(err, syscall.ESRCH) {
		return false, nil
	}
	if errors.Is(err, syscall.EPERM) {
		return true, nil
	}
	return false, err
}
