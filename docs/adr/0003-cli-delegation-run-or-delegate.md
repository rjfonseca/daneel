# ADR 0003: CLI Delegation (`RunOrDelegate`) & Visual Live Reload

* Status: accepted
* Date: 2026-09-10

## Context and Problem Statement

CLI operations such as `daneel harvest`, `daneel tidy`, and `daneel index` perform intensive I/O and SQLite database transactions. If the Web Galaxy is open while a harvest executes, two challenges emerge:
1. Concurrency locks on SQLite (`SQLITE_BUSY`).
2. Lack of live updates in the browser: user had to manually refresh the page to see newly ingested documentation nodes and connections.

How can CLI commands coordinate with the running system while maintaining full standalone functionality if no background services are active?

## Decision Drivers

* No hard daemon dependency: Daneel CLI must remain self-contained and fully functional without running a background service.
* Real-time visual reactivity: open graphical interfaces must reflect data changes instantly.
* Single-writer safety across parallel terminal invocations.

## Considered Options

* **Option 1: Hard Daemon Dependency**: CLI commands fail with "daemon not running" if the socket is absent.
* **Option 2: Standalone Execution Only**: CLI commands always write directly to the SQLite database without notifying the daemon.
* **Option 3: Transparent `RunOrDelegate` Pattern**: Probe socket; if online, delegate coordination and broadcast `ACTION_GALAXY_RELOAD`; if offline, run local engine directly.

## Decision Outcome

Chosen option: **Option 3: Transparent `RunOrDelegate` Pattern**, because:
- Preserves the Unix philosophy of standalone CLI utilities that work out-of-the-box in CI, scripts, or single-run executions.
- When the daemon is present, tasks coordinate smoothly and trigger `ACTION_GALAXY_RELOAD`, causing open Web Galaxy instances to fetch new nodes and links in real time.

### Positive Consequences

* Seamless user experience: terminal harvesting immediately renders new nodes in the browser visualizer.
* Zero regressions for automated CI/CD runs where no daemon runs.
* Centralized SQLite transaction management.

### Negative Consequences

* Slight socket probe overhead (~1-2ms) on CLI startup, negligible for human and batch operations.

## Pros and Cons of the Options

### Option 1: Hard Daemon Dependency

* Good: Strict architecture enforcement.
* Bad: Breaks CLI convenience, scripting in CI pipelines, and quick one-off queries.

### Option 3: Transparent `RunOrDelegate` Pattern

* Good: Best of both worlds—zero setup required for basic CLI use, maximum synchronization when running interactively.
* Bad: Requires dual-path testing (daemon online vs offline).
