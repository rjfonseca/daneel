# ADR 0038: Incremental Watermark Cache and Copilot Primitives

* Status: accepted
* Date: 2026-11-04

## Context and Problem Statement

Daneel manages personal knowledge vaults that scale to tens of thousands of technical notes (`notes/`, `daily/`, `sources/`, `inbox/`, `meta/`) alongside software catalogs tracking hundreds of external Git repositories (`acme.corp` organizations on GitHub and GitLab).

As vaults grow beyond 10,000 notes and 300 repositories, three operational friction points emerged:
1. **Exhaustive Vault I/O Latency**: Routine commands (`daneel tidy` and `daneel doctor`) executed full directory traversals (`filepath.WalkDir`) that re-read and parsed the AST and frontmatter of every note from disk on every invocation, causing multi-second delays and high disk I/O even when zero files had changed.
2. **Sequential Harvester Bottlenecks**: `GitLabHarvester` and `GitHubHarvester` scraped repositories sequentially, leading to extended runtimes and network idling when fetching documentation trees across dozens of projects.
3. **Manual Meeting Ingestion & Daily Scaffolding Friction**:
   - Google Meet and Gemini meeting notes deposited into `inbox/` accumulated without automatic processing until explicit manual invocation of `daneel ingest`.
   - Daily log scaffolding (`daily/YYYY-MM-DD.md`) and end-of-day time reconciliation frequently relied on general-purpose AI coding agents, consuming millions of prompt and generation tokens for tasks that are inherently deterministic.

### The Synaptic Graph Invariant Problem

A naive file-skipping cache (checking only file modification times `mtime` and skipping unchanged notes) fails in graph-based knowledge systems:
- `daneel tidy` requires a comprehensive `targetMap` containing every valid note basename, normalized title, and relative path across the entire vault to convert `[[WikiLinks]]` into canonical Markdown relative links. If unchanged notes are skipped without hydrating their targets, valid WikiLinks become corrupted or unresolved.
- `daneel doctor` must detect broken links across the synaptic graph. When a target note is deleted or renamed, notes linking to it become broken even if their own file content and `mtime` never changed. A cache that only evaluates modified source files would fail to detect newly broken backlinks.

## Decision Drivers

* **Sub-50ms Incremental Diagnostics & Hygiene**: Enable sub-second execution for `daneel tidy` and `daneel doctor` on 10,000+ note vaults by bypassing disk reads for unchanged notes.
* **Full Synaptic Graph Consistency**: Guarantee complete link resolution (`targetMap`) and 100% accurate broken-link detection without re-reading unchanged files from disk.
* **Autonomous Meeting Ingestion in Cognitive Dream Cycle**: Automatically ingest meeting notes and update participant profiles during Phase 1 hygiene without manual CLI steps.
* **Safe, Race-Free Harvester Concurrency**: Parallelize repository scraping across multi-project organizations with bounded worker pools, avoiding HTTP 429 rate limits and thread contention.
* **Deterministic, Zero-Token Copilot Primitives**: Provide instantaneous CLI commands (`daneel morning` and `daneel evening`) to scaffold daily logs, surface Dream Cycle handoff intelligence, and reconcile Staff Engineering time allocations without LLM token overhead.
* **Strict Vault Boundary & Open-Source Invariants**: Enforce safe file writing through `safefile.WriteFile`, maintain English-only UI chrome, and preserve strict neutrality without corporate or personal data leaks.

## Considered Options

* **Option 1: In-Memory Invalidation with Naive `mtime` File Skipping**:
  * *Disadvantages*: Does not persist across CLI invocations. Skipping unchanged files leaves in-memory graphs incomplete, corrupting `targetMap` in `daneel tidy` and missing broken links to deleted targets in `daneel doctor`.
* **Option 2: External Agent Delegation for Daily Scaffolding & Time Tracking**:
  * *Disadvantages*: Incurs significant LLM token costs ($10–30/month), introduces 5–15 second latency, risks non-deterministic formatting or hallucinations, and risks accidental modification of multi-week backlog epics.
* **Option 3: SQLite Watermark Cache, Bounded Harvester Pool, and Deterministic Copilot Primitives (Chosen)**:
  * Persistent SQLite cache storing file modification times, hashes, and extracted link graphs.
  * Hydrates full in-memory graphs in <15ms without disk reads.
  * Bounded 8-worker concurrent harvester pool with mutex protection.
  * Autonomous meeting ingestion during Dream Cycle Phase 1.
  * Deterministic CLI primitives for morning planning and evening reconciliation.

## Decision Outcome

Chosen option: **Option 3: SQLite Watermark Cache, Bounded Harvester Pool, and Deterministic Copilot Primitives**.

### Architectural Topology

