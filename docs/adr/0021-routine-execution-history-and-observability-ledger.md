# ADR 0021: Routine Execution History, Trigger-Agnostic Observability Ledger, and Cockpit Timeline

* Status: accepted
* Date: 2026-09-22

## Context and Problem Statement

Daneel provides continuous background maintenance routines (`workspace`, `dream`, `harvest`, `doctor`, `tidy`, etc.) that keep the second-brain graph consistent, fresh, and well-indexed. These routines are dispatched from multiple heterogeneous triggers:
- **Linux Systemd User Timers** (`systemd --user`)
- **Crontab entries** (`crontab -l`)
- **Interactive Web Cockpit dispatches** (`ExecuteRoutine` ConnectRPC streaming)
- **Direct Terminal CLI / MCP tools** (`daneel routine exec` or automated AI agent dispatches)

Prior to this ADR, routine execution was strictly stateless:
1. **Zero Execution History**: No persistent execution ledger existed. If a scheduled routine ran at 08:00 via systemd or crontab, the operator had no visibility in the Web Cockpit regarding when it started, its execution duration, exit code, or failure details.
2. **Missing Frontend Telemetry Badges**: The protobuf definition `RoutineStatusItem` defined fields `last_run_timestamp` and `last_result`, but the Web Cockpit omitted them from the routine cards.
3. **Trigger Fragility**: Systemd status inspection failed silently when D-Bus user session sockets were missing or unconfigured, marking valid routines as `not installed`.

## Decision Drivers

* **Trigger-Agnostic Persistence**: Provide a single unified ledger that records execution runs regardless of whether the trigger was `systemd`, `cron`, `web_cockpit`, `cli`, or `mcp`.
* **Zero-CGO & Embeddable Storage**: Store run records in a lightweight SQLite database (`.daneel/cache/routine_runs.db` with resilient fallback to `config.XDGCacheDir()`) using `modernc.org/sqlite`.
* **Bounded Storage Footprint**: Automatically prune the ledger to keep a manageable rolling window of execution runs (e.g. latest 500 records).
* **Unified CLI Execution Wrapper**: Introduce `daneel routine exec <name> [--trigger=...]` which wraps command execution, captures start/completion timestamps, computes duration, catches exit codes and error outputs, and records the run in the persistent store.
* **Systemd Unit Template Integration**: Update `internal/routine/systemd.go` to invoke `daneel routine exec <name> --trigger=systemd` inside generated `.service` units.
* **Cockpit Observability Deck & Status Badges**: Expose `GetRoutineHistory` via ConnectRPC and render a dedicated execution timeline and relative timestamp status badges inside `CockpitModal.tsx`.
* **Strict Invariants**: Retain English UI, CGO-free Go backend, and zero-drift WebFS synchronization.

## Considered Options

* **Option 1: Rely Exclusively on Systemd Journals (`journalctl --user`)**:
  * *Disadvantages*: Does not work across platforms (macOS/Windows/containers), fails in environments without systemd user sessions, cannot track cron, CLI, or web dispatches, and requires complex unstructured log parsing.
* **Option 2: In-Memory Ring Buffer in Daemon**:
  * *Disadvantages*: Loses all execution history whenever the daemon restarts or when routines are triggered in headless standalone mode.
* **Option 3: Persistent SQLite Execution Ledger with CLI Wrapper and ConnectRPC History API**:
  * Unifies all execution entry points through a consistent persistence layer.
  * Ensures resilience whether routines are launched via systemd, cron, CLI, or web streaming.

## Decision Outcome

Chosen option: **Option 3: Persistent SQLite Execution Ledger with CLI Wrapper and ConnectRPC History API**.

### Architectural Topology

```
 ┌────────────────────────────────────────────────────────────────────────┐
 │                           Trigger Sources                              │
 │   [ Systemd Timer ]     [ Crontab ]     [ Web Cockpit ]     [ CLI/MCP ]│
 └───────────┬──────────────────┬─────────────────┬───────────────┬───────┘
             │                  │                 │               │
             ▼                  ▼                 ▼               ▼
      daneel routine exec  daneel routine exec    serve.go      CLI Runner
             │                  │                 │               │
             └──────────────────┼─────────────────┴───────────────┘
                                │
                                ▼
                   routine.RecordRun(ctx, run)
                                │
                                ▼
              SQLite Ledger: `routine_runs` table
              (.daneel/cache/routine_runs.db)
                                │
                                ▼
             ConnectRPC: `GetRoutineHistory()`
                                │
                                ▼
              Cockpit: Timeline & Status Badges
```

### Key Technical Specifications

1. **SQLite Storage Schema (`internal/routine/store.go`)**:
   - Table `routine_runs`:
     - `id` (TEXT PRIMARY KEY)
     - `routine_name` (TEXT NOT NULL)
     - `started_at` (INTEGER NOT NULL, Unix timestamp in seconds)
     - `completed_at` (INTEGER NOT NULL, Unix timestamp in seconds)
     - `duration_ms` (INTEGER NOT NULL, elapsed milliseconds)
     - `trigger_source` (TEXT NOT NULL: `systemd`, `cron`, `web_cockpit`, `cli`, `mcp`)
     - `exit_code` (INTEGER NOT NULL: `0` for success, non-zero for error)
     - `status` (TEXT NOT NULL: `success`, `failed`, `running`, `cancelled`)
     - `summary` (TEXT)
     - `error_message` (TEXT)
   - Indexes on `started_at DESC` and `(routine_name, started_at DESC)`.
   - Automatic retention pruning keeping the latest 500 execution entries.

2. **CLI Subcommand Wrapper (`daneel routine exec`)**:
   - Executes routine command with its configured arguments.
   - Measures execution duration and captures output lines and exit codes.
   - Persists execution metadata into the SQLite ledger upon completion.
   - Systemd units generated by `internal/routine/systemd.go` use `ExecStart={{.Executable}} routine exec {{.Name}} --trigger=systemd`.

3. **ConnectRPC Protocol Contract (`api/v1/graph.proto`)**:
   - Messages: `RoutineRunItem`, `GetRoutineHistoryRequest`, `GetRoutineHistoryResponse`.
   - RPC: `rpc GetRoutineHistory(GetRoutineHistoryRequest) returns (GetRoutineHistoryResponse)`.
   - Handler in `cmd/serve.go` exposes query filtering by `routine_name` and paging `limit`.
   - `ExecuteRoutine` records web cockpit streaming executions directly into the ledger with `trigger_source = "web_cockpit"`.
   - `GetRoutinesOverview` enriches routine status cards with real `last_run_timestamp` and `last_result` from the store.

4. **Web Cockpit UI Experience (`CockpitModal.tsx`)**:
   - Routine list cards display `Last: X ago` with emerald/rose status badges and next scheduled trigger.
   - Dedicated Recent Runs / Execution Timeline view in the Actions Deck showing status, routine name, trigger source badge, start time, duration, and expandable error logs.
   - Automatic reactive refresh of recent runs upon routine execution completion.

## Consequences

### Positive
* Comprehensive operational visibility into all background tasks and automated maintenance.
* Complete parity across systemd, cron, web cockpit, and CLI triggers.
* Immediate feedback in the Web Cockpit when background tasks fail or encounter errors.
* Preserves zero-CGO and cross-platform compatibility.

### Negative / Trade-Offs
* Small additional disk footprint for `.daneel/cache/routine_runs.db` (mitigated by automated 500-record rolling retention).
* Subprocess invocation in `daneel routine exec` introduces minimal (<10ms) wrapper overhead.
