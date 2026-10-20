# ADR 0016: Harvester Project Filtering, Toxic Directory Exclusion, and Orphan Pruning

* Status: accepted
* Date: 2026-09-18

## Context and Problem Statement

Daneel harvests technical documentation (`docs/`, `rfcs/`, `adrs/`, `architecture.md`, `catalog-info.yaml`) from organizational repositories hosted across GitLab and GitHub into `sources/` to power full-text search (SQLite FTS5) and the 3D/WebGL galaxy visualizer.

Previously, the harvester operated under a purely additive and blindly recursive model:
1. **Graph Bloat**: Over 90% of indexed nodes resided under `sources/`, overwhelming the ForceAtlas2 physics engine and obscuring core strategic notes.
2. **Toxic Directory Ingestion**: Lack of path filtering allowed vendored dependencies and build outputs (`node_modules`, `vendor/`, `wp-content/`, `dist/`, `.git`) to be harvested (e.g. hundreds of extraneous markdown notes inside npm packages).
3. **Eternal Persistence of Archived Repositories**: GitLab omitted archived projects when querying active repos, but existing files on disk were never pruned. On GitHub, the API returned archived projects without checking the archived state, continuing to harvest them.
4. **No Domain or Project Exclusion**: Non-technical or abandoned repositories (`*poc*`, sandbox experiments, hackathon repos, marketing assets) could not be excluded via declarative configuration.

A resilient, configurable filtering and pruning engine was required to eliminate noise at both ingestion and storage phases.

## Decision Drivers

* **Early Rejection (Network & I/O Savings)**: Skip archived, group-excluded, or project-excluded repositories at the API level before requesting tree structures or file blobs.
* **Toxic Path Elimination**: Guarantee zero ingestion of dependency or build directories (`node_modules`, `vendor`, `wp-content`, etc.) regardless of repository contents.
* **Automated Pruning & Set Difference Reconciliation**: Detect and purge orphaned on-disk repositories ($\text{Orphans} = \text{Disk} - \text{Active}$) as well as previously harvested toxic directories or newly excluded projects.
* **Declarative Configuration**: Allow users to specify `skip_archived`, `exclude_groups`, `exclude_projects`, and custom `ignored_directories` in `.daneel.yaml`.
* **Safe Operations & Introspection**: Provide `--dry-run` and `--json` flags to preview or inspect cleanup actions, accompanied by automatic MOC (`index.md`) reconciliation and FTS5 reindexing.

## Considered Options

* **Option 1: Post-Harvest Shell Scripts or Git Clean**: Rely on external scripts to scan `sources/` and delete directories matching toxic patterns.
  * *Disadvantages*: Does not prevent network consumption during harvest; fragile path handling; fails to reconcile OKF MOCs (`index.md`) or SQLite FTS5 indices.
* **Option 2: Filter Only in Ingestion (`ShouldHarvest`) without Pruning**: Add path filters to skip downloading files, but leave existing local disk files untouched.
  * *Disadvantages*: Does not solve existing vault bloat; archived or deleted remote repos remain on disk indefinitely.
* **Option 3: End-to-End Filtering, Early Skip, and Set-Difference Pruning Engine**:
  * Expand `HarvesterConfig` with `SkipArchived`, `IgnoredDirectories`, `ExcludeGroups`, and `ExcludeProjects`.
  * Enforce early skip in GitHub/GitLab API workers before tree scans.
  * Filter toxic directories immediately in `ShouldHarvest`.
  * Implement `PruneSources` with set difference ($\text{Orphans} = \text{Disk} - \text{Active}$), toxic folder removal, empty folder cleanup, and automatic MOC reconciliation (`ReconcileSourcesCatalog`).
  * Surface via `daneel harvest prune` subcommand and `daneel harvest --prune` flag.

## Decision Outcome

Chosen option: **Option 3: End-to-End Filtering, Early Skip, and Set-Difference Pruning Engine**.

### Architectural Details