```
 ┌─────────────────────────────────────────────────────────────────────────────┐
 │                             Core Engine Inputs                              │
 │   [ Vault Notes (10k+) ]   [ Meeting Inbox ]   [ GitLab / GitHub (300+) ]   │
 └─────────────┬──────────────────────┬──────────────────────────┬─────────────┘
               │                      │                          │
               ▼                      ▼                          ▼
     ScanVaultIncremental       ProcessInbox            Bounded Worker Pool
    (vault_index.db Cache)   (Phase 1 Auto-Ingest)    (max 8 concurrent repos)
               │                      │                          │
               ├──────────────────────┤                          │
               ▼                      ▼                          │
    Synaptic Graph Hydrated     safefile.WriteFile               ▼
   (targetMap & Link Integrity) (sources/reunioes/)       Sync Mutex / Cache
               │                      │                          │
               ├──────────────────────┴──────────────────────────┘
               │
               ▼
 ┌─────────────────────────────────────────────────────────────────────────────┐
 │                         Positronic Copilot Surfaces                         │
 │                                                                             │
 │  🌅 daneel morning                      🌇 daneel evening                   │
 │  • Dream Cycle Handoff Intelligence     • Explicit [A]-[E] Time Parsing     │
 │  • Active P0/P1 Backlog Priorities      • Deterministic YAML Frontmatter    │
 │  • Inbox Attention Checkboxes           • Zero-Token Instant Reconciliation │
 └─────────────────────────────────────────────────────────────────────────────┘
```

---

### Detailed Component Specifications

#### 1. Pillar 1: SQLite `file_watermarks` Incremental Cache (`internal/vault`)

The incremental scanning engine maintains note state in `<vaultRoot>/.daneel/cache/vault_index.db` using zero-CGO SQLite (`modernc.org/sqlite`) with WAL mode enabled:

```sql
CREATE TABLE IF NOT EXISTS file_watermarks (
    path TEXT PRIMARY KEY,
    mtime INTEGER NOT NULL,
    content_hash TEXT NOT NULL,
    link_count INTEGER NOT NULL,
    title TEXT NOT NULL DEFAULT '',
    note_type TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    links_json TEXT NOT NULL DEFAULT '[]',
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_file_watermarks_mtime ON file_watermarks(mtime);
```

##### Scanning & Hydration Algorithm (`ScanVaultIncremental`)
1. **Watermark Pre-load**: Loads all existing records from `file_watermarks` into memory in a single fast query (`< 5ms`).
2. **Fast Filesystem Walk**: Traverses configured indexed directories using `filepath.WalkDir`. Compares disk `mtime` (nanosecond precision) against the cached `mtime`.
3. **Cache Hit Hydration**: For unchanged files, reconstructs the `*Note` struct directly from cached metadata (`Title`, `NoteType`, `Description`, and deserialized `Links` slice) without reading file bytes or running Markdown parsers.
4. **Concurrent Cache Miss Parsing**: Newly created or modified files are dispatched to a bounded worker pool (`min(runtime.NumCPU(), 16)`). Workers parse note content, extract links, compute SHA-256 hashes, and construct updated `FileWatermark` records.
5. **Batch Transactional Upsert**: New and updated watermarks are committed to SQLite in a single transaction.
6. **Stale Record Pruning**: Any database record whose corresponding file is no longer present on disk is deleted in a batch transaction.
7. **Graph Link Integrity**:
   - Because all notes (cached and parsed) supply their full outgoing `Links` slice, `targetMap` contains 100% of the vault's note titles, aliases, and file paths.
   - When a note is deleted, its entry is purged from `targetMap`. Any referencing note immediately surfaces a broken link during `daneel doctor` diagnostics without needing to re-parse the referencing note from disk.
8. **Cache Bypass (`--force`)**: `daneel tidy --force` and `daneel doctor --force` pass `force = true`, which re-parses all notes from disk and refreshes the cache.

#### 2. Pillar 2: Autonomous Phase 1 Meeting Ingestion (`internal/dream`)

To eliminate manual transcription bottlenecks, meeting processing is integrated directly into Phase 1 (Deterministic Graph Hygiene) of `DreamEngine.RunFullCycle`:

```go
if autoFix && !dryRun {
    proc := meetings.NewProcessor(e.vaultRoot)
    if ingRep, err := proc.ProcessInbox(true); err == nil && ingRep != nil {
        report.IngestedMeetingsCount = ingRep.TotalProcessed
    }
}
```

* **Pipeline Flow**:
  1. Identifies raw meeting draft transcripts in `inbox/`.
  2. Extracts participants, decisions, action items, and executive summaries.
  3. Writes structured meeting documents into `sources/reunioes/YYYY-MM-DD-<slug>.md`.
  4. Appends interaction history entries to matching colleague profile notes in `notes/pessoas/`.
  5. Cleans processed drafts from `inbox/` and reports `IngestedMeetingsCount` in the Dream Cycle telemetry.
* **Vault Boundary Enforcement**: All file operations utilize `safefile.WriteFile(vaultRoot, destPath, data, 0600)` to ensure writes remain strictly confined within the vault hierarchy.

