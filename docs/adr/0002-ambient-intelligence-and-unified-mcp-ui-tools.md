# ADR 0002: Ambient Intelligence & Unified MCP UI Tool Suite

* Status: accepted
* Date: 2026-09-10

## Context and Problem Statement

When using an AI assistant (via MCP in Claude, Antigravity, or Cursor), the assistant lacked awareness of the user's active viewport. Previously, tools were fragmented into narrow TUI-only actions (`tui_open_note`, `tui_get_current_state`), which failed if the user was using the Web Galaxy or had no graphical UI running.

How should Daneel provide shared spatial context between human users and AI assistants without coupling tools to a specific UI framework?

## Decision Drivers

* Single unified tool suite for AI assistants regardless of whether the user is in Web Galaxy, TUI, or headless CLI.
* Graceful fallback: if no graphical UI is active, assistant commands must still succeed (e.g. reading note content directly).
* Real-time bidirectional streaming between Web browser and backend without heavyweight WebSocket stacks.

## Considered Options

* **Option 1: Surface-specific tools (`web_open_note`, `tui_open_note`)**
* **Option 2: Unified `ui_*` tools with dynamic presence detection and offline Markdown fallback via ConnectRPC**
* **Option 3: Polling-based HTTP API**

## Decision Outcome

Chosen option: **Option 2: Unified `ui_*` tools with dynamic presence detection and offline Markdown fallback**, because:
- Assistants interact with a unified interface (`ui_get_active_context`, `ui_open_note`, `ui_focus_sector`, `ui_flash_notification`).
- The daemon detects the active surface (`web`, `tui`, or `none`).
- When `ui_open_note` is called and no UI is open, Daneel reads the Markdown file directly from the Vault and returns its content. The assistant can continue answering without failing or complaining to the user.
- Web UI streaming is backed by **ConnectRPC** (`SubscribeUIEvents` server streaming) with debounced viewport reporting (`ReportActiveContext`).

### Positive Consequences

* Ambient intelligence: The assistant knows exactly what note, sector, or text the user is examining.
* Zero tool failures when switching between Web, TUI, or headless terminal workflows.
* Strong type safety using generated Protobuf schemas (`graph.proto`).

### Negative Consequences

* Requires maintaining client streaming connections in browser (`useUIEventSync`) with reconnect logic.

## Pros and Cons of the Options

### Option 1: Surface-Specific Tools

* Good: Simple 1:1 mapping to UI components.
* Bad: Confuses the LLM; requires the assistant to constantly guess whether the user is in Web or TUI; fails when headless.

### Option 2: Unified `ui_*` Tools with Dynamic Presence

* Good: Optimal DX for LLM agents; handles headless fallback transparently; single contract across all frontends.
* Bad: Requires daemon active context tracking state.
