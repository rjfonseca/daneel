# ADR 0006: Autonomous Cognitive REM Dream Cycle and Zettelkasten Routing Engine

* Status: accepted
* Date: 2026-09-10

## Context and Problem Statement

Personal knowledge vaults naturally degrade over time without regular hygiene. Incoming drafts pile up in `inbox/`, cross-links between notes break as files are renamed, and leadership time allocations drift from strategic intentions.

Manual vault organization creates heavy cognitive overhead and is frequently abandoned by busy engineers.

How should Daneel automate vault consolidation, incoming thought routing, and long-term memory stabilization?

## Decision Drivers

* Minimizing cognitive overhead for note organization and triage.
* Neuro-inspired memory consolidation metaphor (REM sleep).
* Deterministic pre-flight hygiene combined with flexible AI agent reasoning.
* Persistent learning loop that adapts to user categorization preferences.

## Considered Options

* **Option 1: Real-time immediate file system watchers that auto-move files as they are saved**
* **Option 2: Periodic batch cognitive Dream Cycle (`daneel dream`) with deterministic hygiene + AI agent pass**
* **Option 3: Purely manual file organization**

## Decision Outcome

Chosen option: **Option 2: Periodic batch cognitive Dream Cycle**, because:
- Real-time auto-movers disrupt active writing flow and create race conditions with text editors.
- A batch REM-cycle executed overnight or on-demand allows:
  1. Deterministic hygiene (WikiLinks to Markdown conversion, MOC updates).
  2. Semantic Zettelkasten routing of `inbox/` thoughts to proper category trees.
  3. Weekly time telemetry aggregation across leadership archetypes.
  4. Strategic friction and tech debt detection via keyword frequency analysis.
  5. Agent feedback recording in `agent_learnings.json`.
- Supports automated overnight runs with system shutdown: `daneel dream && shutdown now`.

### Positive Consequences

* Vault stays permanently organized and indexed without ongoing manual effort.
* Machine learns user preferences over time via the feedback loop.
