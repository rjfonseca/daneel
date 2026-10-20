# ADR 0011: Programmatic Seldon Vault Anchor Manipulation via MCP and Daemon IPC

* Status: accepted
* Date: 2026-09-16

## Context and Problem Statement

ADR 0010 introduced **Seldon Vault Anchors** to the bottom temporal hotbar, enabling users to pin high-priority notes directly in the Web Galaxy HUD (persisted in `localStorage` and configured declaratively in `.daneel.yaml`).

However, AI agents operating in collaborative sessions (e.g. via `daneel chat`, Claude Code, Antigravity CLI, or IDE extensions) lacked programmatic capabilities to interact with these anchors in real time. While the agent could read notes and focus them on-screen via `ui_open_note`, it could not:
1. Programmatically pin an important note (such as an active RFC, onboarding checklist, or critical sprint dashboard) to the user's temporal hotbar during a chat session.
2. Unpin notes from the hotbar when a task or review cycle concluded.
3. Query currently anchored notes to establish ambient temporal context.

## Decision Drivers

* **Ambient Co-Pilot Synergy**: The AI agent must be able to directly modify the user's graphical workspace (hotbar HUD) to reduce cognitive load and keep focus anchors aligned with the ongoing conversation.
* **Unified IPC & Event Model**: Anchor manipulation must reuse the existing Unix domain socket daemon and ConnectRPC streaming infrastructure (`SubscribeUIEvents`) without introducing fragmented protocols.
* **Dual-Origin Consistency**: Programmatic pins must seamlessly coexist with declarative anchors from `.daneel.yaml` and manual pins clicked on `FloatingNoteCard`.
* **Zero-Drift & Strong Typing**: Protocol changes must be codified in Protocol Buffers (`graph.proto`) with generated TypeScript and Go code.
* **Open-Source & English Invariant**: All notifications and labels must adhere to the English-first design guidelines.

## Considered Options

* **Option 1: Direct File Modification of `.daneel.yaml` by MCP Tools**: Have the MCP server edit the YAML file on disk whenever a pin/unpin tool is called.
  - *Drawbacks*: Requires server restart or complex file watcher hot-reloading; clutters version-controlled configuration with ephemeral session pins; cannot selectively target a running browser window.
* **Option 2: Browser Extension / HTTP Webhook Bridge**: Build an external webhook receiver in the browser.
  - *Drawbacks*: Breaks local-first security architecture; introduces unnecessary network attack surface and setup friction.
* **Option 3: Daemon IPC UI Events with ConnectRPC Streaming and LocalStorage Dispatch (Chosen)**: Extend `UIActionType` with `ACTION_PIN_NOTE` and `ACTION_UNPIN_NOTE`, stream events to active Web Galaxy sessions, and mirror updates into `localStorage` via existing DOM synchronization events.

## Decision Outcome

Chosen option: **Option 3: Daemon IPC UI Events with ConnectRPC Streaming and LocalStorage Dispatch**.

### Implementation Details

1. **Protocol Buffers Extension (`api/v1/graph.proto`)**:
   - Added `ACTION_PIN_NOTE = 5;` and `ACTION_UNPIN_NOTE = 6;` to `UIActionType` enum.
   - For `ACTION_PIN_NOTE`: `target_path` contains note path, `payload_json` optionally carries `{ "label": string, "icon": string }`.
   - For `ACTION_UNPIN_NOTE`: `target_path` contains note path.

2. **Frontend Event Synchronization (`web/src/hooks/useUIEventSync.ts` & `usePinnedAnchors.ts`)**:
   - Exported `addLocalPinnedAnchor` and `removeLocalPinnedAnchor` helpers.
   - `addLocalPinnedAnchor` idempotently persists the anchor into `localStorage` (`daneel:pinned_anchors`) and dispatches `daneel:pinned_anchors_changed` window event.
   - `useUIEventSync` catches `ACTION_PIN_NOTE` and `ACTION_UNPIN_NOTE` from the ConnectRPC stream, updates local state, and triggers a cybernetic HUD confirmation toast ("Pinned to hotbar: ...").

3. **Model Context Protocol Tools (`internal/mcp/server.go`)**:
   - Added `ui_pin_note`:
     - Parameters: `rel_path` (required), `label` (optional), `icon` (optional).
     - Dispatches `ACTION_PIN_NOTE` over daemon IPC socket.
   - Added `ui_unpin_note`:
     - Parameters: `rel_path` (required).
     - Dispatches `ACTION_UNPIN_NOTE` over daemon IPC socket.
   - Added `ui_list_pinned_anchors`:
     - Returns active declarative anchors (`hotbar.pinned_anchors`) and weekly synthesis pattern from configuration.

## Pros and Cons of the Chosen Option

### Pros
* **Immediate Visual Feedback**: When an agent executes `ui_pin_note`, the hotbar in the user's browser updates in milliseconds, with an Asimovian HUD toast confirmation.
* **Idempotent & Safe**: Calling `ui_pin_note` multiple times for the same note updates metadata without duplicating slots or breaking order.
* **Offline Resilience**: If the Web Galaxy is not running, MCP tools inform the agent gracefully and explain how to persist anchors into `.daneel.yaml`.
* **Zero Configuration Overhead**: No additional ports, services, or credentials required beyond standard Daneel daemon.

### Cons
* Ephemeral pins created via MCP are stored in client browser `localStorage`; users wanting persistent anchors across different browser profiles must declare them in `.daneel.yaml`.
