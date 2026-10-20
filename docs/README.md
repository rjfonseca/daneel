# Daneel Documentation

Welcome to the **Daneel** documentation hub. Daneel is an autonomous cognitive runtime, knowledge graph engine, and ambient AI assistant for personal and organizational Second Brains.

## Progressive Disclosure Architecture

Documentation is organized modularly so human developers and AI agents can load only the exact context required for their task:

| Topic | Document | Description |
|---|---|---|
| **System Overview** | [`docs/architecture/overview.md`](architecture/overview.md) | High-level system architecture, data stores, and subsystem layout |
| **Search & Storage** | [`docs/architecture/search-and-storage.md`](architecture/search-and-storage.md) | SQLite FTS5 engine, BM25 scoring, relational schema, and indexing |
| **Security Guardrails** | [`docs/architecture/security-guardrails.md`](architecture/security-guardrails.md) | Asimov's Four Laws of Robotics audit engine and leak prevention |
| **Coordinates** | [`docs/architecture/coordinate-spaces.md`](architecture/coordinate-spaces.md) | Graphology, Sigma WebGL normalized space, and DOM screen space math |
| **Daemon** | [`docs/daemon/lifecycle.md`](daemon/lifecycle.md) | Unix socket core, PID management, and `systemd --user` lifecycle |
| **Daemon Debugging** | [`docs/daemon/debugging.md`](daemon/debugging.md) | Diagnostic workflows, socket troubleshooting, and log inspection |
| **Spatial Context** | [`docs/ambient-intelligence/spatial-context.md`](ambient-intelligence/spatial-context.md) | ActiveContext protocol, ConnectRPC streaming, and viewport sync |
| **MCP Tools** | [`docs/ambient-intelligence/mcp-tools.md`](ambient-intelligence/mcp-tools.md) | Unified `ui_*` Model Context Protocol tools and offline fallbacks |
| **Cognitive Dream Cycle** | [`docs/cognitive/dream-cycle.md`](cognitive/dream-cycle.md) | Autonomous REM consolidation, Zettelkasten triage, and feedback loop |
| **Configuration** | [`docs/configuration/schema.md`](configuration/schema.md) | Multi-vault hierarchy, `.daneel.yaml` specification, and XDG layout |
| **Operations Suite** | [`docs/operations/telemetry-and-briefing.md`](operations/telemetry-and-briefing.md) | Time tracking, performance dossiers, briefings, cloner, and scanner |
| **Terminal UI (TUI)** | [`docs/tui/terminal-interface.md`](tui/terminal-interface.md) | Bubble Tea interactive terminal interface, keybindings, and views |
| **CLI Reference** | [`docs/cli/reference.md`](cli/reference.md) | Complete 23-command CLI inventory, flags, and operational parameters |
| **CLI Delegation** | [`docs/cli/delegation.md`](cli/delegation.md) | `RunOrDelegate` pattern, single-writer SQLite, and live reloads |
| **Web Visualizer** | [`docs/web/galaxy-visualizer.md`](web/galaxy-visualizer.md) | Sigma.js, WebGL stellar shaders, ForceAtlas2, and camera rules |
| **Astrophysics LOD** | [`docs/web/lod-and-astrophysics.md`](web/lod-and-astrophysics.md) | 8-class stellar taxonomy and dynamic LOD edge filtering |
| **Harvesters** | [`docs/harvester/pipeline.md`](harvester/pipeline.md) | GitLab, GitHub, Google Drive, and Jira document scrapers |
| **Getting Started** | [`docs/development/getting-started.md`](development/getting-started.md) | Toolchain setup, build recipes, and local development |
| **Testing & Quality** | [`docs/development/quality-and-testing.md`](development/quality-and-testing.md) | Quality gates, Vitest, Playwright, and Go tests |
| **Decisions (ADRs)** | [`docs/adr/README.md`](adr/README.md) | Architectural Decision Records (MADR format) |

## Quick Navigation

- For AI Agents: Start with [`AGENTS.md`](../AGENTS.md) for core invariants, constraints, and operational guidelines.
- For Developers: Start with [`docs/development/getting-started.md`](development/getting-started.md).
- For Architectural Decisions: Inspect [`docs/adr/`](adr/).

## Stable release preparation

- [Compatibility policy](releases/compatibility.md)
- [Release checklist](releases/checklist.md)
- [Vault recovery](operations/recovery.md)
- [Demonstration vault](development/demo-vault.md)
