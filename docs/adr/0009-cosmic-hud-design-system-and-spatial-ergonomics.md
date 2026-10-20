# ADR 0009: Cosmic HUD Design System, Spatial Ergonomics, and Accessibility Guardrails

* Status: accepted
* Date: 2026-09-15

## Context and Problem Statement

Daneel renders a developer second-brain and knowledge galaxy via Sigma.js (WebGL), 2D HTML5 Canvas overlays, and React DOM HUD chrome. While visually striking, an extensive architectural and UX audit identified four systemic deficiencies:

1. **Dual-Engine Design Token Void**: There was no Single Source of Truth (SSOT) shared between WebGL/Canvas 2D renderers and React DOM components. `web/tailwind.config.js` had an empty `extend: {}` configuration, while 10+ distinct dark hex values (`#050508`, `#08090f`, `#0a0f1e`, `#0b0c13`, `#0c0d14`, `#0c0d15`, `#13131a`, etc.) were scattered ad-hoc across components.
2. **Dead Scaffolding**: 80% of `web/src/components/ui/` consisted of unused corporate SaaS shadcn primitives (`button.tsx`, `drawer.tsx`, `sheet.tsx`, `input.tsx`, `textarea.tsx`, `input-group.tsx`), while active components repeatedly hand-rolled 50–100 character strings of inline utility classes.
3. **Viewport Collision & Spatial Ergonomics**: On viewports $< 1440\text{px}$, opening the 440px right-side note drawer (`z-40`) completely occluded the right side of the bottom temporal hotbar (`z-20`), burying key action buttons. While the camera mathematically shifted focal targets to `openCenterX = (width - 440) / 2`, the bottom HUD remained rigidly centered at 50% screen width. Additionally, interactive canvas callout boxes lacked a `pointer` cursor on hover.
4. **Accessibility (a11y) Barriers**: Severe contrast failures existed in stellar classifications (Brown Dwarf at 2.07:1, Red Dwarf at 4.30:1), all interactive HUD buttons lacked `focus-visible` rings, single-character global shortcuts (`1` and `2`) violated WCAG 2.1.4 by conflicting with screen reader heading navigation, and over 12 icon buttons lacked accessible `aria-label` attributes.

## Decision Drivers

* Establish a Single Source of Truth (SSOT) token system shared across WebGL shaders, Canvas 2D math, and React DOM Tailwind styles.
* Create a dedicated atomic HUD primitive library (`HudCard`, `NeonBadge`, `CyberButton`, `HotkeyBadge`, `ShortcutsModal`) under `web/src/components/hud/`.
* Clean up dead component scaffolding.
* Eliminate spatial collisions between the 440px drawer and bottom hotbar via adaptive shift & dock math aligned with camera center.
* Provide clear visual affordance (`cursor: pointer`) on all interactive canvas labels and callout cards.
* Achieve full WCAG 2.1 AA compliance for color contrast, visible focus rings (`.cosmic-focus-visible`), and non-conflicting modifiers (`Alt+1`, `Alt+2`).
* Strictly preserve existing camera coordinate contracts (AGENTS.md) and Playwright E2E invariant tests.

## Considered Options

* **Option 1: Ad-hoc Tailwind utility patches without primitives**: Keep adding inline utility classes and tweak colors in `tailwind.config.js`. (Rejected: Fails to solve code duplication, visual drift, or WebGL/Canvas color synchronization).
* **Option 2: Replace HUD with a third-party UI library (e.g. Radix/Base-UI)**: Adopt a heavyweight UI library. (Rejected: Adds substantial overhead, conflicts with custom WebGL canvas overlays, and does not suit the deep-space cybernetic HUD aesthetic).
* **Option 3: Cosmic HUD Design System with Shared SSOT Tokens, Dedicated Atomic Primitives, and Spatial Ergonomics Alignment**: Chosen option.

## Decision Outcome

Chosen option: **Option 3: Cosmic HUD Design System with Shared SSOT Tokens, Dedicated Atomic Primitives, and Spatial Ergonomics Alignment**.

### Implementation Details

1. **Shared Token System (`web/src/design/tokens.ts`)**:
   - Centralizes colors: `void` (`#050508`), `surface` (`#0a0d18`), `surfaceElevated` (`#0f1424`), `surfaceMuted` (`#141a2e`).
   - Standardizes spectral and sector accents: `starlight` (cyan `#38bdf8`), `quantum` (indigo `#6366f1`), `foundation` (purple `#a855f7`), `cadence` (amber `#f59e0b`), `biolum` (emerald `#10b981`), `supernova` (rose `#fb7185`).
   - Centralizes shadows (`glow-cyan`, `glow-indigo`, `hud-panel`, `hud-deep`), border radii, and backdrop blurs.
   - Wired directly into `web/tailwind.config.js` via `theme.extend`.

2. **Atomic HUD Primitives (`web/src/components/hud/`)**:
   - `HudCard`: Standard glassmorphism container with semantic border variants (`cyan`, `indigo`, `purple`, `amber`), elevation states, and optional sci-fi corner brackets (`techCorners`).
   - `NeonBadge`: Standardized spectral and sector status pills with optional pulsing radar dot.
   - `CyberButton`: Accessible interactive button with `.cosmic-focus-visible`, hover glow, active states, and size variants.
   - `HotkeyBadge`: Monospace keyboard shortcut indicator (`⌘K`, `⌥1`, `?`).
   - `ShortcutsModal`: Accessible cheatsheet dialog triggered via `?` (Shift+/) or help button, listing all global bindings.
   - Purged dead code in `web/src/components/ui/` (`button.tsx`, `drawer.tsx`, `sheet.tsx`, `input.tsx`, `textarea.tsx`, `input-group.tsx`), while preserving `command.tsx` and `scroll-area.tsx`.

3. **Spatial Ergonomics & Viewport Shift-and-Dock**:
   - In `BottomTemporalHotbar.tsx`, whenever `activeNote` or `activeSatellite` is open on desktop viewports ($\ge 1024\text{px}$), the hotbar center dynamically shifts from `left: 50%` to `left: calc((100vw - 440px) / 2)`. This matches the camera's `openCenterX` and eliminates occlusion with the right drawer.
   - In `GalaxyController.tsx`, added a `stageMouseMove` listener to dynamically switch `sigma.getCanvas().style.cursor` between `pointer` and `default` when hovering over sector beacons, category hubs, or satellite callouts.
   - Opening a note drawer automatically dismisses `CosmicControlsPopover` to prevent z-index collisions.

4. **Accessibility (a11y) & Contrast Remediation**:
   - Upgraded Brown Dwarf to `#d97706` (alpha 0.85) and Red Dwarf to `#fb7185`, guaranteeing WCAG AA contrast compliance ($> 4.5:1$).
   - Standardized `.cosmic-focus-visible` utility in `web/src/index.css` with 2px indigo outline and offset.
   - Replaced single-key shortcuts `'1'` and `'2'` with modifier combos `Alt+1` / `Alt+2` (`⌥1` / `⌥2`), satisfying WCAG 2.1.4.
   - Added explicit `aria-label` attributes to all icon-only buttons across `TopCosmicBar`, `FloatingNoteCard`, `ArmsInspectorCard`, and `CosmicControlsPopover`.
   - Added `role="status"` and `aria-live="polite"` to loading overlays and notification toasts.
   - Added `motion-reduce:animate-none` to protect users with vestibular disorders.
