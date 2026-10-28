# ADR 0037: FinOps Token Cost Attribution and Multi-Surface Operational Telemetry

* Status: accepted
* Date: 2026-10-28

## Context and Problem Statement

Daneel coordinates autonomous background maintenance routines (`tidy`, `harvest`, `doctor`, `workspace`, and the Cognitive REM `dream` cycle), exposes native Model Context Protocol (MCP) primitives to external AI coding agents, and ingests execution telemetry from agent frameworks (such as Antigravity CLI, Claude Code, and autonomous subagent loops).

Prior to this ADR, operational tracking suffered from significant visibility blindspots:
1. **Unattributed LLM Spend & Token Consumption**: Autonomous Dream Cycle executions, knowledge synthesis runs, and interactive agent sessions consumed millions of LLM prompt and completion tokens without a unified ledger attributing token counts or calculating estimated dollar costs.
2. **Untracked Recursive Subagent Hierarchies**: Modern agentic harnesses dispatch hierarchical subagents (e.g. parent orchestrator invoking child agents via `invoke_subagent` or parallel workers). Raw logs (`transcript.jsonl` in Antigravity or `.jsonl` session streams in Claude Code) record isolated messages without correlating cost, token usage, and latency back up to parent tasks.
3. **Missing Granular CLI and MCP Primitive Telemetry**: High-frequency MCP primitives (`resolve_note`, `search_vault`, `person_briefing`, `daneel guard`, etc.) lacked unified latency, duration, and error-rate recording, making performance regressions and tool call bottlenecks invisible.
4. **Siloed Presentation Surfaces**: Existing telemetry was split between disparate files and partial views, lacking a cohesive presentation across the Terminal CLI, Bubble Tea TUI, Web Cockpit ConnectRPC deck, and the Vault Markdown Dashboard.

## Decision Drivers

* **3-Pillar Unified Telemetry Architecture**:
  1. *Enhanced Routine & Dream Cycle Ledger*: Track items processed, granular token breakdown (`input_tokens`, `cache_read_tokens`, `output_tokens`), model identity, session ID, and computed financial cost (`estimated_cost_usd`) in SQLite.
  2. *AI Session & Recursive Subagent FinOps Parser*: Parse session artifacts across Antigravity transcripts, Claude Code logs, and ambient spooler files, recursively reconstructing parent-to-child agent trees and attributing cumulative costs.
  3. *Command & MCP Primitive Usage Ledger*: Record invocation latency, execution parameters, exit codes, and caller identity for all CLI commands and MCP tool calls.
* **Configurable Model Pricing Table**: Support customizable per-model rates for input, prompt cache reads, and generation tokens in `config.yaml` / `.daneel.yaml` with built-in default rates for major frontier and efficient LLM models.
* **Multi-Surface Observability**: Surface FinOps and operational telemetry consistently across:
  - Terminal CLI (`daneel telemetry` subcommands)
  - Interactive Bubble Tea TUI (`ViewTelemetry` on hotkey `8`)
  - Web Cockpit ConnectRPC service (`GetOperationalTelemetry`) and React FinOps Deck
  - Automated Markdown Dashboard compilation (`## 5. FinOps & Operational Telemetry`)
* **Zero-CGO & Embeddable Storage Invariant**: Retain zero-CGO SQLite (`modernc.org/sqlite`) stored in `.daneel/cache/routine_runs.db` with standard fallback to XDG cache directories.
* **Open-Source Neutrality & Data Privacy**: Pure English terminology, zero corporate or personal data leaks, and zero mandatory cloud telemetry sidecars.

## Considered Options

* **Option 1: Rely on Cloud Provider Billing APIs**:
  * *Disadvantages*: Provider dashboards are vendor-specific, delayed by several hours, require external API secrets with administrative billing permissions, and cannot correlate token consumption with specific local routines, MCP primitives, or subagent tasks.
* **Option 2: Heavyweight External Monitoring Sidecars (Prometheus / Grafana / OpenTelemetry Collector)**:
  * *Disadvantages*: Violates Daneel's self-contained binary principle, introduces steep resource overhead, and adds complex orchestration requirements for single-user desktop vaults.
