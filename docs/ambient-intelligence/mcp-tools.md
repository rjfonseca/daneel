# Unified MCP UI Tools

Daneel exposes a Model Context Protocol (MCP) server over standard input/output (`daneel mcp`) compatible with Claude, Antigravity, Cursor, and other agent platforms.

## 1. Unified `ui_*` Tool Suite

Instead of fragmented toolsets for different clients, Daneel provides unified `ui_*` tools that operate across whichever surface the user is currently active on (`web`, `tui`, or `none`).

### `ui_get_active_context`
Returns the user's current spatial presence:
- **Input**: None
- **Output**: JSON payload:
  ```json
  {
    "surface": "web",
    "active_note_path": "notes/arquitetura/hexagonal.md",
    "focused_sector": "notes/arquitetura",
    "selected_text": "domain layer invariants",
    "updated_at": "2026-09-10T17:30:00Z"
  }
  ```

### `ui_open_note`
Opens and focuses a note on the user's screen.
- **Input**:
  - `rel_path` (string, required): Relative path of the note in the vault (e.g., `notes/ia/agents.md`).
- **Behavior**:
  - **If Web Galaxy is active**: Animates camera to the target star, isolates its constellation, and opens the detail drawer.
  - **If TUI is active**: Switches to the active pane and positions the cursor.
  - **If no UI is running (`surface: none`)**: Automatically falls back to reading the raw Markdown content directly from the vault and returning it to the assistant. The assistant can continue answering without user-facing errors.

### `ui_focus_domain` (alias: `ui_focus_sector`)
Frames the Web Galaxy camera around a specific conceptual knowledge domain, nebula, or group.
- **Input**:
  - `domain` (string, required): Domain, group, or category name to focus on (e.g., `notes/arquitetura`, `sources/gitlab`). Legacy parameter `sector` is supported as an alias.

### `ui_flash_notification`
Renders a cybernetic glassmorphism HUD notification toast on the user's screen.
- **Input**:
  - `message` (string, required): The notification body.
  - `title` (string, optional): Short header title (e.g., `Security Audit`).

### `ui_pin_note`
Pins a high-priority note to the Seldon Vault Anchors temporal hotbar in the active Web Galaxy UI.
- **Input**:
  - `rel_path` (string, required): Relative path of the note in the vault (e.g. `notes/architecture/rfc-agent-os.md`).
  - `label` (string, optional): Short display label for the hotbar pill (defaults to note title or filename).
  - `icon` (string, optional): Glyph or emoji icon (defaults to `📌`).
- **Behavior**:
  - Dispatches `ACTION_PIN_NOTE` to the active Web Galaxy session via daemon IPC.
  - Idempotently updates client-side `localStorage` (`daneel:pinned_anchors`) and re-renders the hotbar immediately.
  - If the UI is offline, informs the agent and provides the declarative configuration snippet for `.daneel.yaml`.

### `ui_unpin_note`
Unpins a note from the Seldon Vault Anchors temporal hotbar in the active Web Galaxy UI.
- **Input**:
  - `rel_path` (string, required): Relative path of the note to unpin.

### `ui_list_pinned_anchors`
Lists all declarative anchors configured in `.daneel.yaml` (`hotbar.pinned_anchors`) and explains dual-origin synchronization.
- **Input**: None
- **Output**: Markdown summary of configured hotbar anchors and weekly synthesis pattern.

## 2. Discovery & Entity Resolution Tools

Specialized primitives designed to eliminate agent fallback to generic, high-latency filesystem inspection tools (`find`, `grep`, `ls`, `list_dir`).

### `search_vault`
Instant (< 3ms) full-text search across the Second Brain using SQLite FTS5, BM25 ranking, and faceted metadata filters.
- **Input**:
  - `query` (string, optional): Search terms or concept.
  - `scope` (string, optional): Scope filter (`all`, `tech`, `meta`, `daily`, `inbox`, `sources`).
  - `note_type` (string, optional): Faceted filter by OKF type (e.g. `rfc`, `adr`, `arquitetura`, `person`, `processo`).
  - `tag` (string, optional): Filter by specific tag (e.g. `kafka`, `ci-cd`).
  - `limit` (integer, optional): Maximum results to return (default: 10).

### `resolve_note`
Fuzzy resolves any note title, slug, or alias to its canonical relative and absolute vault paths in < 2ms.
- **Input**:
  - `query` (string, required): Title, filename, or alias to resolve (e.g. `'scaffold build system'`, `'manifesto reativo'`).
- **Output**: Canonical relative path, absolute path, OKF note type, and confidence score.

### `search_people`
Searches colleague and leadership profiles with compound name matching (e.g. `'Ada Lovelace'` matching `'Augusta Ada King Noel Byron Lovelace'`), nickname resolution, and typo tolerance.
- **Input**:
  - `query` (string, optional): Person name, nickname, or partial compound name.
  - `squad` (string, optional): Tribe or squad filter.
  - `role` (string, optional): Role or title filter (e.g. `'Engineering Manager'`, `'Staff'`).
  - `email` (string, optional): Email filter.

### `search_projects`
Instant search across all repositories in `~/Work` without filesystem traversal overhead, respecting sandbox boundaries.
- **Input**:
  - `query` (string, optional): Repository name, path segment, or service name (e.g. `'daneel'`, `'core-gateway'`).
  - `dirty_only` (boolean, optional): Filter only repositories with uncommitted local changes.
  - `clean_only` (boolean, optional): Filter only clean repositories.
  - `stack` (string, optional): Filter by primary language (e.g. `'go'`, `'python'`, `'typescript'`).

## 3. Cognitive Telemetry & Agent Evals Tools

Autonomous self-evaluation and telemetry tools for analyzing agent interactions and detecting knowledge gaps.

### `audit_agent_missed_opportunities`
Audits recent AI agent session history (Antigravity, Claude Code, Cursor) to detect Missed Opportunities (information exists in the vault, but the agent interrogated the user) and Knowledge Gaps (vault lacks information, signaling a missing note or Harvester).
- **Input**:
  - `since` (string, optional): Time window for session analysis (e.g. `'24h'`, `'48h'`, `'72h'`). Defaults to `'48h'`.
  - `agent` (string, optional): Filter by agent framework (`'all'`, `'antigravity'`, `'claude-code'`, `'cursor'`). Defaults to `'all'`.
  - `apply` (boolean, optional): When true, persists discovered learnings to `meta/guias/aprendizados-agente.md` and updates `meta/dashboard-metricas.md`. Defaults to `false`.
- **Output**: Formatted Markdown report detailing scanned interactions, suspicious deltas, missed opportunities with vault notes, and knowledge gaps with recommendations.

## 4. Legacy Aliases

For backward compatibility with earlier scripts and prompts, the following aliases remain supported:
- `tui_get_current_state` $\rightarrow$ `ui_get_active_context`
- `tui_open_note` $\rightarrow$ `ui_open_note`
- `tui_flash_notification` $\rightarrow$ `ui_flash_notification`
