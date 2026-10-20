# ADR 0010: Seldon Vault Anchors, Weekly Synthesis Navigation, and Temporal Hotbar Ergonomics

* Status: accepted
* Date: 2026-09-15

## Context and Problem Statement

Daneel's galaxy visualizer includes a bottom temporal bar designed to anchor users in their weekly workflow. User feedback and architectural review revealed four critical areas for improvement:

1. **Weekly Summary Note Navigation**: The previous `WEEK` button merely framed daily log stars within the viewport rather than opening the user's weekly summary / synthesis note (where strategic priorities, delivery tracking, and weekly retrospectives reside). Furthermore, since knowledge vault conventions vary widely (e.g. `weekly/YYYY-W[W].md`, `periodic/weekly/YYYY-WW.md`, `journals/weekly/...`), note resolution must be configurable via `.daneel.yaml` while remaining resilient with fuzzy fallback heuristics.
2. **Visual Inconsistency & Bulk**: The original `BottomTemporalHotbar` used an aggressive `variant="cyan"` styling with oversized vertical pills (56px) and prominent glowing gradients, creating visual dissonance with the sleek, translucent, compact `TopCosmicBar` (`HudCard variant="default" elevation="surface"`, ~44px height).
3. **Redundant Controls & Absence of Custom Anchors**: The current day was already highlighted in the weekly timeline, making a dedicated `[1] TODAY` button redundant. Instead of rigid buttons, users require flexible **Seldon Vault Anchors** to pin high-priority notes (weekly synthesis, project dashboards, strategic charters) directly to the hotbar.
4. **UI Pinning & Overflow Mechanism**: Users need to pin notes directly from the `FloatingNoteCard` interface (stored in `localStorage`), combined with declarative anchors from `.daneel.yaml`. To preserve visual harmony and prevent clutter, the hotbar must enforce a clean visual limit (up to 3 visible anchor slots) and provide an Asimovian overflow popover (`[+N Anchors ▾]`) to view and unpin extra anchors.
5. **Asimovian & Foundation Universe Theming**: All telemetry, chrome, and labels must adhere to the Isaac Asimov / Foundation thematic (e.g. *Galactic Chronometer / Temporal Relay*, *Seldon Vault Anchors*, *Weekly Synthesis*, *Seldon Crypt Archive*), in strict accordance with the English-first open-source invariant.

## Decision Drivers

* Unify the visual language of `BottomTemporalHotbar` with `TopCosmicBar` using the shared Cosmic HUD Design System tokens and `HudCard` primitives.
* Provide multi-tiered resolution for weekly summary notes: explicit template patterns (`hotbar.weekly_pattern`), standard vault conventions, and fuzzy regex matching across vault nodes.
* Replace rigid `TODAY` and `WEEK` action buttons with dynamic **Seldon Vault Anchors**.
* Support dual-origin anchor sources: declarative anchors from `.daneel.yaml` (served via `GetGalaxyResponse.pinned_anchors`) and dynamic user pins toggled in the UI (persisted in `localStorage`).
* Enforce a clean slot limit (max 3 visible anchor pills) with an accessible overflow dropdown popover.
* Maintain Asimov / Foundation lore and terminology across all UI touchpoints.
* Strictly preserve existing camera invariants, accessibility standards (WCAG 2.1 AA), and zero-drift WebFS build synchronization.

## Considered Options

