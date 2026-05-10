package main

import (
	"strings"
	"testing"
	"time"
)

// validConfig returns a minimal valid config for testing.
func validConfig() config {
	return config{
		MainLoopSleep:          15 * time.Second,
		PortCheckTimeout:       4 * time.Second,
		PIDFile:                "ssh-tunnel.pid",
		LogFile:                "ssh-tunnel.log",
		SSHTCPKeepAlive:        true,
		SSHServerAliveInterval: 15,
		SSHConnectTimeout:      10,
		SSHStrictHostChecking:  false,
		SSHBindHost:            "127.0.0.1:8080",
		SSHRemoteAddress:       "user@host",
		SSHRemotePort:          2212,
		SSHSocksDNS:            "local",
	}
}

// --- deriveProxyHost ---

func TestDeriveProxyHost_Loopback(t *testing.T) {
	cfg := validConfig()
	if err := cfg.validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.proxyHost != "127.0.0.1:8080" {
		t.Errorf("proxyHost = %q, want %q", cfg.proxyHost, "127.0.0.1:8080")
	}
	if cfg.proxyPort != "8080" {
		t.Errorf("proxyPort = %q, want %q", cfg.proxyPort, "8080")
	}
}

func TestDeriveProxyHost_WildcardIPv4(t *testing.T) {
	cfg := validConfig()
	cfg.SSHBindHost = "0.0.0.0:9090"
	if err := cfg.validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.proxyHost != "127.0.0.1:9090" {
		t.Errorf("proxyHost = %q, want %q", cfg.proxyHost, "127.0.0.1:9090")
	}
}

func TestDeriveProxyHost_WildcardIPv6(t *testing.T) {
	cfg := validConfig()
	cfg.SSHBindHost = "[::]:8080"
	if err := cfg.validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.proxyHost != "[::1]:8080" {
		t.Errorf("proxyHost = %q, want %q", cfg.proxyHost, "[::1]:8080")
	}
}

func TestDeriveProxyHost_InvalidBindHost(t *testing.T) {
	tests := []struct {
		name string
		host string
	}{
		{"empty", ""},
		{"no port", "localhost"},
		{"port out of range", "127.0.0.1:99999"},
		{"port zero", "127.0.0.1:0"},
		{"negative port", "127.0.0.1:-1"},
		{"non-numeric port", "127.0.0.1:abc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.SSHBindHost = tt.host
			if err := cfg.validate(); err == nil {
				t.Error("expected error")
			}
		})
	}
}

// --- validate ---

func TestValidate_RemotePort(t *testing.T) {
	tests := []struct {
		name string
		port int
		ok   bool
	}{
		{"valid", 22, true},
		{"valid max", 65535, true},
		{"zero", 0, false},
		{"negative", -1, false},
		{"too large", 65536, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.SSHRemotePort = tt.port
			err := cfg.validate()
			if (err == nil) != tt.ok {
				t.Errorf("validate() with port %d: err=%v, want ok=%v", tt.port, err, tt.ok)
			}
		})
	}
}

func TestValidate_MainLoopSleep(t *testing.T) {
	cfg := validConfig()
	cfg.MainLoopSleep = 0
	if err := cfg.validate(); err == nil {
		t.Error("expected error for zero MainLoopSleep")
	}
}

func TestValidate_PortCheckTimeout(t *testing.T) {
	cfg := validConfig()
	cfg.PortCheckTimeout = -1 * time.Second
	if err := cfg.validate(); err == nil {
		t.Error("expected error for negative PortCheckTimeout")
	}
}

func TestValidate_SocksDNS(t *testing.T) {
	tests := []struct {
		mode string
		ok   bool
		want string
	}{
		{"local", true, "local"},
		{"LOCAL", true, "local"},
		{"remote", true, "remote"},
		{"Remote", true, "remote"},
		{"", true, "local"},
		{"invalid", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			cfg := validConfig()
			cfg.SSHSocksDNS = tt.mode
			err := cfg.validate()
			if (err == nil) != tt.ok {
				t.Errorf("mode=%q: err=%v, want ok=%v", tt.mode, err, tt.ok)
			}
			if tt.ok && cfg.SSHSocksDNS != tt.want {
				t.Errorf("mode=%q normalized to %q, want %q", tt.mode, cfg.SSHSocksDNS, tt.want)
			}
		})
	}
}

