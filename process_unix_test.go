//go:build !windows

package main

import (
	"os"
	"os/exec"
	"syscall"
	"testing"
)

func TestShutdownSignals(t *testing.T) {
	signals := shutdownSignals()
	if len(signals) != 2 {
		t.Fatalf("expected 2 shutdown signals, got %d", len(signals))
	}

	expected := map[os.Signal]bool{
		syscall.SIGINT:  true,
		syscall.SIGTERM: true,
	}

	for _, sig := range signals {
		if !expected[sig] {
			t.Errorf("unexpected signal: %v", sig)
		}
	}
}

func TestIsProcessAlive_Running(t *testing.T) {
	cmd := exec.Command("sleep", "10")
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
	cmd := exec.Command("true")
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
	cmd := exec.Command("sleep", "10")
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
