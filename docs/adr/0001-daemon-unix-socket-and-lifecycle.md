# ADR 0001: Daemon Core via Unix Domain Socket & Systemd Lifecycle

* Status: accepted
* Date: 2026-09-10

## Context and Problem Statement

Daneel operates across multiple concurrent execution models: interactive Web visualizer, Terminal UI (TUI), CLI batch commands (`harvest`, `tidy`, `index`), and an MCP server queried by AI agents.

Previously, each component ran in isolation without a centralized coordinator. This caused two major problems:
1. **SQLite Database Locking**: Concurrent processes attempting to index or update notes triggered `SQLITE_BUSY` errors due to single-writer constraints.
2. **Disconnected UI State**: Background actions (such as a documentation harvest) could not broadcast updates to open graphical interfaces.

How should Daneel coordinate background execution, synchronize UI state, and manage long-running lifecycles on local developer machines?

## Decision Drivers

* Zero TCP port collisions with user applications or developer services.
* POSIX file permission security (`0700`) restricting access to the current OS user.
* Clean lifecycle management (foreground, detached background, and native Linux OS service).
* Automatic auto-spawn capabilities so users don't need manual orchestration.

## Considered Options

* **Option 1: TCP Socket on localhost (e.g. 127.0.0.1:43210)**
* **Option 2: Unix Domain Socket (`~/.local/state/daneel/daneel.sock`) with PID Lockfile**
* **Option 3: SQLite table polling / file system watchers**

## Decision Outcome

Chosen option: **Option 2: Unix Domain Socket with PID Lockfile**, because:
- Unix domain sockets avoid TCP port exhaustion, firewall prompts, and cross-user snooping on shared developer workstations.
- Standard XDG path (`$XDG_STATE_HOME/daneel/daneel.sock`) conforms to Linux/Unix conventions.
- Allows native integration with Linux `systemd --user` (`~/.config/systemd/user/daneel.service`) with automated 3-second restart and login activation.
- `daneel serve` can automatically initialize the socket server if offline, acting as master daemon seamlessly.

### Positive Consequences

* Single-writer coordination eliminates SQLite lock contention.
* Zero external network surface area or port allocation conflicts.
* Resilient PID file management (`daneel.pid`) prevents orphan zombie processes.
* Commands `daneel daemon [run|start|stop|status|service]` offer full operational control.

### Negative Consequences

* Unix domain sockets are specific to Unix/Linux/macOS; Windows would require named pipes (`\\.\pipe\...`) if native Windows support without WSL is added in the future.

## Pros and Cons of the Options

### Option 1: TCP Socket on localhost

* Good: Cross-platform by default.
* Bad: Risk of port collisions with other tools; requires port hunting logic; exposes local port to any process running under other local users.

### Option 2: Unix Domain Socket

* Good: Protected by filesystem permissions (`0700`); zero port collisions; highly performant IPC with streaming support.
* Good: Direct compatibility with `systemd --user` service units.
* Bad: Requires Unix-like environment (macOS/Linux/WSL).

### Option 3: SQLite polling / Watchers

* Good: No persistent process needed.
* Bad: High CPU/IO churn; latency on event delivery; cannot handle bidirectional request/response streaming.
