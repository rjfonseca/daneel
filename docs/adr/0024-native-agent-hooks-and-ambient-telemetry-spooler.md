# ADR 0024: Native Agent Hooks, Ambient Telemetry Spooler, and Cross-Agent Continuity

* Status: accepted
* Date: 2026-10-05

## Context and Problem Statement

As artificial intelligence agents (e.g., Claude Code, Antigravity CLI, Cursor) participate heavily in daily engineering workflows across multiple codebases and vaults, capturing their operational context (tool calls, edits, bash commands, reasoning trajectories, and session lifecycles) is essential for second-brain continuity.

Previously, prototype setups attempted to rely on external standalone sidecars (such as `ai-memory`, a Rust-based daemon) or proposed opening secondary TCP ports (e.g., `127.0.0.1:49375`) requiring heavy shell scripts and background listeners. This introduced:
1. **Architectural Duplication**: Maintaining a separate external database, Rust runtime, and independent daemon lifecycle outside Daneel's unified supervisor.
2. **Network & Port Fragility**: TCP port collisions, firewall popups, and localhost binding security risks.
3. **Shell Fragility**: Dense, multi-hundred-line shell scripts parsing JSON via divergent `sed`/`awk`/`jq` implementations across Linux and macOS.
4. **Disconnection from Visual & Terminal Telemetry**: Captured events were isolated in external stores, unavailable to Daneel's real-time Web Galaxy Visualizer, Cosmic Photons, and Bubble Tea Terminal User Interface (TUI).

## Decision Drivers

* **100% Native Go Integration**: Zero external daemon sidecars or Rust dependencies; all hook processing and spooling must reside natively within Daneel.
* **Socket Reuse & Portless Ingress**: Reuse Daneel's existing Unix Domain Socket (`~/.local/state/daneel/daneel.sock`) via established IPC dispatchers, eliminating unnecessary TCP ports.
* **Offline-First Resilience**: If the Daneel background daemon is stopped or suspended, the hook invocation must gracefully fall back to appending directly to local disk spool storage with zero dropped events and zero perceptible latency (< 10ms).
* **Thin-Agent Invocation Interface**: Agent lifecycle hook configuration must point directly to `daneel hook capture <event>` reading from `stdin`, eliminating complex shell wrappers.
* **Ambient Telemetry & Visual Resonance**: Event ingest must feed Daneel's event `Hub` (`s.hub.PublishUIEvent`), powering live cosmic photon streams in the Web Galaxy, a dedicated Seldon Perimeter satellite, and real-time session tracking in the TUI.
* **Phased Compilation & Dream Cycle Feeding**: The raw session buffer (`~/.cache/daneel/spool/<session_id>.jsonl`) will feed downstream Raw-to-Inbox Markdown compilation (`inbox/sessions/`), inter-agent handoff, and the Cognitive REM Dream Cycle (`daneel dream`).

## Considered Options

* **Option 1: External `ai-memory` Sidecar**:
  * *Disadvantages*: Requires separate installation, Rust toolchain or binaries, separate database schemas, and diverges from Daneel's governance and ADR 0001/0015 lifecycle.
* **Option 2: Standalone Secondary HTTP Server on TCP Port 49375**:
  * *Disadvantages*: Introduces multiple HTTP servers in the daemon, port collision risks, and firewall friction; breaks Daneel's unified Unix socket architecture.
* **Option 3: Dedicated HTTP-over-Unix-Socket (`daneel-http.sock`) with `curl` Shell Wrappers**:
  * *Disadvantages*: Retains reliance on `curl --unix-socket` flags in agent environments, introduces a secondary socket file, and fails if the daemon is offline.
* **Option 4: 100% Native Daneel CLI + Unified Unix Socket IPC + Offline Spool Fallback (Chosen)**:
  * Hooks invoke `daneel hook capture <event>`.
  * If the daemon is online: communicates via native IPC (`daneel.sock`) and publishes ambient telemetry to the `Hub`.
  * If the daemon is offline: falls back automatically to direct session file append in `~/.cache/daneel/spool/`.

## Decision Outcome

Chosen option: **Option 4: 100% Native Daneel CLI + Unified Unix Socket IPC + Offline Spool Fallback**.

### Architectural Layers

#### 1. CLI Hook Ingress (`cmd/hook.go`)
- **`daneel hook capture <event>`**:
  - Accepts raw event JSON payloads via `stdin` or CLI flags (`--event`, `--agent`, `--session`, `--cwd`).
  - Attempts immediate IPC dispatch via `daemon.NewClient()`.
  - On IPC connection failure or timeout (< 50ms), automatically triggers the local file-spooler fallback.