1. **Configuration Schema (`internal/config/config.go`)**:
   - `SkipArchived *bool`: Defaults to `true` via helper `ShouldSkipArchived()`.
   - `IgnoredDirectories []string`: Defaults to `["node_modules", "vendor", "wp-content", "dist", "build", "coverage", ".git", ".next"]`.
   - `ExcludeGroups []string`: List of glob patterns for groups/namespaces (e.g. `marketing`, `github-images`, `*sandbox*`).
   - `ExcludeProjects []string`: List of glob patterns for repository names/paths (e.g. `*poc*`, `test-*`).
   - Helper methods: `GetIgnoredDirectories()`, `IsGroupExcluded(group)`, `IsProjectExcluded(project)`.

2. **Ingestion Filtering & Early Skip (`internal/harvester/`)**:
   - `harvester.go`: `ShouldHarvest` immediately checks path segments against `GetIgnoredDirectories()`, rejecting any file within toxic directories.
   - `github.go`: Deserializes `repo.Archived`; skips archived repositories if `ShouldSkipArchived()`; skips matching `IsGroupExcluded()` and `IsProjectExcluded()`; populates `HarvestStats.ActiveProjects`.
   - `gitlab.go`: Queries `archived=false` when `ShouldSkipArchived()`; applies `IsGroupExcluded()` and `IsProjectExcluded()`; populates `HarvestStats.ActiveProjects`.

3. **Pruning & Reconciliation Engine (`internal/harvester/prune.go`)**:
   - `PruneSources(opts PruneOptions) (PruneStats, error)`:
     1. Sweeps `sources/` identifying toxic directories (`node_modules`, etc.) and deletes them recursively.
     2. Identifies repository roots and verifies them against `opts.Config` (group/project exclusions) and `ActiveByPlatform` sets. Repositories not present in active lists or matching exclusions are purged.
     3. Cleans empty parent directories recursively.
     4. Runs `ReconcileSourcesCatalog` to rebuild clean `index.md` OKF MOCs for affected groups and repositories.
   - `LoadActiveRemoteProjects(vaultRoot, cfg, token)`: Remote API loader that queries GitHub and GitLab to map live active repositories for standalone pruning.

4. **Strict Repository MOC Topology & Subdirectory Index Purging (`internal/harvester/catalog.go`)**:
   - Repository roots are identified top-down (markers: `.git`, `README.md`, `catalog-info.yaml`, direct markdown files, or internal doc directories).
   - Once a repository root is detected, descending into child directories is halted via `filepath.SkipDir`, preventing subfolders (`docs/`, `plans/`, `specs/`, `superpowers/`) from generating spurious nested `index.md` files.
   - Any pre-existing `index.md` or `_index.md` files located in subdirectories of a repository are deterministically purged during catalog reconciliation.
   - Each repository maintains exactly one canonical `index.md` at its root listing all markdown documents across all subdirectories.

5. **CLI Surface (`cmd/harvest.go`)**:
   - `daneel harvest [target] --prune [--dry-run]`: Harvests active projects and executes post-harvest pruning.
   - `daneel harvest prune [--dry-run] [--remote] [--reindex] [--token]`: Dedicated pruning command capable of remote synchronization or local-only exclusion/toxic cleanup.
   - Fully supports `--json` structured output and daemon live reload event publishing.

## Consequences

### Positive

* **Massive Graph De-noising**: Removes thousands of extraneous dependency notes from `sources/`, restoring responsive ForceAtlas2 layout and clean semantic connections.
* **Network & Disk Efficiency**: Halts unnecessary tree requests and clone bandwidth for archived or irrelevant repositories.
* **Deterministic Synchronization**: Disk state now converges to remote reality; deleting or archiving a remote repository results in on-disk cleanup upon pruning.
* **Safe Execution**: Dry-run mode (`--dry-run`) provides complete transparency before performing any destructive deletions.

### Negative / Trade-offs

* **Disk Deletion**: Removing orphaned repositories permanently deletes files under `sources/`. Mitigated by scoping strictly to `sources/` and supporting `--dry-run`.
* **API Calls for Standalone Pruning**: Running `daneel harvest prune --remote` requires fetching repository listings from GitHub/GitLab to calculate set differences, subject to API rate limits.
