# Architectural Decision Records (ADRs)

This directory contains the **Architectural Decision Records (ADRs)** for the Daneel Second Brain project, authored following the [Markdown Architectural Decision Records (MADR)](https://adr.github.io/madr/) specification.

## Architectural Decision Log

| ADR | Title | Status | Date |
|---|---|---|---|
| [ADR 0001](0001-daemon-unix-socket-and-lifecycle.md) | Daemon Core via Unix Domain Socket & Systemd Lifecycle | Accepted | 2026-09-10 |
| [ADR 0002](0002-ambient-intelligence-and-unified-mcp-ui-tools.md) | Ambient Intelligence & Unified MCP UI Tool Suite | Accepted | 2026-09-10 |
| [ADR 0003](0003-cli-delegation-run-or-delegate.md) | CLI Delegation (`RunOrDelegate`) & Visual Live Reload | Accepted | 2026-09-10 |
| [ADR 0004](0004-astrophysical-taxonomy-and-lod-edges.md) | Astrophysical Taxonomy & Dynamic LOD Edge Filtering | Accepted | 2026-09-09 |
| [ADR 0005](0005-robotics-security-guardrails.md) | Four Laws of Robotics Security Guardrails for Markdown Notes | Accepted | 2026-09-10 |
| [ADR 0006](0006-cognitive-dream-cycle.md) | Autonomous Cognitive REM Dream Cycle and Zettelkasten Routing Engine | Accepted | 2026-09-10 |
| [ADR 0007](0007-cross-platform-matrix-compatibility.md) | Cross-Platform Matrix Compatibility and Path Relativization | Accepted | 2026-09-11 |
| [ADR 0008](0008-hierarchical-sectors-and-repository-nebulas.md) | Hierarchical Sectors and Repository Nebulas | Accepted | 2026-09-14 |
| [ADR 0009](0009-cosmic-hud-design-system-and-spatial-ergonomics.md) | Cosmic HUD Design System, Lateral Drawer Shift, and Viewport Collision Ergonomics | Accepted | 2026-09-15 |
| [ADR 0010](0010-seldon-vault-anchors-and-temporal-hotbar-ergonomics.md) | Seldon Vault Anchors, Weekly Synthesis Navigation, and Temporal Hotbar Ergonomics | Accepted | 2026-09-15 |
| [ADR 0011](0011-programmatic-seldon-anchor-manipulation-via-mcp-and-daemon.md) | Programmatic Seldon Vault Anchor Manipulation via MCP and Daemon IPC | Accepted | 2026-09-16 |
| [ADR 0012](0012-astronomical-hierarchy-and-okf-repository-anchors.md) | Astronomical Hierarchy and OKF Repository Anchors | Accepted | 2026-09-16 |
| [ADR 0013](0013-web-action-deck-and-session-mining.md) | Web Cockpit Action Deck ConnectRPC Streaming & Agent Session Mining | Accepted | 2026-09-14 |
| [ADR 0014](0014-satellite-arms-interfaces-and-ai-skill-parameterization.md) | Satellite ARMS Specialization, Dynamic Skill Parameterization, and Agent Prompt Dispatch | Accepted | 2026-09-16 |
| [ADR 0015](0015-unified-daemon-and-systemd-routine-scheduler.md) | Unified Daemon Multi-Listener and Systemd Routine Scheduler | Accepted | 2026-09-17 |
| [ADR 0016](0016-harvester-project-filtering-and-pruning.md) | Harvester Project Filtering, Toxic Directory Exclusion, and Orphan Pruning | Accepted | 2026-09-18 |
| [ADR 0017](0017-discovery-and-entity-resolution-primitives.md) | Discovery and Entity Resolution Primitives for AI Agents and CLI | Accepted | 2026-09-18 |
| [ADR 0018](0018-agent-evals-and-agnostic-telemetry.md) | Agent Evals and Agnostic Telemetry for Missed Opportunities and Knowledge Gaps | Accepted | 2026-09-18 |
| [ADR 0019](0019-metacognitive-dream-cycle-and-agent-evolution.md) | Metacognitive Dream Cycle, Engine Profiling, and Autonomous Knowledge Synthesis | Accepted | 2026-09-18 |
| [ADR 0020](0020-web-cockpit-cognitive-command-deck-and-telemetry.md) | Web Cockpit Cognitive Command Deck, Quick-Action Presets, and Introspective Telemetry | Accepted | 2026-09-18 |
| [ADR 0021](0021-routine-execution-history-and-observability-ledger.md) | Routine Execution History, Trigger-Agnostic Observability Ledger, and Cockpit Timeline | Accepted | 2026-09-22 |
| [ADR 0022](0022-workspace-branch-inactivity-and-merged-pruning.md) | Workspace Branch Inactivity Guardrail and Merged Branch Pruning Lifecycle | Accepted | 2026-09-24 |
| [ADR 0023](0023-staff-telemetry-temporal-breakdown-and-leverage-matrix.md) | Staff Telemetry Temporal Breakdown, Rolling Windows, and Leverage Matrix | Accepted | 2026-09-29 |
| [ADR 0024](0024-native-agent-hooks-and-ambient-telemetry-spooler.md) | Native Agent Hooks, Ambient Telemetry Spooler, and Cross-Agent Continuity | Accepted | 2026-10-05 |
| [ADR 0025](0025-local-http-and-vault-read-boundaries.md) | Local HTTP and Vault Read Boundaries | Accepted | 2026-10-14 |
| [ADR 0026](0026-recoverable-vault-maintenance.md) | Recoverable Vault Maintenance | Accepted | 2026-10-14 |
| [ADR 0027](0027-stable-release-contract-and-validation.md) | Stable Release Contract and Validation | Accepted | 2026-10-14 |
| [ADR 0028](0028-deferred-web-panels-and-typed-boundaries.md) | Deferred Web Panels and Typed Boundaries | Accepted | 2026-10-14 |

| [ADR 0029](0029-knowledge-exploration-and-reading.md) | Knowledge Exploration and Reading | Accepted | 2026-10-14 |
| [ADR 0030](0030-local-inbox-capture.md) | Local Inbox Capture | Accepted | 2026-10-14 |
| [ADR 0031](0031-unified-command-bridge.md) | Unified Command Bridge and Shared HUD Content Boundary | Accepted | 2026-10-16 |
| [ADR 0032](0032-bounded-agent-hook-execution.md) | Bounded Agent Hook Execution | Accepted | 2026-10-19 |
| [ADR 0033](0033-audited-frontend-toolchain.md) | Audited Frontend Toolchain | Accepted | 2026-10-19 |

## Mandatory ADR Invariant

> [!IMPORTANT]
> **Strict Invariant**: Every non-trivial architectural change, protocol definition, data flow modification, or visual contract change **MUST** be recorded as a MADR in this directory prior to or alongside the commit. PRs or commits modifying architecture without an ADR are strictly unacceptable.
