# ADR 0013: Web Cockpit Action Deck ConnectRPC Streaming & Agent Session Mining

* Status: accepted
* Date: 2026-09-14

## Context and Problem Statement

Daneel provides two primary user interfaces:
1. **Terminal UI (`internal/tui/` - Bubble Tea):** Includes a rich, deterministic Action Deck (`skills_view.go`) capable of executing routines (`tidy`, `doctor`, `index`, `guard`, `dossier`, `workspace`, `inbox`, `log`) and orchestrating AI skills.
2. **Web Galaxy Visualizer (`cmd/serve.go` + `web/`):** A high-performance WebGL 3D/cosmic graph explorer (Sigma.js + Graphology), but historically read-only with no mechanism to trigger routines, view execution logs, or inspect operational telemetry.

Additionally, AI coding agents (such as Google DeepMind Antigravity) log structured session transcripts in `~/.gemini/antigravity-cli/brain/<conversation-id>/.system_generated/logs/transcript.jsonl`. Crucial user corrections, conventions, and repetitive multi-step prompts remained dormant in these logs without being mined into the Second Brain's persistent memory (`meta/guias/aprendizados-agente.md`) or distilled into reusable agent skills (`SKILL.md`).

## Decision Drivers

* Feature parity between the Web Visualizer and Terminal UI Action Deck.
* Real-time streaming of stdout/stderr output lines from headless routines.
* Continuous cognitive learning loop via deterministic session parsing without wasteful LLM token expenditure.
* Strict adherence to pure Go (CGO-free) and ConnectRPC standards.

## Considered Options

* **Option 1: Polling and Webhook Daemon Execution**: Web client triggers HTTP POST and periodically polls SQLite or a memory queue for execution logs.
* **Option 2: ConnectRPC Server Streaming & Deterministic Session Miner Package**:
  - Extend `GraphService` with server-streaming RPC `ExecuteRoutine` and queries `GetRoutinesOverview` and `GetStaffTelemetry`.
  - Create pure Go `internal/auditor` package with `SessionMiner` and CLI command `daneel audit sessions`.

## Decision Outcome

Chosen option: **Option 2: ConnectRPC Server Streaming & Deterministic Session Miner Package**, because:
- Server streaming via ConnectRPC (`stream RoutineOutputEvent`) works seamlessly across HTTP/1.1 (via chunked transfer), HTTP/2, and gRPC-web without additional WebSocket overhead.
- Centralizing routine execution in `GraphServer` enables real-time terminal-like streaming to the Web frontend.
- `SessionMiner` deterministically parses Antigravity transcripts, detects explicit user corrections using targeted regexes, and aggregates repetitive imperative workflows into proposed `SKILL.md` candidates.

### Positive Consequences

* Web frontend gains full operational parity with TUI's deterministic action deck.
* Real-time terminal log viewer support via ConnectRPC streaming.
* Automated knowledge synthesis from daily agent interactions into `meta/guias/aprendizados-agente.md`.
* New CLI command `daneel audit sessions [--since=48h] [--dry-run] [--mine-skills] [--evals] [--apply]`.