- **`daneel hook handoff [--cwd <path>]`**:
  - In Phase 1: Exits cleanly with status `0` and empty stdout (safe no-op preventing prompt pollution).
  - In Phase 2: Queries compiled session state to output structured handoff context for new agent sessions.
- **`daneel hook status`**:
  - Inspects `~/.cache/daneel/spool/`, outputting active session IDs, uncompiled event counts, and daemon connectivity state.

#### 2. Spooler Core & Cascading Normalizer (`internal/spooler/`)
- **Partitioned Storage**:
  - Events are appended to `filepath.Join(config.XDGCacheDir(), "spool", "<session_id>.jsonl")`.
  - Uses `os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600` with per-session mutex locking for concurrency safety.
- **Smart Cascading Normalizer**:
  - Extracts `session_id`: `X-Session-ID` header > `conversationId` (Antigravity protojson) > `sessionId` / `session_id` (Claude Code / Cursor) > CLI flag > deterministic directory-date hash fallback.
  - Extracts `cwd`: explicit flag > `cwd` JSON field > `workspacePaths[0]` > `workspace_roots[0]` > `os.Getwd()`.
  - Git Root Resolution: Traverses upward from `cwd` to identify `.git` root and correlate against configured vaults.

#### 3. Daemon IPC Dispatch & Hub Telemetry (`internal/daemon/`)
- **Methods in IPC Dispatcher (`internal/daemon/socket.go`)**:
  - `capture_hook_event`: Ingests `HookEventPayload`, writes to the daemon's managed `Spooler`, and publishes a `graphv1.UIEvent` to `s.hub`.
  - `get_hook_handoff`: Retrieves session resume context.
- **Ambient Event Publishing**:
  - Emits `UIEvent` with action `UI_ACTION_TYPE_NOTIFICATION` containing metadata (`agent`, `event`, `cwd`, `timestamp`).

#### 4. Terminal User Interface (TUI) Agent Monitor (`internal/tui/`)
- **Activity & Spool View (`internal/tui/hooks_view.go`)**:
  - Interactive Bubble Tea model displaying active agent sessions, real-time tool call streams, and uncompiled spool metrics.
  - Accessible via global keybinding (`H`).
- **Dashboard Telemetry (`internal/tui/dashboard_view.go`)**:
  - Status pill displaying live active agent count and uncompiled spool size.

#### 5. Web Galaxy Visualizer, Seldon Satellite, and Cosmic Photons (`web/`)
- **Seldon Perimeter Satellite ("Hooks Telemetry")**:
  - Positioned on the orbital ring ($r \approx 7000$) in `web/src/lib/orbitalRings.ts`.
  - Glows active cyan/green when an agent session is active.
- **Cosmic Photon Particle Stream**:
  - Upon receiving a hook `UIEvent` via ConnectRPC streaming, triggers a quantum particle animation from the Hooks Satellite directly to the project's sector/star node in `web/src/lib/trafficSimulation.ts`.
- **HUD Agent Presence Card (`web/src/components/hud/AgentPresenceCard.tsx`)**:
  - Non-intrusive collapsible HUD card displaying active agent, current project, and recent tool actions in concise English.

#### 6. Downstream Raw-to-Inbox Compilation & Cognitive Dream Cycle
- The partitioned `.jsonl` files in `~/.cache/daneel/spool/` serve as an immutable, low-overhead input stream for:
  - **Phase 2 (Compiler)**: Synthesis into `inbox/sessions/YYYY-MM-DD-HH-MM-<agent>.md` wrapped in `<untrusted_content>`.
  - **Cognitive Dream Cycle (`daneel dream`)**: Zettelkasten triage, error pattern detection, and automated architectural rule extraction.

---

## Consequences

### Positive
* **Zero Additional Ports**: Keeps network attack surface minimal; zero firewall or port-binding issues.
* **Fault-Tolerant by Design**: Daemon restart, crash, or freeze never prevents agents from executing; offline fallback captures 100% of telemetry.
* **Sub-Millisecond Overhead**: Direct socket write or fast local file append introduces negligible latency (< 3-5ms), invisible to agents and tools.
* **Unified Visual & Terminal Experience**: Brings agent actions into the cosmic visualizer and TUI dashboard without custom bridges.

### Negative / Trade-offs
* **Spool Directory Management**: Requires automatic compaction or pruning of old `.jsonl` files once compiled into the vault inbox to prevent cache bloat over months of continuous development.
