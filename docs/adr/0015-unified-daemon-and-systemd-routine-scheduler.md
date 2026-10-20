# ADR 0015: Unified Daemon Multi-Listener and Systemd Routine Scheduler

* Status: accepted
* Date: 2026-09-17

## Context and Problem Statement

Daneel previously managed persistent operations across separate, uncoordinated runtimes:
1. `daneel daemon run`: Managed a Unix Domain Socket (`~/.local/state/daneel/daneel.sock`) for IPC, active surface tracking, and MCP coordination.
2. `daneel serve`: Started a standalone HTTP server hosting ConnectRPC and the embedded WebFS Galaxy Visualizer, holding a separate connection pool to SQLite FTS5 (`vault_index.db`).
3. Scheduled maintenance routines (`tidy`, `harvest`, `doctor`): Were invoked manually or required external, unmanaged crontab entries without fault isolation, cgroup constraints, or unified journal logging.

This dual-process architecture introduced potential file lock contention on SQLite, duplicate system resource consumption, and manual operational overhead when provisioning services.

## Decision Drivers

* **Process Unification**: Provide a single daemon process (`daneel daemon run --serve`) that coordinates both the Unix domain socket and the HTTP web visualizer while sharing in-memory search structures.
* **Ports & Adapters Architecture**: Decouple routine scheduling via an abstract interface (`SchedulerPort`) to allow clean unit testing and future schedulers (launchd for macOS, Windows Task Scheduler) while delivering a production-grade Linux `systemd --user` adapter.
* **Operating System Isolation**: Execute batch maintenance tasks (`harvest`, `doctor`, `tidy`) as isolated OS processes (`Type=oneshot`) with cgroup limits, timeout protection, and persistent journal logs (`journalctl`).
* **Turnkey Provisioning**: Provide a unified one-liner command (`daneel daemon service install --with-serve --with-routines`) to install the daemon service and all scheduled routine timers in a single invocation.
* **Concurrency Hardening**: Eliminate database lock contention through WAL mode and `busy_timeout=10000`.

## Considered Options

* **Option 1: Internal Goroutine Cron Scheduler inside Daemon**: Run all routines as background goroutines inside the daemon process.
  * *Disadvantages*: Crashes or high memory usage in one routine (e.g. heavy Git/Jira scraping) could crash the daemon; logs are intertwined; lacks systemd cgroups, system reboot survival, or native `journalctl` integration.
* **Option 2: Standalone Shell Scripts with crontab**: Generate crontab lines via CLI.
  * *Disadvantages*: Crontab lacks user service isolation, unit dependencies, structured systemd status introspection, and journalctl integration.
* **Option 3: Unified Multi-Listener Daemon with Native Systemd User Units & Timers**:
  * Implement coordinated multi-listener in `internal/daemon/socket.go`.
  * Build `internal/routine` using Ports & Adapters with `SystemdAdapter` generating `daneel-<name>.service` and `daneel-<name>.timer`.
  * Support friendly cron parsing to systemd `OnCalendar`.
  * Provide `daneel routine` CLI and turnkey `daemon service install --with-serve --with-routines`.

## Decision Outcome

Chosen option: **Option 3: Unified Multi-Listener Daemon with Native Systemd User Units & Timers**.

### Architectural Layers

1. **Domain & Ports Layer (`internal/routine/port.go`)**:
   - `Routine`: Domain model defining Name, Command, Args, Schedule, Description, and Enabled status.
   - `RoutineStatus`: Operational state containing Enabled, Active, LastTrigger, NextTrigger, and LastResult.
   - `SchedulerPort`: Interface specifying `Install`, `Uninstall`, `Enable`, `Disable`, `List`, `Status`, `RunNow`, and `Logs`.

2. **Cron to OnCalendar Parser (`internal/routine/cron.go`)**:
   - Deterministic parser translating friendly presets (`hourly`, `daily`, `weekly`, `monthly`) and standard 5-part cron expressions (`0 */6 * * *`, `*/30 * * * *`, `0 9 * * 1-5`) into valid systemd `OnCalendar` strings.

3. **Systemd Adapter (`internal/routine/systemd.go`)**:
   - `SystemdAdapter`: Implements `SchedulerPort` managing unit files under `~/.config/systemd/user/`.
   - Generates `daneel-<name>.service` (`Type=oneshot`, `StandardOutput=journal`) and `daneel-<name>.timer` (`Persistent=true`, `WantedBy=timers.target`).
   - Abstracted `CommandRunner` facilitates 100% hermetic unit testing without requiring a live systemd bus.

4. **Multi-Listener Daemon (`internal/daemon/socket.go` & `cmd/daemon.go`)**:
   - `daemon.Server` supports an attached `*http.Server` via `SetHTTPServer`.
   - Mutual graceful shutdown drains HTTP connections within 3 seconds and unlinks the Unix socket on SIGINT/SIGTERM.
   - `daneel daemon run --serve --port 4321` shares a single `search.Engine` instance in memory.

5. **CLI & Operational Surface (`cmd/routine.go` & `cmd/daemon.go`)**:
   - `daneel routine [list|install|uninstall|enable|disable|run|logs|status]`.
   - `daneel daemon service install --with-serve --with-routines`.

6. **SQLite Concurrency Hardening (`internal/search/engine.go`)**:
   - Added `&_pragma=busy_timeout(10000)` to DSN, allowing automatic retry on concurrent writes during oneshot routine execution.

## Consequences

### Positive
* Single resident background process handles both socket IPC and web visualization.
* Zero multi-process SQLite lock contention when running web visualizer and daemon together.
* Robust fault isolation: failing routines do not crash the core daemon.
* Native system-level logs via `journalctl --user -u daneel-<routine>.service`.
* Turnkey setup for new developers via `daneel daemon service install --with-serve --with-routines`.

### Negative / Trade-offs
* `systemd --user` requires Linux; non-Linux hosts or containerized environments without dbus must use fallback CLI execution or alternative adapters in the future.