* **Option 1: Hardcode weekly note path and keep rigid TODAY/WEEK buttons**: Assume a fixed path like `weekly/YYYY-WW.md` and only tweak button styles. (Rejected: Incompatible with Daneel's open-source philosophy and breaks on vaults with differing directory layouts; leaves no room for custom pinned notes).
* **Option 2: Dedicated full-screen navigation modal for pinned notes**: Open a separate modal for anchors. (Rejected: High interaction friction; breaks the HUD metaphor and forces users out of the galaxy workspace).
* **Option 3: Unified Cosmic Temporal Relay with Configurable Weekly Synthesis, Seldon Vault Anchors, and LocalStorage Pinning**: Chosen option.

## Decision Outcome

Chosen option: **Option 3: Unified Cosmic Temporal Relay with Configurable Weekly Synthesis, Seldon Vault Anchors, and LocalStorage Pinning**.

### Implementation Details

1. **Backend Configuration (`internal/config/config.go` & `api/v1/graph.proto`)**:
   - Extended `Config` struct with `HotbarConfig` containing `WeeklyPattern string` and `PinnedAnchors []PinnedAnchorConfig`.
   - Updated `api/v1/graph.proto` with `PinnedAnchor` message, `repeated PinnedAnchor pinned_anchors = 4;` and `string weekly_pattern = 5;` in `GetGalaxyResponse`.
   - Wired `cmd/serve.go` to populate protobuf responses from runtime configuration.

2. **Weekly Synthesis Note Resolver (`web/src/lib/weeklyResolver.ts`)**:
   - Implemented `findWeeklySummaryNote(nodes, week, year, customPattern)`:
     1. Evaluates custom pattern if specified (substituting `{{year}}`, `{{week}}` zero-padded, and `{{weekNum}}`).
     2. Tests standard second-brain conventions (`weekly/${year}-W${weekPadded}.md`, `weekly/${year}-W${week}.md`, `periodic/weekly/...`, `journals/weekly/...`, `notes/weekly/...`).
     3. Performs fuzzy heuristic matching on nodes whose path or title contains both `W${week}` (or `week-${week}`) and `${year}`.
     4. Falls back gracefully to weekly cluster framing if no synthesis note exists, emitting an Asimov-themed HUD notification.

3. **Anchor Management Hook (`web/src/hooks/usePinnedAnchors.ts`)**:
   - Merges declarative anchors from backend configuration with local client anchors stored in `localStorage` under `daneel:pinned_anchors`.
   - Dispatches a custom window event (`daneel:pinned_anchors_changed`) to ensure instant cross-component synchronization between `FloatingNoteCard` and `BottomTemporalHotbar`.
   - Provides helper methods: `isPinned(path)`, `togglePin(note)`, and `removePin(path)`.

4. **Redesigned Bottom Temporal Hotbar (`web/src/components/galaxy/BottomTemporalHotbar.tsx`)**:
   - Redesigned using `HudCard variant="default" elevation="surface"` matching `TopCosmicBar`'s 44px compact profile.
   - **Left**: Temporal Relay micro-header with rotating Seldon chronometer glyph (`◈`) and `W{week} · {year}`.
   - **Center**: 7 compact day buttons (Mon–Sun). The active day is cleanly highlighted with a subtle cyan glow and border (`shadow-[0_0_10px_rgba(56,189,248,0.3)]`), eliminating redundant buttons while keeping single-click navigation to daily notes.
   - **Right (Seldon Vault Anchors)**:
     - Primary anchor: **Weekly Synthesis** (`[◈ W{week}]`). Clicking opens the resolved weekly note (or frames week cluster if absent).
     - Up to 2 additional pinned note pills directly accessible on the hotbar.
     - An Asimovian overflow button (`[+N Anchors ▾]`) when total anchors exceed visible slots, toggling a sleek `<HudCard>` popover to view and unpin extra notes.
   - Keyboard shortcut `Alt+1` / `1` focuses today's daily log; `Alt+2` / `2` focuses the Weekly Synthesis note.

5. **In-Card Pin Action (`web/src/components/FloatingNoteCard.tsx`)**:
   - Added a `Pin` action button in the HUD window controls header with tooltip "Pin to Seldon Anchors" / "Unpin from Seldon Anchors".
   - Dynamically highlights active pin status with cyan starlight tint.

## Pros and Cons of the Chosen Option

### Pros
* **Visual Harmony**: Hotbar and TopCosmicBar share identical height, background glassmorphism, borders, and typography.
* **Open-Source Adaptability**: Any user or organization can configure their weekly note pattern without modifying code.
* **Immediate Utility**: Direct jump to weekly priorities with one click or shortcut.
* **Ergonomic Anchoring**: Users can keep critical notes one click away at all times.
* **Lore Immersion**: Foundation / Asimov thematic terminology creates a cohesive, inspiring aesthetic.

### Cons
* If a user configures an invalid custom weekly pattern that matches zero files, the system must fall back to fuzzy heuristics and then cluster framing.