* **Option 3: Embedded SQLite FinOps Ledger and Multi-Surface Telemetry Engine (Chosen)**:
  * Leverages existing SQLite infrastructure (`routine_runs.db`) and IPC socket.
  * Captures real-time token counts, recursive subagent trees, command durations, and MCP primitive metrics locally with sub-millisecond overhead.

## Decision Outcome

Chosen option: **Option 3: Embedded SQLite FinOps Ledger and Multi-Surface Telemetry Engine**.

### Architectural Topology

```
 ┌─────────────────────────────────────────────────────────────────────────────┐
 │                               Telemetry Ingress                             │
 │  [ Scheduled Routines ]   [ Agent Transcripts ]   [ CLI / MCP Primitives ]  │
 │  (dream, harvest, tidy)   (Antigravity / Claude)  (search, resolve, prep)   │
 └─────────────┬─────────────────────┬──────────────────────────┬──────────────┘
               │                     │                          │
               ▼                     ▼                          ▼
       routine.RecordRun      internal/telemetry          RecordCommand /
      (Token & Cost Ledger)  (Recursive Tree Parser)     RecordMCPUsage
               │                     │                          │
               └─────────────────────┼──────────────────────────┘
                                     │
                                     ▼
                     SQLite Storage: `routine_runs.db`
           ┌──────────────────────────────────────────────────┐
           │ • routine_runs      (tokens, model, cost, stats) │
           │ • session_telemetry (sessions, subagent trees)   │
           │ • command_telemetry (CLI & MCP tool latency)     │
           └─────────────────────────┬────────────────────────┘
                                     │
               ┌─────────────────────┼─────────────────────┐
               ▼                     ▼                     ▼
          Terminal CLI          Bubble Tea TUI        ConnectRPC & Web
       `daneel telemetry`     `ViewTelemetry` (8)   `GetOperationalTelemetry`
               │                                           │
               └─────────────────────┬─────────────────────┘
                                     ▼
                        Markdown Vault Dashboard
                   (Section 5: FinOps & Telemetry)
```

### Detailed Component Specifications

#### 1. Pillar 1: Enhanced Routine & Dream Cycle Execution Ledger
The SQLite table `routine_runs` in `.daneel/cache/routine_runs.db` (or `~/.cache/daneel/routine_runs.db`) is migrated to include token metrics and operational statistics:
* `items_processed` (INTEGER DEFAULT 0): Quantity of notes, links, or repositories operated upon.
* `input_tokens` (INTEGER DEFAULT 0): Raw input prompt tokens consumed.
* `cache_read_tokens` (INTEGER DEFAULT 0): Cached prompt tokens read at discount.
* `output_tokens` (INTEGER DEFAULT 0): Completion tokens generated.
* `estimated_cost_usd` (REAL DEFAULT 0.0): Monetary cost computed via model pricing table.
* `model` (TEXT): Identifier of the LLM model utilized (e.g. `gemini-pro`, `claude-sonnet`).
* `session_id` (TEXT): Correlated agent session ID when triggered interactively or via agent loops.

#### 2. Pillar 2: AI Session & Recursive Subagent FinOps Parser (`internal/telemetry`)
A unified log parser resolves agent activity across three log formats:
* **Antigravity Transcripts**: `transcript.jsonl` files storing structured user, model, and tool call turns.
* **Claude Code Logs**: `.jsonl` session files tracking tool executions and token usage.
* **Ambient Spooler Files**: `~/.cache/daneel/spool/<session_id>.jsonl` streaming real-time hooks.

**Recursive Subagent Tree Resolution**:
When a parent agent dispatches a subagent via `invoke_subagent` or `dispatch_agent`:
1. The parser identifies the child session identifier from the invocation parameters.
2. The child transcript is ingested recursively, maintaining parent-child node relationships.
3. Total input, cached, and output tokens from all descendants are rolled up to the root session.
4. Total execution cost in USD is calculated by mapping each model name against the `FinOpsConfig` pricing table.

