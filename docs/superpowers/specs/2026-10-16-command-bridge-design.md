# Unified Command Bridge

Status: implemented and validated. Layout direction B selected and approved by the user.

## Intent

Replace the fragmented upper HUD with the selected Command Bridge: one continuous two-row surface. Keep the galaxy prominent and retain Daneel's restrained cosmic instrument-panel identity. The bottom temporal bar is the visual reference for grouping and alignment.

The selected prototype establishes the arrangement, not production styles, fixture statistics, or simulated behavior. Prototype files are disposable and must not become application dependencies.

## Scope and constraints

- All application labels, tooltips, accessible names, and documentation are English.
- Use existing React, Tailwind, Lucide icons, HudCard, HotkeyBadge, and design tokens; add no dependencies.
- Preserve existing action callbacks, real statistics, navigation history, saved searches, capture, review, search shortcuts, and lazy-loaded operational panels.
- Preserve camera math, graph rendering, drawer width, temporal navigation, anchors, and persistence semantics.
- Do not add new vault mutations, backend APIs, or global application state frameworks.
- Rebuild embedded assets exclusively through task web-build; verify with task web-verify-sync.

These constraints apply throughout the implementation plan.

## Visual contract

Use one outer HudCard surface, one perimeter border, one restrained shadow, and a thin internal divider. Inner groups use spacing and separators instead of individually boxed buttons. Use existing dark surfaces, muted text, and cyan/indigo accents. Reserve strong accents for selection, focus, and Capture. Use Lucide icons rather than mixed emoji. Do not introduce decorative motion.

The outer header remains 12 CSS pixels from the viewport edges, including on mobile. Row one contains brand/status, the existing Galaxy / Sector / Cluster breadcrumb, and actual telemetry. Its far-right action is Center galaxy. Row two contains navigation on the left and search/actions on the right. Both rows share their outer padding and vertical centers. Icon targets are at least 36 by 36 CSS pixels on desktop and 44 by 44 on compact layouts. Retain visible keyboard focus and reduced-motion behavior.

Desktop row two order: Back, Forward, separator, Notes, Recent, Saved, Review, flexible space, Search, separator, Capture, View, More. Capture remains directly accessible. More contains Cockpit and Keyboard shortcuts. Center galaxy lives only in row one, apart from the existing Galaxy breadcrumb's overview action.

Telemetry shows Stars, Filaments, and Harvesters using existing data and accessible descriptions. Zero remains zero; unavailable data remains an em dash. Do not imply backend health from the existing decorative status indicator; label it as the local vault context, not a verified network connection.

## Responsive behavior

Breakpoints below refer to viewport width; header spans the viewport regardless of a side drawer.

| Width | Row one | Row two | More |
| --- | --- | --- | --- |
| >= 1280px | Full brand, breadcrumb, all telemetry, Center | Full desktop order | Cockpit, Keyboard shortcuts |
| 768–1279px | Full brand, truncated breadcrumb, Stars, Center | Back/Forward, Notes, Search, Capture, View, More | Recent, Saved, Review, Cockpit, Keyboard shortcuts, Galaxy statistics |
| < 768px | Brand mark with accessible name, truncated breadcrumb, Center | Notes, icon-only Search and Capture, View, More | Back, Forward, Recent, Saved, Review, Cockpit, Keyboard shortcuts, Galaxy statistics |

Galaxy statistics in More opens a small informational disclosure using the same live data. At reduced widths, moving an action to More must retain its disabled state, accessible name, and callback. Hidden duplicates must not remain focusable. Search and Capture icon buttons retain their existing accessible names and test IDs. The brand text and decorative status may collapse before reducing interaction target size. Breadcrumb ancestors remain operable, and truncated labels expose their full names accessibly. No third row, clipped controls, or horizontal scrolling at 320px or above. Text at 200% zoom must remain reachable using the compact layout.

Desktop layout is a starting breakpoint decision made for this spec, not a user-specified pixel preference. Verify and adjust breakpoints upward if actual font metrics cause collisions, preserving the action-priority table and minimum target sizes.

## Interaction contract

Use ordinary buttons and labelled navigation regions. More is a disclosure containing buttons, not an ARIA menu requiring arrow-key semantics. Preserve the Knowledge navigation region and Cosmic Breadcrumbs naming. Recent and Saved remain labelled regions with their existing content and callbacks.

