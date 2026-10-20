# Unified Command Bridge Implementation Plan

> **For agentic workers:** This plan was approved and implemented task-by-task using `superpowers:executing-plans`. The checkboxes below record completed work.

**Goal:** Replace fragmented upper menus with the selected two-row Command Bridge while retaining existing actions and cosmic styling.

**Architecture:** Compose inline knowledge navigation inside TopCosmicBar. App coordinates one transient disclosure state; a measured CSS boundary positions floating panels below the header. Existing feature controllers, lazy imports, camera calculations, and temporal navigation remain in place.

**Tech Stack:** React 18, TypeScript, Tailwind, existing HudCard/HotkeyBadge, Lucide, Vitest/Testing Library, Playwright.

**Spec:** [Unified Command Bridge](../specs/2026-10-16-command-bridge-design.md)

## Global Constraints

- All application labels, tooltips, accessible names, and documentation are English.
- Use existing React, Tailwind, Lucide icons, HudCard, HotkeyBadge, and design tokens; add no dependencies.
- Preserve existing action callbacks, real statistics, navigation history, saved searches, capture, review, search shortcuts, and lazy-loaded operational panels.
- Preserve camera math, graph rendering, drawer width, temporal navigation, anchors, and persistence semantics.
- Do not add new vault mutations, backend APIs, or global application state frameworks.
- Rebuild embedded assets exclusively through task web-build; verify with task web-verify-sync.

## Review Focus

- A long breadcrumb with large counters must not displace essential controls at narrow widths: Task 3 browser bounds assertions.
- A breakpoint change while a disclosure is open must not leave focus on hidden controls: Tasks 1 and 3 focus checks.
- More → Recent → Escape while a reader is open must preserve the reader and focus More: Tasks 1 and 3 interaction checks.
- Resized header and short landscape viewport must not cover panel controls: Tasks 2 and 3 geometry/scroll checks.
- A dialog opened from More must return focus to a visible control after resizing: Tasks 1 and 3 modal-return checks.

## File and interface map

| File | Responsibility |
| --- | --- |
| `web/src/components/TopCosmicBar.tsx` | One header surface, two rows, responsive groups, More/statistics, measurement |
| `web/src/components/NavigationToolbar.tsx` | Inline navigation, existing history and recent/saved actions |
| `web/src/lib/headerLayout.ts` (new) | Shared HeaderDisclosure and HeaderLayout types, breakpoint resolver |
| `web/src/App.tsx` | Header composition and controlled disclosure state; application CSS variable scope |
| `web/src/components/{KnowledgeExplorer,KnowledgeReviewPanel,ViewControlsPanel,FloatingNoteCard,ArmsInspectorCard}.tsx` | Shared upper boundary and scrollable height |
| `web/tests/{TopCosmicBar,KnowledgeNavigation}.test.tsx` | Action, telemetry, disclosure and focus contracts |
| `web/tests/headerLayout.test.ts` (new) | Responsive mode boundary cases |
| `web/tests/e2e/command-bridge.spec.ts` (new) | Real layout, responsive and interaction verification |
| `web/tests/e2e/ux-feedback.spec.ts` | Preserve existing workflows, adapt compact action lookup when required |
| `docs/adr/0031-unified-command-bridge.md`, `docs/adr/README.md` | Accepted visual contract and log entry |
| `internal/webfs/dist` | Generated assets only |

## Task 1: Compose and wire the responsive Command Bridge

**Interfaces**

- Export `HeaderDisclosure = 'more' | 'recent' | 'saved' | 'stats' | 'view' | null` and `HeaderLayout = 'full' | 'condensed' | 'compact'` from `web/src/lib/headerLayout.ts`.
- Export `resolveHeaderLayout(width: number): HeaderLayout`, returning full at >=1280, condensed at >=768, otherwise compact. Share this resolver between responsive rendering and breakpoint-change dismissal; avoid mismatched CSS/JS thresholds.
- Export the existing `NavigationToolbarProps`. Add controlled `disclosure: HeaderDisclosure`, `onDisclosureChange: (next: HeaderDisclosure) => void`, and `layout: HeaderLayout`; remove its local recent/saved open state and fixed outer surface.
- Extend `TopCosmicBarProps` with `navigation: Omit<NavigationToolbarProps, 'disclosure' | 'onDisclosureChange' | 'layout'>`, `disclosure: HeaderDisclosure`, and `onDisclosureChange: (next: HeaderDisclosure) => void`. Keep the current callbacks and test IDs. Existing isControlsOpen is derived from disclosure at App level.
- App owns `headerDisclosure`; all existing View entry/close paths set that state. Modal/Notes/Review entry callbacks clear it before invoking current feature actions.

- [x] Add behavior tests in `TopCosmicBar.test.tsx` and `KnowledgeNavigation.test.tsx`: every current action calls its supplied callback once, disabled history cannot fire, absent optional callbacks produce no dead actions, statistics preserve zero/unknown values, and platform shortcut labels remain correct. Replace tests that lock the obsolete separately boxed layout with assertions about behavior and reachability.
- [x] Add `headerLayout.test.ts` cases for widths 320, 767, 768, 1279, 1280, and 1440. Test disclosure switching (More → Recent/Saved/View), Escape before reader close, focus fallback to More, outside-click dismissal without focus theft, and responsive dismissal. Use ResizeObserver/matchMedia stubs only as needed to expose real UI transitions.
- [x] Run `cd web && npm test -- tests/TopCosmicBar.test.tsx tests/KnowledgeNavigation.test.tsx tests/headerLayout.test.ts`; confirm new contract tests fail for the missing behavior before implementation.
- [x] Implement shared types/resolver and controlled inline navigation. Build the single two-row HudCard in TopCosmicBar using the spec's action-priority table. Keep one reachable instance per action at each layout. Render real telemetry and an informational statistics disclosure. Use ordinary disclosure buttons, visible focus styles, and existing icons/tokens.
- [x] Update App to mount only the composed header, replace conflicting View boolean state, and close disclosures before dialogs. Preserve lazy imports and established Notes/Review exclusivity. Keep search and cockpit shortcuts wired to current behavior; use controlled disclosure handlers for the global close-controls path.
- [x] Implement disclosure focus handoff, Escape consumption, breakpoint dismissal, and visible-trigger fallback for dialogs launched through More. Do not add a document-wide Escape handler that bypasses existing priority. A focus-return proxy on More may be used when a modal component expects a trigger ref.
- [x] Rerun targeted tests and `cd web && npm run typecheck`. Require passing output; resolve changed prop call sites in tests and application before proceeding.

