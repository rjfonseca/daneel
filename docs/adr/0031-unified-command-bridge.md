# ADR 0031: Unified Command Bridge and Shared HUD Content Boundary

* Status: accepted
* Date: 2026-10-16

## Context and Problem Statement

The upper galaxy interface separates brand, telemetry, and action surfaces from a second independently positioned knowledge toolbar. Floating readers and panels use unrelated fixed top offsets. Consolidating the header without changing those offsets would overlap controls and content.

## Decision Drivers

- Follow the selected two-row Command Bridge layout.
- Retain the cosmic HUD primitives, keyboard access, responsive reachability, and real telemetry.
- Preserve graph coordinate contracts, lazy loading, existing callbacks, and temporal navigation.
- Keep positioning consistent when header height changes.

## Considered Options

- Keep independent HUD islands and align their existing offsets.
- Consolidate all actions into one horizontal row with overflow.
- Use one two-row header and publish a shared measured content boundary.
- Use a compact central capsule with most actions behind disclosures.

## Decision Outcome

Use one two-row header with TopCosmicBar as the composition root and inline NavigationToolbar content. App owns a single transient-header-disclosure state including View. Publish the measured header bottom plus an 8px gap through --hud-content-top on the application root, with a 128px initial fallback. Floating panels and normal readers consume the boundary; fullscreen overlays retain existing behavior. Camera geometry and drawer widths remain unchanged.

See [the design](../superpowers/specs/2026-10-16-command-bridge-design.md) for responsive action priorities and focus contracts.

Recent and Saved disclosures render through React portals into the application root. This keeps their fixed positioning outside the glass header's backdrop-filter containing block. Their maximum height uses the shared content boundary and an 8px viewport-bottom gap, with the list scrolling inside the panel.

### Consequences

- Header and panels share an explicit placement contract instead of independent top offsets.
- ResizeObserver requires cleanup and a safe initial fallback.
- Compact layouts put less frequent actions in More; they remain accessible by keyboard and touch.
- Reader placement changes require short-viewport and camera regression coverage.
- This proposal adds no dependency, protocol, persistence format, or backend behavior.
