# ADR 0019: Metacognitive Dream Cycle, Engine Profiling, and Autonomous Knowledge Synthesis

* Status: accepted
* Date: 2026-09-18

## Context and Problem Statement

Daneel's nocturnal Dream Cycle (established in ADR 0006) coordinates deterministic graph hygiene, Zettelkasten inbox triage, Staff Engineer telemetry compilation, and AI agent execution. However, as the Second Brain scaled in complexity and multi-agent operations intensified, three primary limitations emerged:

1. **Black-Box Engine Execution**: The cycle provided no granular timing telemetry across internal processing phases. When vault hygiene, FTS5 re-indexing, or inbox triage experienced latency spikes or performance degradation, neither the human engineer nor the AI agent had visibility into execution bottlenecks.
2. **Passive Knowledge Retention without Active Synthesis**: Emerging technical concepts recorded in daily notes or newly cloned repositories remained siloed within their respective subdirectories. The system lacked an active Zettelkasten mechanism to automatically detect cross-domain intersection opportunities (e.g., bridging an emerging concept with active backend architectures) and scaffold synthesis candidate drafts (`type: sintese`).
3. **Absence of a Persistent Evolution Ledger**: While user feedback and evaluations were recorded in `agent_learnings.json`, the agent lacked an introspective, append-only narrative log (`meta/guias/agente-evolucao.md`) and dedicated dashboard metric tracking (`## 🧠 4. Metacognição & Evolução Autônoma do Agente` in `meta/dashboard-metricas.md`) documenting its self-healing insights, detected bottlenecks, and longitudinal skill maturation.

## Decision Drivers

* **Zero-Overhead Micro-Profiling**: Track execution duration (`duration_ms`) and item throughput deterministically across all consolidation phases without external dependencies.
* **Introspective Bottleneck Detection**: Automatically detect graph entropy (broken links, missing MOCs, inbox backlog accumulation $\ge 10$) and generate self-healing action items.
* **Active Zettelkasten Knowledge Induction**: Cross-synthesize emerging concepts with high-traffic systems, generating structured synthesis drafts in `notes/sinteses/` (or `inbox/` if uninitialized) with standard OKF frontmatter (`type: sintese`, `status: draft`).
* **Persistent Append-Only Evolution Log**: Maintain a human- and agent-readable audit trail in `meta/guias/agente-evolucao.md` recording cycle timings, bottlenecks, and introspective reflections.
* **CLI Parity & English UI Invariant**: Expose engine profiling via `--stats` (and alias `--metacognition`) and provide `--synthesize` control, rendering styled Lipgloss summary cards in concise English.

## Considered Options

* **Option 1: Rely solely on AI Agent Prompting**: Delegate profiling, bottleneck identification, and knowledge synthesis entirely to the nocturnal LLM prompt.
  * *Disadvantages*: High token overhead; non-deterministic execution times; inability to measure Go engine runtime internally; prone to hallucination or skipping subtle graph bottlenecks.
* **Option 2: External Python Script or Sidecar**: Implement metacognitive monitoring and note synthesis via an external Python daemon or cron script.
  * *Disadvantages*: Violates single-binary Go architecture; introduces external environment dependencies and IPC overhead.
* **Option 3: Integrated 5-Layer Go Cognitive REM Pipeline**: Expand `DreamEngine` into a cohesive 5-phase pipeline that profiles internal stages, runs metacognitive analysis, induces Zettelkasten syntheses, appends to the evolution log, and updates vault dashboards.

## Decision Outcome

Chosen option: **Option 3: Integrated 5-Layer Go Cognitive REM Pipeline**.

### Architecture & Data Flow

```
                      daneel dream [--stats] [--synthesize]
                                       │
      ┌────────────────────────────────┼────────────────────────────────┐
      ▼                                ▼                                ▼
 Phase 1: Deterministic          Phase 2: Strategic               Phase 3: Zettelkasten
 Graph Hygiene (Links, MOCs,     Telemetry (Daily Log,            Inbox Triage & Emerging
 Workspace Cataloging)           Temporal Window, People)         Concepts Discovery
      │                                │                                │
      └────────────────────────────────┼────────────────────────────────┘
                                       │
                                       ▼
                         Phase 4: Metacognitive Pass
                         (Duration Profiling, Bottlenecks,
                          Introspective Self-Healing Insights)
                                       │
                                       ▼
                         Phase 5: Knowledge Synthesis Induction
                         (Cross-domain note generation in notes/sinteses/)
                                       │
                                       ▼
                   Persistent Ledger & Dashboard Synchronization
                   (meta/guias/agente-evolucao.md & meta/dashboard-metricas.md)
```

### Key Components

1. **Stage Profiling & Introspection (`internal/dream/metacognition.go`)**:
   - `StageTiming`: Captures `Name`, `DurationMs`, `ItemsCount`, and `Description`.
   - `RunMetacognitivePass`: Evaluates graph hygiene metrics:
     - Broken links $\rightarrow$ Flags graph traversal friction and recommends automated link healing.
     - Inbox accumulation $\ge 10$ drafts $\rightarrow$ Flags nocturnal batch delays and recommends asynchronous intake hooks.
     - Staff drift alerts $\rightarrow$ Identifies deviation from quarterly strategic engineering allocations.
     - Missing MOCs $\rightarrow$ Flags dense folders lacking entry points and recommends MOC generation.
     - Feedback calibrations $> 5$ $\rightarrow$ Suggests consolidating active rules into root `AGENTS.md`.

2. **Active Knowledge Synthesis Induction (`InduceKnowledgeSyntheses`)**:
   - Detects emerging concepts from daily interactions and pairs them with top active systems.
   - Scaffolds synthesis drafts with canonical OKF frontmatter (`type: sintese`, `status: draft`, `tags: [sintese, zettelkasten, arquitetura]`).
   - Writes drafts to `notes/sinteses/<slug>.md` (or `inbox/sintese-<slug>.md` if `notes/sinteses` does not yet exist), leaving existing notes intact.

3. **Persistent Evolution Log (`AppendEvolutionLog`)**:
   - Appends markdown entries into `meta/guias/agente-evolucao.md` documenting:
     - Cycle reference date and total duration in milliseconds.
     - Stage execution breakdown table.
     - Detected bottlenecks and generated insights.
     - Induced synthesis links.

4. **Dashboard Metrics Synchronization (`UpdateDashboardMetricas`)**:
   - Preserves existing sections (including `## 🤖 3. Telemetria de Agentes & Evals`) while generating or updating `## 🧠 4. Metacognição & Evolução Autônoma do Agente` with cycle duration, bottleneck status, and induced knowledge syntheses.

5. **CLI Interface (`cmd/dream.go`)**:
   - Flags: `--stats` (and `--metacognition`), `--synthesize` (default `true`), `--no-agent`, `--json`.
   - Renders a styled Lipgloss summary card displaying total duration, phase breakdown, warning badges (`BOTTLENECK`), insight badges (`INSIGHT`), and synthesis badges (`SYNTHESIS`) in strict English.

## Consequences

### Positive
* Sub-millisecond visibility into internal dream engine stages and resource allocation.
* Proactive knowledge bridge formation between newly observed ideas and established platform architectures.
* Continuous, tamper-evident evolution log enabling longitudinal evaluation of agent autonomy and self-repair.
* Clean CLI output and JSON integration for programmatic consumption by IDEs and MCP tools.

### Negative
* Phase 5 note generation produces draft files that require periodic user review or promotion.
