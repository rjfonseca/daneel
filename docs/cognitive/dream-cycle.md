# Cognitive Dream Cycle & Memory Consolidation

The **Dream Cycle** is Daneel's autonomous background consolidation engine, inspired by REM sleep and neuroscience theories of memory stabilization. It runs nightly or on-demand to maintain vault hygiene, triage incoming thoughts, compile telemetry, and monitor strategic friction.

---

## 1. Architectural Overview

```
                          ┌───────────────────────────┐
                          │   daneel dream Execution  │
                          └─────────────┬─────────────┘
                                        │
             ┌──────────────────────────┼──────────────────────────┐
             ▼                          ▼                          ▼
   ┌───────────────────┐      ┌───────────────────┐      ┌───────────────────┐
   │ 1. Vault Hygiene  │      │ 2. Inbox Triage   │      │ 3. Telemetry &    │
   │    & Link Fixes   │      │    & Routing      │      │    Radar Analysis │
   └───────────────────┘      └───────────────────┘      └───────────────────┘
             │                          │                          │
             └──────────────────────────┼──────────────────────────┘
                                        │
                                        ▼
                              ┌───────────────────┐
                              │ 4. Agent Learning │
                              │    & Self-Healing │
                              └───────────────────┘
```

---

## 2. The Five Consolidation Phases

### Phase 1: Deterministic Graph Hygiene & Indexing
Before deep processing begins, deterministic hygiene executes:
- Converts un-rendered `[[wiki-links]]` to valid relative Markdown links and tracks fixed vs. broken counts.
- Synchronizes directory-level Maps of Content (`moc-*.md`).
- Updates people profiles with recent meeting notes and interaction dates.
- Catalogs local repositories and updates `moc-workspace-work.md`.
- Re-indexes the SQLite FTS5 database.

### Phase 2: Strategic Telemetry & Leadership Radar
Aggregates daily logs (`daily/YYYY-MM-DD.md`) over a configurable temporal window:
- Aggregates hours across strategic leadership archetypes (e.g., Strategic, Local Impact, Glue Work, Overhead).
- Computes percentage breakdown against calibrated target thresholds.
- Generates drift alerts if time allocation deviates significantly from targets.
- Cross-synthesizes technical activity with strategic business radar priorities.

### Phase 3: Zettelkasten Inbox Triage & Emerging Concepts
Unprocessed files in `inbox/` are scanned and evaluated against configured routing rules:
- Matches keywords to destination categories (e.g., `notes/ia`, `notes/arquitetura`, `notes/processos`).
- Discovers emerging concepts and topics from daily entries and backlog tickets.
- Generates interactive handoff questions cached in `.daneel/cache/dream_handoff.json`.

### Phase 4: Metacognitive Profiling & Introspective Bottleneck Analysis
Evaluates the internal health and efficiency of the Dream Engine execution:
- Tracks precise duration (`duration_ms`) and item throughput across each phase.
- Detects graph entropy bottlenecks: broken links, high inbox density ($\ge 10$ items), and missing canonical MOCs.
- Generates self-healing recommendations and skill refactoring proposals.
- Synchronizes metrics in `meta/dashboard-metricas.md` under section `## 🧠 4. Metacognição & Evolução Autônoma do Agente`.

### Phase 5: Active Knowledge Synthesis Induction & Evolution Log
Proactively bridges disparate domains into cohesive Zettelkasten syntheses:
- Connects emerging concepts with top active systems to scaffold synthesis candidate notes (`type: sintese`, `status: draft`) in `notes/sinteses/` (or `inbox/`).
- Appends an immutable, chronological execution record into `meta/guias/agente-evolucao.md` documenting reference dates, cycle runtimes, detected bottlenecks, and induced syntheses.

---

## 3. CLI Invocations

```bash
# Run full cognitive dream cycle with AI agent
daneel dream

# Run overnight consolidation and power down machine upon completion
daneel dream && shutdown now

# Run deterministic hygiene and telemetry pass without launching AI agent
daneel dream --no-agent

# Inspect engine profiling, phase breakdown, and metacognitive insights
daneel dream --no-agent --stats

# Disable automatic knowledge synthesis induction
daneel dream --no-agent --synthesize=false

# Export complete dream report as JSON
daneel dream --no-agent --json

# Specify target date for historical evaluation
daneel dream --date 2026-08-15

# Audit recent agent transcripts for missed vault opportunities and knowledge gaps
daneel audit agents --since 48h --apply
```

## Persistence and recovery

The internal dry-run cycle does not write notes, people profiles, caches, logs or
dashboards. Executing cycles replace files atomically and preserve earlier bytes
under `.daneel/backups/`. Persistence failures stop the cycle and return an error;
completed earlier writes are not automatically rolled back. Run one mutating
maintenance operation per vault at a time. See [recovery](../operations/recovery.md).
