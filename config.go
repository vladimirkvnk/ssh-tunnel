package main

import (
	"fmt"
	"net/url"
	"time"

	"github.com/caarlos0/env/v11"
)

type config struct {
	proxyHost        string        `env:"SSH_TUNNEL_HOST" envDefault:"localhost:8080"`
	mainLoopSleep    time.Duration `env:"SSH_TUNNEL_MAIN_LOOP_SLEEP_SEC" envDefault:"15s"`
	portCheckTimeout time.Duration `env:"SSH_TUNNEL_PORT_CHECK_TIMEOUT_SEC" envDefault:"4s"`
	pidFile          string        `env:"SSH_TUNNEL_PID_FILE" envDefault:"ssh-tunnel.pid"`
	logFile          string        `env:"SSH_TUNNEL_LOG_FILE" envDefault:"ssh-tunnel.log"`
	sshOptions       sshOptions
}

type sshOptions struct {
	tcpKeepAlive        bool     `env:"SSH_TUNNEL_TCP_KEEPALIVE" envDefault:"true"`
	serverAliveInterval int      `env:"SSH_TUNNEL_SERVER_ALIVE_INTERVAL" envDefault:"15"`
	connectTimeout      int      `env:"SSH_TUNNEL_CONNECT_TIMEOUT" envDefault:"10"`
	strictHostChecking  bool     `env:"SSH_TUNNEL_STRICT_HOST_CHECKING" envDefault:"false"`
	bindHost            string   `env:"SSH_TUNNEL_BIND_HOST" envDefault:"0.0.0.0:8080"`
	remoteAddress       string   `env:"SSH_TUNNEL_REMOTE_ADDRESS,required"`
	remotePort          int      `env:"SSH_TUNNEL_REMOTE_PORT" envDefault:"2212"`
	miscOptions         []string `env:"SSH_TUNNEL_MISC_OPTIONS" envSeparator:" " envDefault:"-N -C"`
}

func newConfig() (*config, error) {
	var cfg config
	if err := env.Parse(&cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &cfg, nil
}

func (c *config) validate() error {
	if _, err := url.Parse("tcp://" + c.proxyHost); err != nil {
		return fmt.Errorf("invalid proxy host: %w", err)
	}

	if c.sshOptions.remotePort <= 0 || c.sshOptions.remotePort > 65535 {
		return fmt.Errorf("invalid remote port: %d", c.sshOptions.remotePort)
	}

	if c.mainLoopSleep <= 0 {
		return fmt.Errorf("main loop sleep must be positive")
	}

	return nil
}

func (c *config) serializeSSHOptions() []string {
	opts := make([]string, 0, 16)

	// Add miscellaneous options
	opts = append(opts, c.sshOptions.miscOptions...)

	// TCP keepalive
	if c.sshOptions.tcpKeepAlive {
		opts = append(opts, "-o", "TCPKeepAlive=yes")
	}

	// Server alive interval
	if c.sshOptions.serverAliveInterval > 0 {
		opts = append(opts, "-o", fmt.Sprintf("ServerAliveInterval=%d", c.sshOptions.serverAliveInterval))
	}

	// Connect timeout
	if c.sshOptions.connectTimeout > 0 {
		opts = append(opts, "-o", fmt.Sprintf("ConnectTimeout=%d", c.sshOptions.connectTimeout))
	}

	// Strict host key checking
	if !c.sshOptions.strictHostChecking {
		opts = append(opts, "-o", "StrictHostKeyChecking=no")
	}

	// Dynamic port forwarding
	opts = append(opts,
		"-D", c.sshOptions.bindHost,
		c.sshOptions.remoteAddress,
		"-p", fmt.Sprintf("%d", c.sshOptions.remotePort),
	)

	return opts
}
