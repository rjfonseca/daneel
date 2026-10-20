# Daemon Core & Lifecycle

Daneel uses a lightweight, local-first Daemon Core running over a Unix Domain Socket to coordinate background tasks, prevent SQLite write lock contention, and manage real-time UI synchronization.

## 1. Socket & PID Conventions

- **Socket Path**: `$XDG_STATE_HOME/daneel/daneel.sock` (defaults to `~/.local/state/daneel/daneel.sock`).
- **PID File**: `~/.local/state/daneel/daneel.pid`.
- **Directory Permissions**: Created automatically with `0700` (user-only access).

## 2. Operational Modes

### Foreground Mode (`daneel daemon run`)
Executes the socket server directly attached to stdout/stderr. Traps `SIGINT` and `SIGTERM` to perform graceful cleanup of the socket and PID file upon exit. Ideal for debugging or container environments.

### Background Detached Mode (`daneel daemon start` & `stop`)
- `daneel daemon start`: Spawns a detached child process (`setsid`), writes the PID, and verifies the socket becomes ready within 3 seconds.
- `daneel daemon stop`: Reads the PID file, sends `SIGTERM`, and confirms shutdown.
- `daneel daemon status`: Reports running state, PID, socket location, active surface (`web`, `tui`, or `none`), and last active timestamp.

### Linux User Service (`systemd --user`)
Daneel provides built-in systemd integration for persistent background operation on Linux:
```bash
daneel daemon service install    # Generates unit file, runs daemon-reload, enables and starts service
daneel daemon service status     # Runs systemctl --user status daneel
daneel daemon service uninstall  # Stops, disables, and removes unit file
```
The generated unit file at `~/.config/systemd/user/daneel.service` configures:
- `Restart=on-failure` with `RestartSec=5`
- Automatic start upon user login (`WantedBy=default.target`)

### Turnkey Multi-Listener & Routine Installation
You can install both the unified daemon (hosting both the Unix socket and the HTTP visualizer) and all scheduled maintenance routines with a single command:
```bash
daneel daemon service install --with-serve --port 4321 --with-routines
```
To tear down the service and routine timers:
```bash
daneel daemon service uninstall --with-routines
```

## 3. Unified Multi-Listener (`daneel daemon run --serve`)

A single persistent process can manage both the Unix socket IPC and the HTTP web visualizer (ConnectRPC + embedded WebFS):
```bash
daneel daemon run --serve --port 4321
```
Benefits of the unified multi-listener:
- **Shared Memory Pool**: Shares the in-memory SQLite cache and search engine, eliminating multi-process lock contention.
- **Mutual Graceful Shutdown**: On `SIGINT` or `SIGTERM`, drains HTTP connections within 3 seconds and closes the Unix socket cleanly.
- **SQLite Concurrency**: SQLite connections use WAL mode with `busy_timeout=10000` to smoothly handle concurrent writes from background batch routines.

## 4. Scheduled Routines Engine (`daneel routine`)

Daneel delegates periodic maintenance to individual systemd user units and timers (`Type=oneshot`):
- `daneel routine list`: Lists all configured routines with their schedule, active state, and trigger history.
- `daneel routine install [--all | <name>]`: Generates and enables `daneel-<name>.{service,timer}` under `~/.config/systemd/user/`.
- `daneel routine run <name>`: Immediately executes a routine oneshot via systemd.
- `daneel routine logs <name> [-n 50] [-f]`: Streams logs from `journalctl --user -u daneel-<name>.service`.
- `daneel routine enable / disable <name>`: Controls timer activation.


The HTTP listener is restricted to `127.0.0.1`, including unified daemon mode. The configuration `serve.host` field does not enable remote access. Remote proxies and network serving are unsupported; see [ADR 0025](../adr/0025-local-http-and-vault-read-boundaries.md).
