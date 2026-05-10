//go:build windows

package main

import (
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

const (
	helperProcessEnv   = "SSH_TUNNEL_TEST_HELPER_PROCESS"
	helperProcessExit  = "exit"
	helperProcessSleep = "sleep"
)

func TestShutdownSignals(t *testing.T) {
	signals := shutdownSignals()
	if len(signals) != 2 {
		t.Fatalf("expected 2 shutdown signals, got %d", len(signals))
	}

	expected := map[os.Signal]bool{
		os.Interrupt:    true,
		syscall.SIGTERM: true,
	}
	for _, sig := range signals {
		if !expected[sig] {
			t.Errorf("unexpected signal: %v", sig)
		}
	}
}

func TestIsProcessAlive_Running(t *testing.T) {
	cmd := helperProcessCommand(t, helperProcessSleep)
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start process: %v", err)
	}
	defer func() { _ = cmd.Process.Kill() }()

	alive, err := isProcessAlive(cmd.Process.Pid)
	if err != nil {
		t.Fatalf("isProcessAlive: %v", err)
	}
	if !alive {
		t.Error("expected process to be alive")
	}

	if err := cmd.Process.Kill(); err != nil {
		t.Logf("kill: %v", err)
	}

	if err := cmd.Wait(); err != nil {
		t.Logf("wait: %v", err)
	}
}

func TestIsProcessAlive_Dead(t *testing.T) {
	cmd := helperProcessCommand(t, helperProcessExit)
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to run process: %v", err)
	}

	alive, err := isProcessAlive(cmd.Process.Pid)
	if err != nil {
		t.Fatalf("isProcessAlive: %v", err)
	}
	if alive {
		t.Error("expected process to be dead")
	}
}

func TestTerminateProcess(t *testing.T) {
	cmd := helperProcessCommand(t, helperProcessSleep)
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start process: %v", err)
	}

	if err := terminateProcess(cmd.Process); err != nil {
		t.Fatalf("terminateProcess failed: %v", err)
	}

	if err := cmd.Wait(); err != nil {
		t.Logf("wait: %v", err)
	}

	alive, err := isProcessAlive(cmd.Process.Pid)
	if err != nil {
		t.Fatalf("isProcessAlive: %v", err)
	}
	if alive {
		t.Error("expected process to be dead after termination")
	}
}

// helperProcessCommand starts this test binary in child-process mode.
func helperProcessCommand(t *testing.T, mode string) *exec.Cmd {
	t.Helper()
	if mode != helperProcessSleep && mode != helperProcessExit {
		t.Fatalf("unsupported helper process mode: %s", mode)
	}

	//nolint:gosec // G204: tests intentionally start this test binary with fixed helper modes.
	cmd := exec.Command(os.Args[0], "-test.run=TestWindowsHelperProcess", "--", mode)
	cmd.Env = append(os.Environ(), helperProcessEnv+"="+mode)
	return cmd
}

// TestWindowsHelperProcess runs as a child process for process-lifecycle tests.
func TestWindowsHelperProcess(t *testing.T) {
	switch os.Getenv(helperProcessEnv) {
	case "":
		return
	case helperProcessSleep:
		time.Sleep(30 * time.Second)
	case helperProcessExit:
		return
	default:
		os.Exit(2)
	}
}
