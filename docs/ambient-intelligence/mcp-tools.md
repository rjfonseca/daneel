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

## 3. Universal Note Ingestion & Capture Tools

Tools for autonomous AI agents to capture structured research, documentation, or host files directly into the Second Brain without shell execution privileges.

### `ingest_note`
Ingests raw markdown content directly into the Vault, synthesizing YAML frontmatter and immediately indexing in SQLite FTS5.
- **Input**:
  - `title` (string, required): Title of the note.
  - `content` (string, required): Markdown body content.
  - `destination` (string, optional): Vault destination directory (default: `'inbox'`).
  - `tags` (array of strings, optional): Tags to attach in frontmatter.
  - `source_url` (string, optional): Source reference URL.
- **Output**: JSON payload with ingestion status, relative path, title, and byte size:
  ```json
  {
    "status": "success",
    "rel_path": "inbox/distributed-consensus-notes.md",
    "title": "Distributed Consensus Notes",
    "size_bytes": 1024,
    "warning": ""
  }
  ```
- **Example Call**:
  ```json
  {
    "name": "ingest_note",
    "arguments": {
      "title": "Distributed Consensus Notes",
      "content": "## Raft Invariants\n- Leader election safety\n- Log matching property",
      "destination": "notes/systems",
      "tags": ["distributed-systems", "consensus"],
      "source_url": "https://example.com/raft-paper"
    }
  }
  ```

### `ingest_path`
Ingests an arbitrary file or directory from the host filesystem into the Vault, supporting recursive scans, frontmatter synthesis, and FTS5 indexing.
- **Input**:
  - `path` (string, required): Absolute or relative host path to file or directory.
  - `destination` (string, optional): Vault destination directory (default: `'inbox'`).
  - `recursive` (boolean, optional): Recursively scan directories (default: `true`).
  - `move` (boolean, optional): Delete source file after successful ingestion (default: `false`).
- **Output**: JSON payload reporting processed notes and any non-fatal warnings:
  ```json
  {
    "total_processed": 1,
    "items": [
      {
        "source_path": "/home/developer/Downloads/architecture-decision.md",
        "rel_path": "notes/architecture/architecture-decision.md",
        "title": "Architecture Decision Record",
        "size_bytes": 2048,
        "warning": ""
      }
    ],
    "errors": []
  }
  ```
- **Example Call**:
  ```json
  {
    "name": "ingest_path",
    "arguments": {
      "path": "/home/developer/Downloads/architecture-decision.md",
      "destination": "notes/architecture",
      "recursive": true,
      "move": false
    }
  }
  ```

## 4. Cognitive Telemetry & Agent Evals Tools

Autonomous self-evaluation and telemetry tools for analyzing agent interactions and detecting knowledge gaps.

### `audit_agent_missed_opportunities`
Audits recent AI agent session history (Antigravity, Claude Code, Cursor) to detect Missed Opportunities (information exists in the vault, but the agent interrogated the user) and Knowledge Gaps (vault lacks information, signaling a missing note or Harvester).
- **Input**:
  - `since` (string, optional): Time window for session analysis (e.g. `'24h'`, `'48h'`, `'72h'`). Defaults to `'48h'`.
  - `agent` (string, optional): Filter by agent framework (`'all'`, `'antigravity'`, `'claude-code'`, `'cursor'`). Defaults to `'all'`.
  - `apply` (boolean, optional): When true, persists discovered learnings to `meta/guias/aprendizados-agente.md` and updates `meta/dashboard-metricas.md`. Defaults to `false`.
- **Output**: Formatted Markdown report detailing scanned interactions, suspicious deltas, missed opportunities with vault notes, and knowledge gaps with recommendations.

## 5. Agent Optimization & Dynamic Execution Tools

### `resolve_tools`
Dynamic Tool Retrieval (Tool-as-RAG) primitive. Resolves and returns matching MCP tool definitions and schemas based on user task intent or keyword queries, preventing context window saturation.
- **Input**:
  - `query` (string, required): Search query or capability description (e.g. `'execute code'`, `'manage workspace'`, `'read colleague'`).
  - `category` (string, optional): Filter by tool domain (`'vault'`, `'people'`, `'workspace'`, `'ui'`, `'tui'`, `'routines'`, `'ingest'`, `'guard'`, `'codeact'`).
  - `limit` (integer, optional): Maximum matching tools to return (defaults to `5`).
  - `format` (string, optional): `'toon'` (default) or `'json'`.
- **Output**: Compact list of matching tool schemas and parameter descriptions.

### `codeact_execute`
Ephemeral, capability-bounded WebAssembly (Wasm) execution sandbox powered by pure Go `wazero` (zero CGO). Executes data transformations, batch filters, and calculations in a single turn.
- **Input**:
  - `wasm_bytes_base64` (string, optional): Base64-encoded Wasm binary module.
  - `wasm_path` (string, optional): Relative path to a pre-compiled Wasm binary module in the vault or workspace.
  - `stdin` (string, optional): Standard input payload (TOON, JSON, or text) fed to the module.
  - `timeout_ms` (integer, optional): Execution deadline in milliseconds (maximum 10000ms, defaults to 5000ms).
- **Behavior**:
  - Runs in a memory-bounded sandbox (16MB linear memory).
  - Isolated from host network and restricted to an ephemeral scratch directory.
  - Audited against the Four Laws of Robotics (ADR 0005).
- **Output**: Standard output and execution summary in TOON or text.

## 6. Token-Oriented Object Notation (TOON) Serialization

All collection retrieval and discovery tools (`search_vault`, `search_people`, `search_projects`, `person_briefing`, `get_active_backlog`, `get_backlinks`, `scan_workspace_repos`, `audit_agent_missed_opportunities`) serialize tabular collections in **TOON (Token-Oriented Object Notation)** by default.

TOON saves 40% to 65% of context window tokens compared to Markdown and JSON by declaring the schema once in the collection header (`results[N]{field1,field2}:`) and transmitting compact delimited rows. It also provides an explicit `[N]` size guardrail against LLM truncation hallucinations.

### Format Parameter
Every retrieval tool accepts an optional `format` parameter:
- `format: "toon"` (**default**): Compact tabular TOON notation.
- `format: "markdown"`: Legacy human-oriented Markdown bullet lists and headers.

Example TOON response for `search_vault`:
```text
results[2]{title,path,scope,type,snippet,score}:
  Architecture Decision,notes/arch.md,all,tecnologia,Distributed service patterns,9.50
  API Gateway,notes/gateway.md,all,rfc,Reverse proxy routing,8.20
```

## 7. Legacy Aliases

For backward compatibility with earlier scripts and prompts, the following aliases remain supported:
- `tui_get_current_state` $\rightarrow$ `ui_get_active_context`
- `tui_open_note` $\rightarrow$ `ui_open_note`
- `tui_flash_notification` $\rightarrow$ `ui_flash_notification`
