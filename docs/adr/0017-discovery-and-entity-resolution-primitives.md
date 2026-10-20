# ADR 0017: Discovery and Entity Resolution Primitives for AI Agents and CLI

* Status: accepted
* Date: 2026-09-18

## Context and Problem Statement

Autonomous AI agents (such as Antigravity/AGY, Claude Code, Cursor, and Windsurf) and CLI engineers interacting with the Second Brain and local repositories (`~/Work`) frequently regress to generic, high-latency filesystem inspection tools (`find`, `grep`, `fd`, `rg`, `list_dir`).

Repeated filesystem inspection creates avoidable overhead when agents need to:
- Locate notes about people, files, or concepts.
- Explore daily notes, meetings, and repositories.
- Resolve canonical filenames from partial names or aliases.
- Discover repositories and their metadata within a configured workspace.

### Root Cause: Content Search vs. Entity Resolution

Existing full-text search (`daneel search` / `search_vault`) indexes and scores note contents with BM25. However, agents usually do not need full paragraphs; they need to resolve an **identity or canonical path** (e.g., *"What is the file for Ada Lovelace?"*, *"Where is repo gateway located in ~/Work?"*, *"Which note documents the build system?"*). Furthermore, strict exact matching in `daneel prep` failed when queried with compound names like `"Ada Lovelace"` against notes titled `"Augusta Ada King Noel Byron Lovelace"` with aliases like `(lovelace)`.

A specialized, sub-3ms discovery and entity resolution engine was needed across both CLI and Model Context Protocol (MCP) server interfaces.

## Decision Drivers

* **Sub-3ms Resolution Latency**: Deliver deterministic responses in under 3ms without filesystem traversal overhead.
* **Fuzzy Canonical Path Resolution (`resolve_note` / `daneel resolve`)**: Map ambiguous slugs, titles, and aliases to canonical relative and absolute paths.
* **Compound Name & Alias Matching (`search_people` / `daneel people`)**: Support first name + surname queries, nickname resolution, and faceted filtering by squad, role, and corporate email.
* **Preflight Self-Healing (`daneel prep`)**: Upgrade `PreflightService` to use compound name and alias matching, eliminating false negative failures.
* **Cached Workspace Discovery (`search_projects` / `daneel projects`)**: Maintain a high-performance SQLite cache of `~/Work` repositories, eliminating recursive disk traversals and respecting sandbox boundaries.
* **Faceted Search (`search_vault` / `daneel search`)**: Support typed filtering (`--type`, `--tag`) leveraging structured columns in `notes_meta` with or without full-text queries.
* **Dual Interface Parity**: Expose every discovery capability symmetrically across both the Pure Go CLI and the MCP stdio JSON-RPC server.

## Considered Options

* **Option 1: Rely on Free-Text BM25 Search with Heuristics**: Direct agents to use `search_vault` and parse top snippets.
  * *Disadvantages*: High token cost for agents (1,500-4,000 tokens per search); ranking favors high term frequency in body over canonical title/path matches; cannot filter by Git repository status or clean/dirty states.
* **Option 2: On-Demand Filesystem Sweeps**: Execute in-memory filepath walks on every CLI/MCP command.
  * *Disadvantages*: Latency that grows with the number of repositories in a workspace; violates sandbox read-only boundaries.
* **Option 3: Specialized Discovery & Entity Resolution Engine with SQLite Metadata**:
  * Extend `notes_meta` schema with an `aliases` column and safe schema migration.
  * Create `people_meta` table indexed by `name`, `role`, `squad`, and `email`.
  * Create `workspace_repos` table caching Git status, stack, branch, and metadata from `daneel workspace`.
  * Implement pure Go fuzzy scoring (exact match -> token containment -> substring -> Levenshtein/Jaro-Winkler similarity).
  * Expose four specialized primitives: `resolve_note`, `search_people`, `search_projects`, and faceted `search_vault`.

## Decision Outcome

Chosen option: **Option 3: Specialized Discovery & Entity Resolution Engine with SQLite Metadata**.

### Architectural Details