// --- serializeSSHOptions ---

func TestSerializeSSHOptions_Defaults(t *testing.T) {
	cfg := validConfig()
	if err := cfg.validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}

	opts := cfg.serializeSSHOptions()
	joined := strings.Join(opts, " ")

	if !strings.Contains(joined, "-N -C") {
		t.Error("missing -N -C base flags")
	}
	if !strings.Contains(joined, "TCPKeepAlive=yes") {
		t.Error("missing TCPKeepAlive=yes")
	}
	if !strings.Contains(joined, "StrictHostKeyChecking=no") {
		t.Error("missing StrictHostKeyChecking=no")
	}
	if !strings.Contains(joined, "-D 127.0.0.1:8080") {
		t.Error("missing dynamic forwarding flag")
	}
	if !strings.Contains(joined, "-p 2212") {
		t.Error("missing remote port flag")
	}
	if !strings.Contains(joined, "user@host") {
		t.Error("missing remote address")
	}
}

func TestSerializeSSHOptions_KeepAliveDisabled(t *testing.T) {
	cfg := validConfig()
	cfg.SSHTCPKeepAlive = false
	if err := cfg.validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}

	opts := cfg.serializeSSHOptions()
	joined := strings.Join(opts, " ")

	if strings.Contains(joined, "TCPKeepAlive") {
		t.Error("TCPKeepAlive should not be present when disabled")
	}
}

func TestSerializeSSHOptions_StrictHostCheckingEnabled(t *testing.T) {
	cfg := validConfig()
	cfg.SSHStrictHostChecking = true
	if err := cfg.validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}

	opts := cfg.serializeSSHOptions()
	joined := strings.Join(opts, " ")

	if strings.Contains(joined, "StrictHostKeyChecking") {
		t.Error("StrictHostKeyChecking should not be present when enabled (ssh default is ask)")
	}
}

func TestSerializeSSHOptions_NoServerAliveInterval(t *testing.T) {
	cfg := validConfig()
	cfg.SSHServerAliveInterval = 0
	if err := cfg.validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}

	opts := cfg.serializeSSHOptions()
	joined := strings.Join(opts, " ")

	if strings.Contains(joined, "ServerAliveInterval") {
		t.Error("ServerAliveInterval should not be present when set to 0")
	}
}

// --- getPortSpecificPIDFile ---

func TestGetPortSpecificPIDFile(t *testing.T) {
	cfg := validConfig()
	if err := cfg.validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}

	tests := []struct {
		pidFile string
		want    string
	}{
		{"ssh-tunnel.pid", "ssh-tunnel-8080.pid"},
		{"custom.pid", "custom-8080.pid"},
		{"noext", "noext-8080"},
		{"/tmp/tunnel.pid", "/tmp/tunnel-8080.pid"},
	}

	for _, tt := range tests {
		t.Run(tt.pidFile, func(t *testing.T) {
			cfg.PIDFile = tt.pidFile
			if got := cfg.getPortSpecificPIDFile(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// --- getPortSpecificLogFile ---

func TestGetPortSpecificLogFile(t *testing.T) {
	cfg := validConfig()
	if err := cfg.validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}

	tests := []struct {
		logFile string
		want    string
	}{
		{"ssh-tunnel.log", "ssh-tunnel-8080.log"},
		{"custom.log", "custom-8080.log"},
		{"noext", "noext-8080"},
		{"/var/log/tunnel.log", "/var/log/tunnel-8080.log"},
	}

	for _, tt := range tests {
		t.Run(tt.logFile, func(t *testing.T) {
			cfg.LogFile = tt.logFile
			if got := cfg.getPortSpecificLogFile(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
