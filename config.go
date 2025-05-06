package main

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Default configuration values
const (
	DefaultProxyHost        = "localhost:8080"
	DefaultMainLoopSleep    = 15 * time.Second
	DefaultPortCheckTimeout = 4 * time.Second
	DefaultPIDFile          = "ssh-tunnel.pid"
	DefaultLogFile          = "ssh-tunnel.log"
	DefaultServerAliveInt   = 15
	DefaultConnectTimeout   = 10
	DefaultRemotePort       = 2212
)

// config holds all application configuration parameters.
// Fields can be set via environment variables as noted in struct tags.
type config struct {
	// Proxy settings
	ProxyHost        string        `env:"SSH_TUNNEL_HOST"`
	MainLoopSleep    time.Duration `env:"SSH_TUNNEL_MAIN_LOOP_SLEEP_SEC"`
	PortCheckTimeout time.Duration `env:"SSH_TUNNEL_PORT_CHECK_TIMEOUT_SEC"`
	PIDFile          string        `env:"SSH_TUNNEL_PID_FILE"`
	LogFile          string        `env:"SSH_TUNNEL_LOG_FILE"`

	// SSH options
	SSHOptions sshOptions
}

// sshOptions contains SSH-specific configuration parameters.
type sshOptions struct {
	TCPKeepAlive        bool     `env:"SSH_TUNNEL_TCP_KEEPALIVE"`
	ServerAliveInterval int      `env:"SSH_TUNNEL_SERVER_ALIVE_INTERVAL"`
	ConnectTimeout      int      `env:"SSH_TUNNEL_CONNECT_TIMEOUT"`
	StrictHostChecking  bool     `env:"SSH_TUNNEL_STRICT_HOST_CHECKING"`
	BindHost            string   `env:"SSH_TUNNEL_BIND_HOST"`
	RemoteAddress       string   `env:"SSH_TUNNEL_REMOTE_ADDRESS"`
	RemotePort          int      `env:"SSH_TUNNEL_REMOTE_PORT"`
	MiscOptions         []string // Additional SSH options
}

// newConfigFromEnv creates a new config instance populated from environment variables.
// Returns error if required configuration is missing or invalid.
func newConfigFromEnv() (*config, error) {
	conf := &config{
		ProxyHost:        DefaultProxyHost,
		MainLoopSleep:    DefaultMainLoopSleep,
		PortCheckTimeout: DefaultPortCheckTimeout,
		PIDFile:          DefaultPIDFile,
		LogFile:          DefaultLogFile,
		SSHOptions: sshOptions{
			TCPKeepAlive:        true,
			ServerAliveInterval: DefaultServerAliveInt,
			ConnectTimeout:      DefaultConnectTimeout,
			StrictHostChecking:  false,
			BindHost:            "0.0.0.0:8080",
			MiscOptions:         []string{"-N", "-C"}, // No command, compression
		},
	}

	// Load from environment variables
	if err := conf.loadFromEnv(); err != nil {
		return nil, err
	}

	// Validate configuration
	if err := conf.validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return conf, nil
}

// loadFromEnv populates the config from environment variables.
func (c *config) loadFromEnv() error {
	if val := os.Getenv("SSH_TUNNEL_HOST"); val != "" {
		c.ProxyHost = val
	}

	if val := os.Getenv("SSH_TUNNEL_MAIN_LOOP_SLEEP_SEC"); val != "" {
		seconds, err := strconv.Atoi(val)
		if err != nil {
			return fmt.Errorf("invalid SSH_TUNNEL_MAIN_LOOP_SLEEP_SEC: %w", err)
		}
		c.MainLoopSleep = time.Duration(seconds) * time.Second
	}

	if val := os.Getenv("SSH_TUNNEL_PORT_CHECK_TIMEOUT_SEC"); val != "" {
		seconds, err := strconv.Atoi(val)
		if err != nil {
			return fmt.Errorf("invalid SSH_TUNNEL_PORT_CHECK_TIMEOUT_SEC: %w", err)
		}
		c.PortCheckTimeout = time.Duration(seconds) * time.Second
	}

	if val := os.Getenv("SSH_TUNNEL_PID_FILE"); val != "" {
		c.PIDFile = val
	}

	if val := os.Getenv("SSH_TUNNEL_LOG_FILE"); val != "" {
		c.LogFile = val
	}

	// SSH Options
	if val := os.Getenv("SSH_TUNNEL_REMOTE_ADDRESS"); val != "" {
		c.SSHOptions.RemoteAddress = val
	} else {
		return fmt.Errorf("SSH_TUNNEL_REMOTE_ADDRESS is required")
	}

	if val := os.Getenv("SSH_TUNNEL_REMOTE_PORT"); val != "" {
		port, err := strconv.Atoi(val)
		if err != nil {
			return fmt.Errorf("invalid SSH_TUNNEL_REMOTE_PORT: %w", err)
		}
		c.SSHOptions.RemotePort = port
	} else {
		c.SSHOptions.RemotePort = DefaultRemotePort
	}

	if val := os.Getenv("SSH_TUNNEL_BIND_HOST"); val != "" {
		c.SSHOptions.BindHost = val
	}

	return nil
}

// validate checks if the configuration is valid.
func (c *config) validate() error {
	if c.SSHOptions.RemoteAddress == "" {
		return fmt.Errorf("remote address cannot be empty")
	}

	if c.SSHOptions.RemotePort <= 0 || c.SSHOptions.RemotePort > 65535 {
		return fmt.Errorf("invalid remote port: %d", c.SSHOptions.RemotePort)
	}

	if c.MainLoopSleep <= 0 {
		return fmt.Errorf("main loop sleep duration must be positive")
	}

	return nil
}

// serializeSSHOptions converts SSH options to command line arguments.
func (c *config) serializeSSHOptions() []string {
	opts := make([]string, 0, 16)

	// Add miscellaneous options first
	opts = append(opts, c.SSHOptions.MiscOptions...)

	// TCP keepalive
	if c.SSHOptions.TCPKeepAlive {
		opts = append(opts, "-o", "TCPKeepAlive=yes")
	} else {
		opts = append(opts, "-o", "TCPKeepAlive=no")
	}

	// Server alive interval
	if c.SSHOptions.ServerAliveInterval > 0 {
		opts = append(opts, "-o", fmt.Sprintf("ServerAliveInterval=%d", c.SSHOptions.ServerAliveInterval))
	}

	// Connect timeout
	if c.SSHOptions.ConnectTimeout > 0 {
		opts = append(opts, "-o", fmt.Sprintf("ConnectTimeout=%d", c.SSHOptions.ConnectTimeout))
	}

	// Strict host key checking
	if !c.SSHOptions.StrictHostChecking {
		opts = append(opts, "-o", "StrictHostKeyChecking=no")
	}

	// Dynamic port forwarding
	opts = append(opts,
		"-D", c.SSHOptions.BindHost,
		c.SSHOptions.RemoteAddress,
		"-p", fmt.Sprintf("%d", c.SSHOptions.RemotePort),
	)

	return opts
}
