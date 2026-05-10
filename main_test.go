package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
)

// --- resolveAddr ---

func TestResolveAddr_IPv4(t *testing.T) {
	app := &Application{}
	got, err := app.resolveAddr(context.Background(), "1.2.3.4:443")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "1.2.3.4:443" {
		t.Errorf("got %q, want %q", got, "1.2.3.4:443")
	}
}

func TestResolveAddr_IPv6(t *testing.T) {
	app := &Application{}
	got, err := app.resolveAddr(context.Background(), "[::1]:8080")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "[::1]:8080" {
		t.Errorf("got %q, want %q", got, "[::1]:8080")
	}
}

func TestResolveAddr_NotHostPort(t *testing.T) {
	app := &Application{}
	got, err := app.resolveAddr(context.Background(), "just-a-string")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "just-a-string" {
		t.Errorf("got %q, want %q", got, "just-a-string")
	}
}

func TestResolveAddr_DNSResolves(t *testing.T) {
	app := &Application{}
	got, err := app.resolveAddr(context.Background(), "localhost:8080")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// localhost should resolve to 127.0.0.1:8080 or [::1]:8080
	if got != "127.0.0.1:8080" && got != "[::1]:8080" {
		t.Errorf("got %q, want localhost resolved to 127.0.0.1 or ::1", got)
	}
}

func TestResolveAddr_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	app := &Application{}
	_, err := app.resolveAddr(ctx, "nonexistent-host.test:80")
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}

// --- isProcessRunning ---

func TestIsProcessRunning_NilCmd(t *testing.T) {
	app := &Application{}
	if app.isProcessRunning(nil) {
		t.Error("expected false for nil cmd")
	}
}

func TestIsProcessRunning_NilProcess(t *testing.T) {
	app := &Application{}
	cmd := &exec.Cmd{}
	if app.isProcessRunning(cmd) {
		t.Error("expected false for cmd with nil Process")
	}
}

func TestIsProcessRunning_Finished(t *testing.T) {
	cmd := exec.Command("true")
	_ = cmd.Run()

	app := &Application{}
	if app.isProcessRunning(cmd) {
		t.Error("expected false for finished process")
	}
}

func TestIsProcessRunning_Active(t *testing.T) {
	cmd := exec.Command("sleep", "10")
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start: %v", err)
	}
	defer func() { _ = cmd.Process.Kill() }()

	app := &Application{}
	if !app.isProcessRunning(cmd) {
		t.Error("expected true for running process")
	}
	_ = cmd.Process.Kill()
	_ = cmd.Wait()
}

// --- createPIDFile + cleanup ---

func newTestApp(t *testing.T) *Application {
	t.Helper()

	dir := t.TempDir()
	cfg := validConfig()
	cfg.PIDFile = filepath.Join(dir, "ssh-tunnel.pid")
	cfg.LogFile = filepath.Join(dir, "ssh-tunnel.log")
	if err := cfg.validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}

	return &Application{
		config:       &cfg,
		shutdownChan: make(chan struct{}),
	}
}

func TestCreatePIDFile_New(t *testing.T) {
	app := newTestApp(t)

	if err := app.createPIDFile(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	pidFile := filepath.Clean(app.config.getPortSpecificPIDFile())
	data, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatalf("failed to read PID file: %v", err)
	}

	pid := string(data)
	if pid != strconv.Itoa(os.Getpid()) {
		t.Errorf("PID file contains %q, want %d", pid, os.Getpid())
	}
}

func TestCreatePIDFile_DuplicateInstance(t *testing.T) {
	app := newTestApp(t)

	// First creation succeeds
	if err := app.createPIDFile(); err != nil {
		t.Fatalf("first createPIDFile: %v", err)
	}

	// Second creation must fail — current process is alive
	if err := app.createPIDFile(); err == nil {
		t.Error("expected error for duplicate instance")
	}
}

func TestCreatePIDFile_StaleFile(t *testing.T) {
	app := newTestApp(t)
	pidFile := app.config.getPortSpecificPIDFile()

	// Write a PID that definitely doesn't exist
	nonexistentPid := 999999999
	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(nonexistentPid)), 0600); err != nil {
		t.Fatalf("failed to write stale PID file: %v", err)
	}

	// Should remove stale file and create new one
	if err := app.createPIDFile(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Clean(pidFile))
	if err != nil {
		t.Fatalf("failed to read PID file: %v", err)
	}
	if string(data) != strconv.Itoa(os.Getpid()) {
		t.Errorf("PID file contains %q, want %d", string(data), os.Getpid())
	}
}

func TestCreatePIDFile_CorruptedContent(t *testing.T) {
	app := newTestApp(t)
	pidFile := app.config.getPortSpecificPIDFile()

	if err := os.WriteFile(pidFile, []byte("not-a-number"), 0600); err != nil {
		t.Fatalf("failed to write corrupted PID file: %v", err)
	}

	if err := app.createPIDFile(); err == nil {
		t.Error("expected error for corrupted PID file")
	}
}

func TestCreatePIDFile_ProcessCheckError(t *testing.T) {
	app := newTestApp(t)
	pidFile := app.config.getPortSpecificPIDFile()

	const existingPID = 12345
	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(existingPID)), 0600); err != nil {
		t.Fatalf("failed to write PID file: %v", err)
	}

	wantErr := errors.New("process check failed")
	originalCheckProcessAlive := checkProcessAlive
	checkProcessAlive = func(pid int) (bool, error) {
		if pid != existingPID {
			t.Fatalf("checked PID %d, want %d", pid, existingPID)
		}
		return false, wantErr
	}
	t.Cleanup(func() {
		checkProcessAlive = originalCheckProcessAlive
	})

	err := app.createPIDFile()
	if !errors.Is(err, wantErr) {
		t.Fatalf("createPIDFile error = %v, want wrapped %v", err, wantErr)
	}

	data, err := os.ReadFile(filepath.Clean(pidFile))
	if err != nil {
		t.Fatalf("failed to read PID file: %v", err)
	}
	if string(data) != strconv.Itoa(existingPID) {
		t.Errorf("PID file contains %q, want original PID %d", string(data), existingPID)
	}
}

func TestCleanup_RemovesPIDFile(t *testing.T) {
	app := newTestApp(t)

	// Create logger so cleanup can close it
	logger, err := app.createLogger()
	if err != nil {
		t.Fatalf("createLogger: %v", err)
	}
	app.logger = logger

	if err := app.createPIDFile(); err != nil {
		t.Fatalf("createPIDFile: %v", err)
	}

	pidFile := app.config.getPortSpecificPIDFile()
	if _, err := os.Stat(pidFile); err != nil {
		t.Fatalf("PID file should exist before cleanup: %v", err)
	}

	app.cleanup()

	if _, err := os.Stat(pidFile); !os.IsNotExist(err) {
		t.Error("PID file should be removed after cleanup")
	}
}