#### 3. Pillar 3: Bounded 8-Worker Concurrent Repository Harvester (`internal/harvester`)

`GitLabHarvester` and `GitHubHarvester` parallelize multi-repository documentation extraction:
* **Worker Pool Architecture**: Configures a fixed pool of up to 8 concurrent repository workers (`const maxRepoWorkers = 8`). Repositories are distributed via a buffered channel (`repoCh`).
* **Thread-Safe State**: Access to the SHA cache map (`cache[key] = sha`) and metric counters (`HarvestStats`) is synchronized via `sync.Mutex`, validated under the Go race detector (`go test -race`).
* **Rate-Limit & Storm Protection**: Bounded concurrency avoids triggering secondary API rate limits or HTTP 429 throttling on remote hosting providers, while achieving up to 6x speedup across large repository portfolios.

#### 4. Pillar 4: Deterministic Copilot Daily Primitives (`internal/ops/daily`)

Daneel provides high-speed, zero-token daily primitives via CLI:

##### `daneel morning`
* Reads recent autonomous actions, healed links, alerts, and metacognition insights from `.daneel/cache/dream_handoff.json`.
* Lists pending triage items from `inbox/`.
* Queries active uncompleted `P0` and `P1` tasks from the weekly backlog (`backlog.GetActiveBacklog`).
* Discovers ingested meetings for the date under `sources/reunioes/`.
* Creates or idempotently updates `daily/YYYY-MM-DD.md` with standard YAML frontmatter and a `## 🤝 Handoff do Copiloto (Daneel ➔ <Owner>)` section in < 15ms.

##### `daneel evening`
* Parses explicit inline time annotations from the daily log body (e.g. `- [A] 2.0h: Architecture review`, `- 1.5h [C] Mentoring 1:1`, `- [B] (1.5h) Implementation`).
* Aggregates duration across Staff Engineering archetype buckets:
  - `A`: Strategic (`tempo-A_Estrategico`)
  - `B`: Local Impact (`tempo-B_ImpactoLocal`)
  - `C`: Glue Work (`tempo-C_GlueWork`)
  - `D`: Low Impact (`tempo-D_LowImpact`)
  - `E`: Overhead (`tempo-E_Overhead`)
* **Safety Invariant**:
  - If inline annotations are found (`TotalHours > 0`), updates frontmatter `tempo-*` keys while preserving all other frontmatter attributes and the note body.
  - If no inline annotations are found (`TotalHours == 0`), frontmatter values are **never** overwritten with zeroes; existing frontmatter numbers are preserved intact.

---

### Explicit Scope Boundaries & Rejected Alternatives

#### 1. Rejection of Semantic `[x]` Auto-Completion of Weekly Backlog Epics
* **Proposed Idea**: Have `daneel evening` or `daneel morning` parse daily logs and automatically check off (`- [x]`) matching weekly backlog epics in `meta/sumario-semanal.md` or backlog files.
* **Why Rejected**: Lexical keyword or fuzzy matching in Go cannot distinguish between *"spent 2 hours working on RFC-0038 today"* and *"completed 100% of the multi-week RFC-0038 initiative"*. Automated regex matching causes destructive false positives, prematurely closing active strategic initiatives. Backlog epic completion requires conscious human decision or high-confidence interactive review in Dream Cycle Phase 2.

#### 2. Rejection of `--close` Dream Cycle Chaining
* **Proposed Idea**: Add a `--close` flag to `daneel evening` that executes a full cognitive Dream Cycle and triggers automatic Git commits.
* **Why Rejected**: Violates the Single Responsibility Principle (SRP). `daneel evening` is a sub-10ms deterministic time reconciler designed for instant end-of-day execution. Chaining cognitive REM cycles (which include graph analysis, clustering, and LLM-assisted metacognition) introduces multi-second latency, token burn, and potential lock contention. Heavy cognitive consolidation remains the responsibility of `daneel dream` and scheduled background systemd routines.

---

## Consequences

### Positive

* **Massive Hygiene & Diagnostics Speedup**: Vault scanning time for 10,000+ notes drops from seconds to < 50ms on incremental runs.
* **Preserved Synaptic Integrity**: 100% accurate WikiLink resolution and broken-link detection without disk read overhead.
* **Autonomous Ingestion**: Meeting transcripts in `inbox/` are processed and linked without manual intervention.
* **Harvester Scalability**: Concurrent scraping handles 300+ repositories in parallel without race conditions or rate-limit penalties.
* **Zero-Cost Daily Scaffolding**: Morning briefing and evening time reconciliation execute deterministically in milliseconds without LLM token expenses.

### Negative / Trade-Offs

* **Cache State Management**: Requires managing `<vaultRoot>/.daneel/cache/vault_index.db`, necessitating `--force` flags for edge cases where filesystem timestamps are modified out of band.
* **Cold Cache Initial Overhead**: The initial run on an unindexed vault must perform full hashing and AST extraction before incremental benefits take effect.