#### 3. Pillar 3: Command & MCP Primitive Usage Ledger
Two dedicated tables record high-frequency command and tool invocations:
* `command_telemetry`:
  - `id` (TEXT PRIMARY KEY)
  - `command_name` (TEXT NOT NULL, e.g. `search`, `harvest`, `doctor`)
  - `executed_at` (INTEGER NOT NULL, Unix timestamp in seconds)
  - `duration_ms` (INTEGER NOT NULL, elapsed execution milliseconds)
  - `exit_code` (INTEGER NOT NULL)
  - `caller` (TEXT, e.g. `cli`, `daemon`, `mcp`, `subagent`)
  - `metadata_json` (TEXT)
* `session_telemetry`:
  - `session_id` (TEXT PRIMARY KEY)
  - `parent_session_id` (TEXT, nullable for root sessions)
  - `agent_type` (TEXT NOT NULL, e.g. `antigravity`, `claude_code`, `custom`)
  - `started_at` (INTEGER NOT NULL)
  - `completed_at` (INTEGER NOT NULL)
  - `total_input_tokens` (INTEGER NOT NULL)
  - `total_cache_read_tokens` (INTEGER NOT NULL)
  - `total_output_tokens` (INTEGER NOT NULL)
  - `total_cost_usd` (REAL NOT NULL)
  - `subagent_count` (INTEGER NOT NULL)

#### 4. Model Pricing Table & Budget Configuration (`internal/config`)
`TelemetryConfig` is enhanced with a typed `FinOps FinOpsConfig` block:
```yaml
telemetry:
  finops:
    daily_budget_usd: 15.00
    monthly_budget_usd: 300.00
    default_model: "gemini-pro"
    pricing:
      "gemini-pro":
        input_per_1m: 1.25
        cache_read_per_1m: 0.3125
        output_per_1m: 5.00
      "gemini-flash":
        input_per_1m: 0.15
        cache_read_per_1m: 0.0375
        output_per_1m: 0.60
      "gemini-flash-lite":
        input_per_1m: 0.075
        cache_read_per_1m: 0.01875
        output_per_1m: 0.30
      "claude-opus":
        input_per_1m: 15.00
        cache_read_per_1m: 1.50
        output_per_1m: 75.00
      "claude-sonnet":
        input_per_1m: 3.00
        cache_read_per_1m: 0.30
        output_per_1m: 15.00
      "claude-haiku":
        input_per_1m: 0.80
        cache_read_per_1m: 0.08
        output_per_1m: 4.00
      "gpt-codex":
        input_per_1m: 2.50
        cache_read_per_1m: 1.25
        output_per_1m: 10.00
```
Helper methods provide case-insensitive model normalization, alias resolution, and cost computation.

#### 5. Multi-Surface Observability Interfaces
* **CLI (`daneel telemetry`)**:
  - `daneel telemetry [--days=7] [--json]`
  - `daneel telemetry --finops` (or `--sessions`)
  - `daneel telemetry --routines`
  - `daneel telemetry --primitives` (or `--commands`)
* **Bubble Tea TUI (`ViewTelemetry`)**:
  - Tab 8 (`8: 📈 Telemetry`) renders active spend vs. daily/monthly budget bars, model cost breakdowns, and recent subagent trees.
* **ConnectRPC Service & Web Cockpit**:
  - `GetOperationalTelemetry` stream/RPC delivers aggregated metrics, model breakdown, and recent session cards to the Web Cockpit FinOps Deck.
* **Vault Markdown Dashboard (`dashboard-metricas.md`)**:
  - Automatically compiles Section 5 summarizing 7-day rolling spend, model token distributions, top invoked MCP tools, and average execution latencies.

## Consequences

### Positive

* **Total Financial & Operational Transparency**: Complete accounting of all token spend, recursive agent loops, and execution latencies across the knowledge vault.
* **Recursive Attribution**: Accurate attribution of runaway subagent trees and high-frequency tool loops.
* **Zero External Dependencies**: Pure CGO-free Go SQLite implementation runs seamlessly in offline and constrained environments.
* **Cross-Surface Consistency**: Identical numbers and metrics presented in CLI, TUI, Web Cockpit, and Markdown notes.

### Negative / Trade-Offs

* **Parsing Overhead**: Inspecting deep subagent trees across multiple large JSONL files requires streaming parsers to avoid high memory spikes.
* **Pricing Currency Volatility**: Model pricing tables require periodic updates as model providers adjust pricing tiers.
