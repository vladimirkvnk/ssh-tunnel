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

type Application struct {
	config        *config
	httpTransport *http.Transport
	logger        *slog.Logger
	sshProcess    *exec.Cmd
	sshMutex      sync.RWMutex
	shutdownChan  chan struct{}
}

func main() {
	// Initialize configuration
	config, err := newConfig()
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Initialize application
	app := &Application{
		config:       config,
		shutdownChan: make(chan struct{}),
	}

	if err := app.initialize(); err != nil {
		slog.Error("Initialization failed", "error", err)
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
	logFile := app.config.getPortSpecificLogFile()
	file, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
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
		return net.Dial("tcp", app.config.ProxyHost)
	}

	proxyFunc := func(r *http.Request) (*url.URL, error) {
		proxyURL := fmt.Sprintf("socks5://%s", app.config.ProxyHost)
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

	ticker := time.NewTicker(app.config.MainLoopSleep)
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
	conn, err := net.DialTimeout("tcp", app.config.ProxyHost, app.config.PortCheckTimeout)
	if err != nil {
		app.logger.Error("Proxy port unavailable", "host", app.config.ProxyHost, "error", err)
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
	cmd := exec.Command("ssh", app.config.serializeSSHOptions()...)
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
	for range 5 {
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
	pidFile := app.config.getPortSpecificPIDFile()
	
	if _, err := os.Stat(pidFile); err == nil {
		content, err := os.ReadFile(pidFile)
		if err != nil {
			return fmt.Errorf("failed to read PID file: %w", err)
		}

		pid, err := strconv.Atoi(string(bytes.TrimSpace(content)))
		if err != nil {
			return fmt.Errorf("failed to parse PID: %w", err)
		}

		if process, err := os.FindProcess(pid); err == nil {
			if err = process.Signal(syscall.Signal(0)); err == nil {
				_, port, _ := net.SplitHostPort(app.config.ProxyHost)
				return fmt.Errorf("another instance is already running on port %s with PID %d", port, pid)
			}
		}

		os.Remove(pidFile)
	}

	return os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())), 0644)
}

// cleanup performs application cleanup tasks.
func (app *Application) cleanup() {
	app.stopSSH()

	pidFile := app.config.getPortSpecificPIDFile()
	if err := os.Remove(pidFile); err != nil && !os.IsNotExist(err) {
		app.logger.Error("Failed to remove PID file", "error", err)
	}

	app.logger.Info("Application shutdown complete")
}
