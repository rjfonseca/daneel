# ADR 0046: Contextual Constellation Dimming + Zoom-Out, Canonical Sectors & Label Decluttering

- **Status:** Accepted
- **Date:** 2026-11-25
- **Deciders:** Engineering / Web Galaxy Visualizer

## Context and Problem Statement

Live visual usability testing of the Web Galaxy visualizer against a 12,000+ note production vault revealed three systemic UX defects:

1. **Nested source sector mismatch:** Macro sector anchors (`__SECTOR_*`) derived their keys from normalized path heuristics (`resolveProjectCategory(cat)`), whereas individual stars sometimes stored deeply nested group strings (e.g., `org/ai/wiki`). Focusing those sectors or unfolding them in Hybrid LOD failed to match child stars, rendering an empty dark canvas under floating nebula callout cards.
2. **Note isolation trap:** Selecting any note activated strict constellation hiding (`hidden: true` for non-constellation nodes) and blocked zoom-out exit, stranding leaf or weakly linked notes in an empty black canvas with no way to zoom back out to the galaxy.
3. **Overlay & HUD collisions:** Unbounded root nebula callout cards, count pills rendered over card titles (`OverlayCanvas` above Sigma labels), un-clamped outward ARMS station callouts behind the 2-row Command Bridge header, frozen hover tooltips across focus transitions, and a `top-20` reset pill buried behind the header degraded navigation clarity.

## Decision Drivers

* Keep Sector Focus and Semantic Zoom LOD consistent across flat and nested repository paths.
* Preserve cosmic context when inspecting a note and allow intuitive scroll-out recovery back to the galaxy.
* Eliminate overlapping text boxes, stray hover artifacts, and HUD chrome occlusion without regressing headless Playwright visual invariants.
* Maintain strict deep-module boundaries (`nebulas.ts`, `focusZoomExit.ts`, `labelLayout.ts`, `galaxyReducers.ts`, `hoverController.ts`).

## Considered Options

* **Option A — Canonical sector normalization, Soft Cosmic Dimming + 3-level zoom-out, AABB LOD culling, Altitude-Damped Log Zoom, and Safe-HUD Label Layout (Chosen):** Introduce `resolveCanonicalSectorKey` for consistent sector matching; dim LOD-visible background stars to low opacity instead of hiding them during constellation focus while adding `'exit-constellation'` to `evaluateFocusZoomExit`; replace single-point centroid LOD culling with world-space AABB bounding-volume viewport intersection in `semanticZoomLod.ts` (populated from member stars after ForceAtlas2 settles); add altitude-adaptive log-space damping, `MAX_LOG_LEAD` clamping, `MAX_LOG_SPEED` throttling, and instant reverse braking in `smoothCamera.ts`; add smooth Super-Node opacity dissolve near unfold boundaries; extract `labelLayout.ts` to sanitize ARMS prefixes, cap overview root labels with progressive zoom reveal, clamp station callouts inside HUD safe bounds, suppress colliding count pills in `superNodeRenderer.ts`, anchor `SectorResetIndicator` safely to `--hud-content-top`, and clear `HoverController` on navigation transitions.
* **Option B — Full node removal on note click with modal-only navigation:** Leaves single-link notes visually disconnected and breaks spatial orientation. Rejected.

## Consequences

* **Positive:** Nested source sectors render child stars and sub-nebulas reliably; selecting any note preserves faint surrounding stars plus 1-hop neighbor labels and exits constellation cleanly on scroll-out without closing the reader drawer; overview label density is bounded and legible; zero stale hover tooltips survive clicks.
* **Negative:** None identified; deep modules remain unit-tested in isolation and byte-synced into `internal/webfs/dist`.
