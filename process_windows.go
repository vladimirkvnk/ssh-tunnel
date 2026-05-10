//go:build windows

package main

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

var (
	modkernel32             = syscall.NewLazyDLL("kernel32.dll")
	procOpenProcess         = modkernel32.NewProc("OpenProcess")
	procWaitForSingleObject = modkernel32.NewProc("WaitForSingleObject")
)

const (
	processSynchronize = 0x00100000
	waitObject0        = 0x00000000
	waitTimeout        = 0x00000102
	waitFailed         = 0xFFFFFFFF
	errAccessDenied    = syscall.Errno(5)
	errInvalidArgument = syscall.Errno(87)
)

// terminateProcess kills the process on Windows.
// Windows has no equivalent of SIGTERM for external processes,
// so Process.Kill (TerminateProcess) is the only reliable option.
func terminateProcess(proc *os.Process) error {
	return proc.Kill()
}

// isProcessAlive checks if a process with the given PID is still running.
// Uses OpenProcess + WaitForSingleObject on Windows.
func isProcessAlive(pid int) (bool, error) {
	handle, _, openErr := procOpenProcess.Call(
		uintptr(processSynchronize),
		0,
		uintptr(pid),
	)
	if handle == 0 {
		if errors.Is(openErr, errInvalidArgument) {
			return false, nil
		}
		if errors.Is(openErr, errAccessDenied) {
			return true, nil
		}
		return false, fmt.Errorf("OpenProcess failed: %w", openErr)
	}
	defer func() { _ = syscall.CloseHandle(syscall.Handle(handle)) }()

	result, _, waitErr := procWaitForSingleObject.Call(handle, 0)
	switch result {
	case waitTimeout:
		return true, nil
	case waitObject0:
		return false, nil
	case waitFailed:
		return false, fmt.Errorf("WaitForSingleObject failed: %w", waitErr)
	default:
		return false, fmt.Errorf("WaitForSingleObject returned unexpected status: %#x", result)
	}
}
