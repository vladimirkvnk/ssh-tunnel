package main

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"
)

// Application represents the main application structure.
type Application struct {
	conf          *config
	httpTransport *http.Transport
	logger        *slog.Logger
	sshProcess    *exec.Cmd
	sshMutex      sync.RWMutex
	shutdownChan  chan struct{}
}

func main() {
	// Initialize configuration
	conf, err := newConfigFromEnv()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize application
	app := &Application{
		conf:         conf,
		shutdownChan: make(chan struct{}),
	}

	if err := app.initialize(); err != nil {
		fmt.Fprintf(os.Stderr, "Initialization failed: %v\n", err)
		os.Exit(1)
	}
	defer app.cleanup()

	// Run main loop
	app.run()
}

// initialize sets up the application components.
func (app *Application) initialize() error {
	// Initialize logger
	logger, err := app.createLogger()
	if err != nil {
		return fmt.Errorf("logger initialization failed: %w", err)
	}
	app.logger = logger

	// Create PID file
	if err := app.createPIDFile(); err != nil {
		return fmt.Errorf("PID file creation failed: %w", err)
	}

	// Setup HTTP transport
	app.httpTransport = app.createHTTPTransport()

	// Setup signal handling
	app.setupSignalHandler()

	return nil
}

// createLogger initializes the application logger.
func (app *Application) createLogger() (*slog.Logger, error) {
	file, err := os.OpenFile(app.conf.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	return slog.New(slog.NewJSONHandler(file, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})), nil
}

// createHTTPTransport creates a configured HTTP transport.
func (app *Application) createHTTPTransport() *http.Transport {
	dialFunc := func(network, addr string) (net.Conn, error) {
		return net.Dial("tcp", app.conf.ProxyHost)
	}

	proxyFunc := func(r *http.Request) (*url.URL, error) {
		proxyURL := fmt.Sprintf("socks5://%s", app.conf.ProxyHost)
		return url.Parse(proxyURL)
	}

	return &http.Transport{
		Dial:            dialFunc,
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		Proxy:           proxyFunc,
	}
}

// setupSignalHandler configures OS signal handling.
func (app *Application) setupSignalHandler() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		app.logger.Info("Received signal, shutting down", "signal", sig)
		close(app.shutdownChan)
	}()
}

// run executes the main application loop.
func (app *Application) run() {
	app.logger.Info("Starting SSH tunnel application")

	ticker := time.NewTicker(app.conf.MainLoopSleep)
	defer ticker.Stop()

	for {
		select {
		case <-app.shutdownChan:
			app.logger.Info("Shutting down...")
			return
		case <-ticker.C:
			if !app.checkTraffic() {
				app.restartTunnel()
			}
		}
	}
}

// restartTunnel stops and starts the SSH tunnel.
func (app *Application) restartTunnel() {
	app.stopSSH()
	if err := app.startSSH(); err != nil {
		app.logger.Error("Failed to restart SSH tunnel", "error", err)
	}
}

// checkTraffic verifies if the tunnel is functioning properly.
func (app *Application) checkTraffic() bool {
	if !app.checkPort() {
		return false
	}

	client := &http.Client{
		Transport: app.httpTransport,
		Timeout:   10 * time.Second,
	}

	req, err := http.NewRequest("HEAD", "https://google.com", nil)
	if err != nil {
		app.logger.Error("Failed to create request", "error", err)
		return false
	}

	resp, err := client.Do(req)
	if err != nil {
		app.logger.Error("Traffic check failed", "error", err)
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// checkPort verifies if the proxy port is available.
func (app *Application) checkPort() bool {
	conn, err := net.DialTimeout("tcp", app.conf.ProxyHost, app.conf.PortCheckTimeout)
	if err != nil {
		app.logger.Error("Proxy port unavailable", "host", app.conf.ProxyHost, "error", err)
		return false
	}
	conn.Close()
	return true
}

// startSSH starts the SSH tunnel process.
func (app *Application) startSSH() error {
	app.sshMutex.Lock()
	defer app.sshMutex.Unlock()

	if app.sshProcess != nil && app.isProcessRunning(app.sshProcess) {
		app.logger.Info("SSH process is already running")
		return nil
	}

	app.logger.Info("Starting SSH process")
	cmd := exec.Command("ssh", app.conf.serializeSSHOptions()...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start SSH: %w", err)
	}

	app.sshProcess = cmd

	// Verify the tunnel is ready
	if !app.waitForTunnelReady() {
		return fmt.Errorf("tunnel failed to become ready")
	}

	return nil
}

// isProcessRunning checks if a process is running.
func (app *Application) isProcessRunning(cmd *exec.Cmd) bool {
	return cmd != nil && cmd.Process != nil && cmd.ProcessState == nil
}

// waitForTunnelReady waits for the tunnel to become available.
func (app *Application) waitForTunnelReady() bool {
	for i := 0; i < 5; i++ {
		if app.checkPort() {
			app.logger.Info("SSH tunnel is ready")
			return true
		}
		time.Sleep(1 * time.Second)
	}
	return false
}

// stopSSH stops the SSH tunnel process.
func (app *Application) stopSSH() {
	app.sshMutex.Lock()
	defer app.sshMutex.Unlock()

	if app.sshProcess == nil || !app.isProcessRunning(app.sshProcess) {
		return
	}

	app.logger.Info("Stopping SSH process")
	if err := app.sshProcess.Process.Signal(syscall.SIGTERM); err != nil {
		app.logger.Error("Failed to send SIGTERM", "error", err)
		if err := app.sshProcess.Process.Kill(); err != nil {
			app.logger.Error("Failed to kill process", "error", err)
		}
	}

	_, err := app.sshProcess.Process.Wait()
	if err != nil {
		app.logger.Error("Error waiting for process", "error", err)
	}

	app.sshProcess = nil
}

// createPIDFile creates the PID file.
func (app *Application) createPIDFile() error {
	if _, err := os.Stat(app.conf.PIDFile); err == nil {
		content, err := os.ReadFile(app.conf.PIDFile)
		if err != nil {
			return fmt.Errorf("failed to read PID file: %w", err)
		}

		pid, err := strconv.Atoi(string(bytes.TrimSpace(content)))
		if err != nil {
			return fmt.Errorf("failed to parse PID: %w", err)
		}

		if process, err := os.FindProcess(pid); err == nil {
			if err = process.Signal(syscall.Signal(0)); err == nil {
				return fmt.Errorf("another instance is already running with PID %d", pid)
			}
		}

		os.Remove(app.conf.PIDFile)
	}

	return os.WriteFile(app.conf.PIDFile, []byte(strconv.Itoa(os.Getpid())), 0644)
}

// cleanup performs application cleanup tasks.
func (app *Application) cleanup() {
	app.stopSSH()

	if err := os.Remove(app.conf.PIDFile); err != nil && !os.IsNotExist(err) {
		app.logger.Error("Failed to remove PID file", "error", err)
	}

	app.logger.Info("Application shutdown complete")
}
