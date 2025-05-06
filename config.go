package main

import (
	"fmt"
	"net/url"
	"time"

	"github.com/caarlos0/env/v11"
)

type config struct {
	// Main config
	ProxyHost        string        `env:"HOST" envDefault:"localhost:8080"`
	MainLoopSleep    time.Duration `env:"MAIN_LOOP_SLEEP_SEC" envDefault:"15s"`
	PortCheckTimeout time.Duration `env:"PORT_CHECK_TIMEOUT_SEC" envDefault:"4s"`
	PIDFile          string        `env:"PID_FILE" envDefault:"ssh-tunnel.pid"`
	LogFile          string        `env:"LOG_FILE" envDefault:"ssh-tunnel.log"`

	// SSH Options
	SSHTCPKeepAlive        bool     `env:"TCP_KEEPALIVE" envDefault:"true"`
	SSHServerAliveInterval int      `env:"SERVER_ALIVE_INTERVAL" envDefault:"15"`
	SSHConnectTimeout      int      `env:"CONNECT_TIMEOUT" envDefault:"10"`
	SSHStrictHostChecking  bool     `env:"STRICT_HOST_CHECKING" envDefault:"false"`
	SSHBindHost            string   `env:"BIND_HOST" envDefault:"0.0.0.0:8080"`
	SSHRemoteAddress       string   `env:"REMOTE_ADDRESS,required"`
	SSHRemotePort          int      `env:"REMOTE_PORT" envDefault:"2212"`
	SSHMiscOptions         []string `env:"MISC_OPTIONS" envSeparator:" " envDefault:"-N -C"`
}

func newConfig() (*config, error) {
	var cfg config
	opts := env.Options{
		Prefix: "SSH_TUNNEL_",
	}

	if err := env.ParseWithOptions(&cfg, opts); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &cfg, nil
}

func (c *config) validate() error {
	if _, err := url.Parse("tcp://" + c.ProxyHost); err != nil {
		return fmt.Errorf("invalid proxy host: %w", err)
	}

	if c.SSHRemotePort <= 0 || c.SSHRemotePort > 65535 {
		return fmt.Errorf("invalid remote port: %d", c.SSHRemotePort)
	}

	if c.MainLoopSleep <= 0 {
		return fmt.Errorf("main loop sleep must be positive")
	}

	return nil
}

func (c *config) serializeSSHOptions() []string {
	opts := make([]string, 0, 16)

	// Add miscellaneous options
	opts = append(opts, c.SSHMiscOptions...)

	// TCP keepalive
	if c.SSHTCPKeepAlive {
		opts = append(opts, "-o", "TCPKeepAlive=yes")
	}

	// Server alive interval
	if c.SSHServerAliveInterval > 0 {
		opts = append(opts, "-o", fmt.Sprintf("ServerAliveInterval=%d", c.SSHServerAliveInterval))
	}

	// Connect timeout
	if c.SSHConnectTimeout > 0 {
		opts = append(opts, "-o", fmt.Sprintf("ConnectTimeout=%d", c.SSHConnectTimeout))
	}

	// Strict host key checking
	if !c.SSHStrictHostChecking {
		opts = append(opts, "-o", "StrictHostKeyChecking=no")
	}

	// Dynamic port forwarding
	opts = append(opts,
		"-D", c.SSHBindHost,
		c.SSHRemoteAddress,
		"-p", fmt.Sprintf("%d", c.SSHRemotePort),
	)

	return opts
}