## Task 2: Establish the shared header boundary

**Interfaces**

- App root receives `data-hud-root`. TopCosmicBar observes its actual outer header bounds and publishes `--hud-content-top` on that root, equal to `header.getBoundingClientRect().bottom + 8` in CSS pixels.
- Consumers use `top: var(--hud-content-top, 128px)` and a viewport-bounded bottom/maximum height. Keep other positioning, reader width and fullscreen rules intact.

- [x] Add a header measurement test: initial fallback is usable; a mocked measured bottom of 116 publishes 124px; subsequent resize to 132 publishes 140px; observer disconnects on unmount. Test that irrelevant repeats do not trigger an update loop.
- [x] Run targeted header tests and confirm the new measurement behavior fails before implementing it.
- [x] Implement mount measurement and ResizeObserver cleanup in TopCosmicBar. Root-scoped CSS publication must survive header rerenders; no modification of body/global CSS outside the app scope. Include the fallback in consumers so lazy panels are correctly positioned before measurement.
- [x] Replace hardcoded top offsets in KnowledgeExplorer, KnowledgeReviewPanel, ViewControlsPanel, header disclosures, normal FloatingNoteCard, and ArmsInspectorCard. Remove conflicting top utilities rather than stacking duplicate rules. Keep maximized reader inset rules and modal layering intact. Short panels scroll internally; preserve bottom temporal bar geometry.
- [x] Run `cd web && npm test -- tests/TopCosmicBar.test.tsx tests/KnowledgeNavigation.test.tsx tests/BottomTemporalHotbar.test.tsx` and `cd web && npm run typecheck`. Require passing output. Geometry correctness is verified in Task 3, not inferred from jsdom.

## Task 3: Verify real interaction, document, and generate delivery assets

**Interfaces**

- `command-bridge.spec.ts` uses the existing neutral isolated backend and accessible UI; no local vault reads/writes.
- Preserve `quick-search-btn`, `capture-note-btn`, `reset-camera-btn`, `toggle-controls-btn`, and existing operational test IDs when actions move to More.
- If a compact action moves, tests open More before selecting it; do not weaken existing reader/URL/focus assertions.

- [x] Write browser cases at 1440x900, 1024x768, 768x1024, 390x844, 320x740, and 844x390. Assert two rows within one surface, all visible control bounds within the viewport, and no intersections between adjacent action targets. Supply long neutral breadcrumb text and large/zero/unknown statistics via test fixtures or controlled component test data, never hardcoded production content. Include 200% zoom-equivalent available width and keyboard reachability.
- [x] Exercise full and compact callbacks: navigate history, open Notes, Recent, Saved, Review, Capture, Search, View, Cockpit, shortcuts, statistics, and Center. Assert resulting real UI state or callback outcome instead of page-load success. Verify More → Recent → Escape retains a previously opened reader, restores More focus, and does not change URL.
- [x] Add breakpoint-resize checks with a disclosure open; verify dismissal and visible focus. Open a dialog from More, resize, close it, and verify focus returns visibly. Confirm search Tab focus containment and Cmd/Ctrl+K remain correct.
- [x] Open normal note reader, satellite inspector, Notes, Review, and View separately; compare their top bounds to header bottom + 8px within 1px rounding tolerance. Verify bottom bounds remain within viewport and final controls can be scrolled into view in landscape. Preserve maximized reader behavior. Capture screenshots for desktop, mobile, open View, and open reader.
- [x] Add the ADR 0031 entry to `docs/adr/README.md`; after approved implementation, change ADR/spec status to accepted/implemented. Describe actual final behavior, including any breakpoint adjustment justified by browser evidence.
- [x] Run `cd web && npm test -- --maxWorkers=2`, `cd web && npm run typecheck`, and `cd web && npm run lint` as separate commands; require all exit codes zero. The default Vitest worker count exceeded the shared container's resource budget in a first full run. Run these before the required generated asset build.
- [x] Run `task web-build`, followed by `task web-verify-sync`; require successful build and no source/generated drift.
- [x] Run `cd web && DANEEL_E2E_EMBEDDED=1 npx playwright test`. This uses isolated port 8088 and keeps the existing hot-reload server on 5173 available. Require the new suite and existing camera, console-error, deferred-panel, and ux-feedback suites to execute and pass; do not silently skip the new coverage.
- [x] Inspect the captured screenshots, record actual results and any limitations, and review the diff for unrelated changes, personal data, or accidental mockup inclusion. Do not claim browser validation from unit checks alone.
- [x] After validated implementation, remove only `.superpowers/brainstorm/606-1792190358/content/menu-prototypes.html` and stop that session with the companion's stop script. Preserve unrelated existing prototype directories. Keep the final spec, ADR, and plan as durable artifacts.

## Review and execution boundary

Implementation is complete and validated in the approved isolated worktree. No commit or push was created.
