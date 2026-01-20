# SSH Tunnel Manager

Persistent SSH SOCKS5 tunnel with health checks and auto-restart.

## Quick start

```bash
export SSH_TUNNEL_REMOTE_ADDRESS=user@example.com
./ssh-tunnel
```

## Configuration

Required:
- `SSH_TUNNEL_REMOTE_ADDRESS` (user@host)

Common optional:
- `SSH_TUNNEL_BIND_HOST` (default `127.0.0.1:8080`)
- `SSH_TUNNEL_REMOTE_PORT` (default `2212`)
- `SSH_TUNNEL_MAIN_LOOP_SLEEP_SEC` (default `15s`, Go duration)
- `SSH_TUNNEL_PORT_CHECK_TIMEOUT_SEC` (default `4s`, Go duration)
- `SSH_TUNNEL_LOG_STDOUT` (default `false`)
- `SSH_TUNNEL_SOCKS_DNS` (`local` or `remote`, default `local`)

Advanced:
- `SSH_TUNNEL_TCP_KEEPALIVE` (default `true`)
- `SSH_TUNNEL_SERVER_ALIVE_INTERVAL` (default `15`)
- `SSH_TUNNEL_CONNECT_TIMEOUT` (default `10`)
- `SSH_TUNNEL_STRICT_HOST_CHECKING` (default `false`)
- `SSH_TUNNEL_PID_FILE` (default `ssh-tunnel.pid`)
- `SSH_TUNNEL_LOG_FILE` (default `ssh-tunnel.log`)

## Multiple instances

Use different ports in `SSH_TUNNEL_BIND_HOST`. Log/PID files are suffixed with the port (e.g. `ssh-tunnel-8080.log`).

```bash
SSH_TUNNEL_REMOTE_ADDRESS=user@example.com SSH_TUNNEL_BIND_HOST=127.0.0.1:8080 ./ssh-tunnel &
SSH_TUNNEL_REMOTE_ADDRESS=user@example.com SSH_TUNNEL_BIND_HOST=127.0.0.1:9090 ./ssh-tunnel &
```

## License

MIT
