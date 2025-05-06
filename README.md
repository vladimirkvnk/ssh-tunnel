# SSH Tunnel Manager

![GitHub](https://img.shields.io/badge/go-%3E%3D1.24-blue)
![GitHub](https://img.shields.io/badge/license-MIT-green)

A robust SSH tunnel manager that maintains a persistent SOCKS5 proxy connection with automatic reconnection capabilities.

## Features

- 🛡️ Persistent SSH tunnel with SOCKS5 proxy
- 🔄 Automatic reconnection on failure
- 🚦 Health checking and monitoring
- ⚡ Graceful shutdown handling
- 🔧 Configurable via environment variables
- 📊 Structured JSON logging

## How It Works

The application establishes an SSH tunnel with dynamic port forwarding (SOCKS5 proxy) and continuously monitors its health by:

1. Checking if the proxy port is available
2. Making test requests through the tunnel
3. Automatically restarting the tunnel if any checks fail
4. Handling OS signals for graceful shutdown

## Quick Start

### Prerequisites

- Go 1.24+
- SSH client installed
- SSH access to remote server

### Installation

```bash
git clone https://github.com/your-repo/ssh-tunnel.git
cd ssh-tunnel
go build
```

### Basic Usage

```bash
export SSH_TUNNEL_REMOTE_ADDRESS=user@example.com
./ssh-tunnel
```

## Configuration

All configuration is done through environment variables:

### Required Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `SSH_TUNNEL_REMOTE_ADDRESS` | SSH server address (user@host) | `user@example.com` |

### Optional Variables

**Application Settings:**
| Variable | Default | Description |
|----------|---------|-------------|
| `SSH_TUNNEL_HOST` | `localhost:8080` | SOCKS5 proxy host:port |
| `SSH_TUNNEL_MAIN_LOOP_SLEEP_SEC` | `15` | Health check interval (seconds) |
| `SSH_TUNNEL_PORT_CHECK_TIMEOUT_SEC` | `4` | Port check timeout (seconds) |
| `SSH_TUNNEL_PID_FILE` | `ssh-tunnel.pid` | PID file location |
| `SSH_TUNNEL_LOG_FILE` | `ssh-tunnel.log` | Log file location |

**SSH Settings:**
| Variable | Default | Description |
|----------|---------|-------------|
| `SSH_TUNNEL_REMOTE_PORT` | `2212` | SSH server port |
| `SSH_TUNNEL_BIND_HOST` | `0.0.0.0:8080` | Local bind address |
| `SSH_TUNNEL_SERVER_ALIVE_INTERVAL` | `15` | Keepalive interval |
| `SSH_TUNNEL_CONNECT_TIMEOUT` | `10` | Connection timeout |
| `SSH_TUNNEL_STRICT_HOST_CHECKING` | `false` | Enable strict host checking |

### Example Configuration

```bash
# Required
export SSH_TUNNEL_REMOTE_ADDRESS=user@example.com

# Optional overrides
export SSH_TUNNEL_REMOTE_PORT=2222
export SSH_TUNNEL_BIND_HOST=127.0.0.1:9090
export SSH_TUNNEL_MAIN_LOOP_SLEEP_SEC=30
```

## Running as a Service

### systemd Service Example

Create `/etc/systemd/system/ssh-tunnel.service`:

```ini
[Unit]
Description=SSH Tunnel Manager
After=network.target

[Service]
Environment="SSH_TUNNEL_REMOTE_ADDRESS=user@example.com"
Environment="SSH_TUNNEL_REMOTE_PORT=2222"
ExecStart=/path/to/ssh-tunnel
Restart=always
User=youruser

[Install]
WantedBy=multi-user.target
```

Then enable and start the service:

```bash
sudo systemctl daemon-reload
sudo systemctl enable ssh-tunnel
sudo systemctl start ssh-tunnel
```

## Monitoring

The application writes structured JSON logs to the specified log file. Example log entry:

```json
{
  "time": "2023-11-15T12:34:56Z",
  "level": "INFO",
  "msg": "Starting SSH process",
  "pid": 12345,
  "args": ["ssh", "-N", "-C", "-D", "0.0.0.0:8080", "user@example.com"]
}
```

## Troubleshooting

1. **Connection failures**:
   - Verify SSH access works manually
   - Check firewall settings
   - Enable debug logging by setting `LOG_LEVEL=debug`

2. **Port in use**:
   - Change `SSH_TUNNEL_BIND_HOST` to use a different port
   - Verify no other instances are running

3. **Permission issues**:
   - Ensure user has write access to PID and log files
   - Verify SSH keys are properly configured

## License

MIT License.