Only one transient header disclosure is open at a time: More, Recent, Saved, statistics, or View. Selecting Recent/Saved from More replaces the disclosure rather than layering nested flyouts. Opening Search, Capture, Cockpit, or shortcuts dismisses header disclosures before opening the existing dialog. Notes and Review remain mutually exclusive; activating either closes header disclosures. An existing reader may remain open behind a disclosure.

Escape dismisses the active disclosure before any underlying reader and restores focus to its visible trigger. Recent/Saved opened through More restore focus to More. Outside pointer dismissal closes without stealing focus from the clicked target. Switching disclosures focuses the new disclosure's first meaningful control. Modal dialogs preserve focus containment and restore focus to their visible invoking control. If resizing hides that control, use the visible More trigger as fallback. Do not restore focus to a hidden element.

Resizing across a breakpoint dismisses header disclosures and restores focus to an equivalent visible trigger or More. Retain existing global shortcut behavior, including Cmd/Ctrl+K, Alt+C, and ?. Opening View uses the current settings panel with all existing controls and descriptions.

## Placement and overlap

The header publishes its measured bottom edge plus an 8px gap as --hud-content-top on the application root. Use ResizeObserver on the header, an initial 128px fallback, and cleanup on unmount. Measure on mount and whenever size changes; avoid React render loops. This measures DOM geometry only and does not affect Sigma coordinate math.

Replace independent top offsets in KnowledgeExplorer, KnowledgeReviewPanel, ViewControlsPanel, Recent/Saved/statistics disclosures, the normal FloatingNoteCard, and ArmsInspectorCard with this shared value. Constrain their bottom edges and allow internal scrolling, including 844x390 landscape. Keep existing widths and drawer horizontal positioning. A maximized reader or modal dialog may intentionally cover the header; preserve its existing modal/fullscreen behavior. Toasts retain their existing behavior.

The bottom temporal bar retains its structure and behavior; align styling through existing shared HUD primitives without expanding this task into a calendar redesign. Existing drawer compensation and camera invariants remain unchanged.

## Component responsibilities

- TopCosmicBar owns the single header surface, row layout, More/statistics disclosures, and measured header boundary. Extend its existing props with navigation: NavigationToolbarProps.
- NavigationToolbar becomes inline content: remove its fixed positioning and outer HudCard, export NavigationToolbarProps, and accept compact overflow composition through the parent. It retains recent/saved data rendering and action wiring.
- Coordinate disclosure state with a controlled header disclosure type: 'more' | 'recent' | 'saved' | 'stats' | 'view' | null. App owns this state because View already lives at App level; derive isControlsOpen from it rather than maintaining conflicting booleans. Pass the controlled value and change callback to the header and navigation content. Existing keyboard close-controls behavior sets it to null.
- App mounts NavigationToolbar through TopCosmicBar instead of as a separate fixed sibling. It still owns feature state and callbacks. Do not eagerly import lazy feature panels.
- Existing floating panels consume the placement variable. No general overlay framework is needed.

Exact style utilities and helper extraction within these components are implementation choices. Changes to the visual placement contract are recorded in the accepted ADR 0031.

## Acceptance and validation

1. One two-row header surface at 1440x900, 1024x768, 768x1024, 390x844, 320x740, and 844x390. No horizontal overflow or intersecting control bounds. Include a long sector/cluster path and 200% zoom equivalent.
2. Every current action remains reachable in desktop and compact layouts and invokes the existing behavior exactly once; no duplicate enabled controls for the same action.
3. More → Recent/Saved, View switching, outside click, Escape, breakpoint resizing, and modal focus return behave as specified, including with a note already open.
4. Notes, Review, View, normal reader, and satellite inspector start below the header boundary, stay within the viewport, and scroll internally on short screens. Fullscreen overlays retain their intended behavior.
5. Actual statistics, unknown statistics, zero values, disabled history, empty Recent/Saved, English tooltips, and platform-specific search shortcuts remain correct.
6. Camera invariants, bottom-bar date/anchor behavior, URL history, explorer batching, search focus containment, deferred panels, and console-error suites pass.
7. Run unit tests, typecheck, lint, generated asset build, WebFS verification, and production embedded Playwright. Use the isolated embedded profile while the user's dev server occupies port 5173. Inspect captured screenshots before declaring the layout verified.

## Delivery boundary

This document and its plan are durable records of the approved, implemented design. After validation, remove only this exploration's prototype file and stop its companion session. Preserve unrelated .superpowers content. Do not commit disposable mockups. Keep the durable spec, ADR, and implementation plan.
