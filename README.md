# port-who

> Who is eating my port?

`port-who` shows what process is listening on any TCP port — name, PID, uptime, full command. No `lsof` flags to memorize. Zero dependencies, reads `/proc` directly.

```
❯ port-who

  PORT     PROCESS               PID       UPTIME
  ──────────────────────────────────────────────────
  :3000    next-server           167564    3h 23m
  :5432    postgres              1823      5d 2h
  :6379    redis-server          2041      5d 2h
  :8080    caddy                 998       5d 2h

  port-who <port>  for details  ·  port-who --kill <port>  to terminate
```

```
❯ port-who 3000

  next-server (v15.5.18)  PID 167564
  node /home/dave/projects/myapp/.next/standalone/server.js
  uptime  3h 23m

  port-who --kill 3000  to terminate
```

```
❯ port-who --kill 3000

  killing next-server (PID 167564) … ✓
```

## Install

### One line (no Go needed)

```bash
curl -fsSL https://raw.githubusercontent.com/dawitlabs/port-who/main/install.sh | sh
```

### Go

```bash
go install github.com/dawitlabs/port-who@latest
```

### Manual — download binary

Go to [Releases](https://github.com/dawitlabs/port-who/releases/latest):

| Platform | File |
|----------|------|
| Linux x86_64 | `port-who-linux-amd64` |
| Linux ARM64 | `port-who-linux-arm64` |
| Mac (Apple Silicon) | `port-who-darwin-arm64` |
| Mac (Intel) | `port-who-darwin-amd64` |

```bash
chmod +x port-who-linux-amd64 && mv port-who-linux-amd64 ~/.local/bin/port-who
```

## Usage

```bash
port-who               # list all listening TCP ports
port-who 3000          # who's on :3000?
port-who --kill 3000   # send SIGTERM to whatever owns :3000
```

## How it works

Reads `/proc/net/tcp` and `/proc/net/tcp6` directly — no `lsof`, no `ss`, no `netstat`. Maps socket inodes to PIDs via `/proc/*/fd/`, then reads name, command, and uptime from `/proc/<pid>/`. Single static binary, no runtime deps.

> Linux only. macOS users: `lsof -i :3000` or [use sudo](https://stackoverflow.com/questions/4421633).

## License

MIT