1. **Database Schema Enhancements (`internal/search/engine.go`)**:
   - `notes_meta`: Added `aliases TEXT` column with automatic migration for existing databases (`ALTER TABLE notes_meta ADD COLUMN aliases TEXT;`).
   - `people_meta`: Persists structured person metadata (`rel_path`, `name`, `role`, `squad`, `email`, `aliases`, `mtime`) populated incrementally during `IndexVault` and `IndexNote`. Auto-hydrates on demand if cache is empty.
   - `workspace_repos`: Persists repository metadata (`rel_path`, `name`, `host`, `abs_path`, `branch`, `is_clean`, `stack`, `description`, `mtime`, `json_data`). Populated incrementally during `daneel workspace` and auto-scanned on first run.

2. **Primitive 1: Fuzzy Note Resolution (`internal/search/resolve.go` & `cmd/resolve.go`)**:
   - Pure Go scoring hierarchy:
     1. Exact relative path match (`score: 1.00`).
     2. Exact slug / filename match (`score: 0.99`).
     3. Exact title match (`score: 0.98`).
     4. Exact alias match (`score: 0.97`).
     5. Query tokens subset of slug or title tokens (`score: 0.88 - 0.95`).
     6. Substring match (`score: 0.76 - 0.80`).
     7. Levenshtein string similarity (`score: similarity * 0.75`).
   - CLI: `daneel resolve <query> [--json]`.
   - MCP Tool: `resolve_note`.

3. **Primitive 2: Colleague Discovery (`internal/search/people.go` & `cmd/people.go`)**:
   - Compound name matcher: Tokenizes queries and checks subset containment against full names (e.g. `"Ada Lovelace"` matches `"Augusta Ada King Noel Byron Lovelace"`).
   - Alias & nickname support: Matches aliases like `"(lovelace)"` or `"lovelace"`.
   - Typo tolerance: Fuzzy token matching with Levenshtein distance $\le 1$ (e.g. `"Turin"` matches `"Alan Mathison Turing"`).
   - Filters: Case-insensitive faceted filtering on `squad`, `role`, and `email`.
   - CLI: `daneel people [query] [--squad <squad>] [--role <role>] [--email <email>] [--json]`.
   - MCP Tool: `search_people`.

4. **Primitive 3: Workspace Repository Search (`internal/search/projects.go` & `cmd/projects.go`)**:
   - Sub-3ms query execution over `workspace_repos` SQLite table.
   - Filtering: by repository name/slug query, dirty uncommitted state (`--dirty`), clean state (`--clean`), and primary stack/language (`--stack`).
   - Auto-population: Automatically cached whenever `daneel workspace` runs, or auto-hydrated on first call.
   - CLI: `daneel projects [query] [--dirty] [--clean] [--stack <stack>] [-l, --limit <limit>] [--json]`.
   - MCP Tool: `search_projects`.

5. **Primitive 4: Faceted Full-Text Search (`internal/search/engine.go` & `cmd/search.go`)**:
   - `SearchFaceted(ctx, SearchOptions{...})`: Accepts optional `Query`, `Scope`, `NoteType`, `Tag`, and `Limit`.
   - Enables metadata-only faceted searches when query string is omitted (e.g. `daneel search --type rfc`, `daneel search --tag ci-cd`).
   - Seamless backwards compatibility with existing `Search(ctx, query, scope, limit)` method.
   - CLI: `daneel search [query] [--type <type>] [--tag <tag>] [--scope <scope>]`.
   - MCP Tool: `search_vault` updated with `note_type` and `tag` parameters.

6. **Preflight Service Self-Healing (`internal/ops/briefing/preflight.go`)**:
   - `PreflightService.GenerateBriefing(query)` upgraded to use the compound name and alias matching engine.
   - Eliminates `no person profile found matching query: 'Ada Lovelace'` failures.

### Verification and Quality Safeguards

- **Unit Tests**:
  - `internal/search/discovery_test.go`: Validates `ResolveNote`, `SearchPeople`, `SearchProjects`, and `SearchFaceted` in under 25ms.
  - `internal/ops/briefing/preflight_test.go`: Validates compound name and alias resolution in `GenerateBriefing`.
  - `internal/mcp/server_test.go`: Validates MCP registration and JSON-RPC dispatch for all discovery tools.
  - `cmd/discovery_test.go`: Validates CLI execution, flag handling, and `--json` formatting.
