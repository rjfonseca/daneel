# ADR 0042: Search Smart Sync Modes and Isolated `sources_index.db`

* Status: accepted
* Date: 2026-11-16

## Context and Problem Statement

In large knowledge vaults containing thousands of curated notes alongside bulky third-party documentation or meeting transcripts under `sources/`, `daneel search` previously suffered from two performance and storage bottlenecks:

1. **Unconditional Pre-Search Indexing**: Every `daneel search` invocation unconditionally executed an incremental `IndexVault` walk across all configured directories, adding disk I/O latency even when the daemon's hourly `tidy` routine had recently synchronized the index.
2. **Single-Database Bloat (`vault_index.db`)**: Low-signal or high-volume external documents in `sources/` were stored in the same SQLite FTS5 database as high-signal curated notes (`notes/`, `meta/`, `daily/`, `inbox/`, `public/`), inflating `vault_index.db` size, slowing down BM25 scans, and causing unnecessary page cache churn for queries that only target curated knowledge.

## Decision Drivers

* **Sub-10ms Search Hot Path**: Avoid redundant filesystem walks on `daneel search` when the index was recently synchronized, while still supporting explicit overrides.
* **Curated Knowledge Priority**: Keep `vault_index.db` lean and fast for curated scopes (`tech`, `meta`, `daily`, `inbox`, `public`), without opening or scanning `sources/` unless required.
* **Lazy Resource Allocation**: Open `sources_index.db` only when indexing `sources/`, querying `scope=sources`, or falling back in `scope=all` when primary results are fewer than `limit`.
* **Clean Migration**: Automatically purge legacy `scope = 'sources'` rows from existing single-DB `vault_index.db` installations and reclaim space with `VACUUM`.
* **Pure Go / Zero CGO**: Maintain `modernc.org/sqlite` compatibility with `context.Context` propagation and structured `log/slog` logging.

## Considered Options

* **Option 1: Keep single `vault_index.db` with SQL `WHERE scope != 'sources'` filtering**:
  * *Disadvantages*: FTS5 index pages and database file size remain dominated by `sources/` content; `VACUUM` and WAL checkpoints still pay the cost of the combined corpus.
* **Option 2: Separate `sources_index.db` with lazy connection + `index_metadata` staleness tracking (Chosen)**:
  * Isolates `sources/` into a sibling `sources_index.db` in the same cache directory (`:memory:` handled cleanly in tests).
  * Tracks `last_sync_at` in `index_metadata` and exposes `ShouldSync` / `IndexVaultIfStale` with explicit CLI flags (`--no-sync`, `--force-sync`, `--minimum-sync-age`).

## Decision Outcome

Chosen option: **Option 2: Separate `sources_index.db` with lazy connection + `index_metadata` staleness tracking**.

### 1. Intelligent Sync Modes (`cmd/search.go`, `internal/search/engine.go`)

* Added `index_metadata (key TEXT PRIMARY KEY, value TEXT NOT NULL)` to persist `last_sync_at` (RFC3339Nano) whenever `IndexVault` succeeds.
* Added `Engine.ShouldSync(ctx, minAge)` and `Engine.IndexVaultIfStale(ctx, dirs, minAge)`.
* Replaced unconditional pre-search `IndexVault` in `daneel search` with three explicit flags:
  * `--no-sync` (`bool`, default `false`): Never run `IndexVault` before searching.
  * `--force-sync` (`bool`, default `false`): Always run incremental `IndexVault` before searching, ignoring `--minimum-sync-age`.
  * `--minimum-sync-age` (`time.Duration`, default `2h30m`, 2.5x the default hourly `tidy` interval): Run incremental `IndexVault` only if `time.Since(lastSync) >= minimumSyncAge` or if never synced.
* Passing `--no-sync` and `--force-sync` together returns an explicit validation error.
* `--reindex` (`-r`) continues to force a full reindex.

### 2. Isolated `sources_index.db` (`internal/search/engine.go`)

* `Engine` derives `sourcesDBPath` in the same directory as `dbPath` (`sources_index.db`, or `:memory:` for in-memory test databases) and lazily opens `sourcesDB` via `ensureSourcesDB()`, closing both connections in `Engine.Close()`.
* `IndexVault` routes `sources/` paths to `sourcesDB` and all other directories to `db`.
* On first `IndexVault` run (tracked via `sources_migrated = 1` in `index_metadata`), any legacy `scope = 'sources'` rows in `db` (`notes_meta`, `notes_fts`, `note_links`) are deleted and `db` is `VACUUM`ed.
* `SearchFaceted` routes queries as follows:
  * `tech`, `meta`, `daily`, `inbox`, `public`: Queries only `db`; `sources_index.db` is never opened.
  * `sources`: Queries only `sourcesDB`.
  * `all` or `""`: Queries `db` first; only opens and queries `sourcesDB` if `len(results) < limit`, appending up to `limit - len(results)` results so curated notes always take priority.
* `IndexNote`, `GetNote`, `GetBacklinks`, and `HealthCheck` route or aggregate across `sourcesDB` when applicable.

## Consequences

* **Positive**: `daneel search` skips redundant directory walks within the `2h30m` freshness window and avoids touching `sources_index.db` for curated-scope queries or when `db` already satisfies `limit`.
* **Positive**: Existing single-DB caches automatically shrink on the next `IndexVault` run without manual user intervention.
* **Negative**: Vault-wide operations (`HealthCheck`, `GetBacklinks`) must check `sources_index.db` when it exists on disk, adding a second SQLite query when `sources/` is populated.
