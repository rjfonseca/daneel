# ADR 0045: Pixel-Accurate Stellar SDF Shader, Log-Space Smooth Camera, 2-Step Auto-Unfocus & Deep Modules

* Status: accepted
* Date: 2026-11-25

## Context and Problem Statement

While ADR 0044 introduced Semantic Zoom LOD tiers (`overview` / `sector` / `cluster`), three critical rendering and interaction defects degraded the Web Galaxy experience on large vaults (12k+ notes, 25k+ edges):

1. **Blurry Stars on Deep Zoom**: `nodeStellarProgram.ts` treated Sigma.js's `v_radius` and `v_diffVector` as CSS pixels when clamping coronal gas thickness (`clamp(v_radius * 0.18, 1.5, 3.5)`). In Sigma.js v3 (`NodeCircleProgram`), those varyings are actually in **normalized framed-graph coordinates** ($\sim 10^{-3}$ to $10^{-5}$), where 1 CSS pixel equals `u_correctionRatio * 2.0`. As a result, `innerR` permanently clamped to `0.50`, turning the outer 50% of every star into a soft blur proportional to zoom magnification.
2. **Choppy Zoom Animations & Frame Drops**:
   - Sigma's default `MouseCaptor.handleWheel` jumped fixed `1.7x` steps, ignored `deltaY` magnitude, dropped events within `50ms`, and discarded wheel momentum.
   - `Camera.animate` interpolated `camera.ratio` linearly rather than logarithmically, causing sluggish starts and harsh end-snaps across large ratio spans (`1.0` $\rightarrow$ `0.04`).
   - `hideEdgesOnMove` / `hideLabelsOnMove` flickered UI elements on every scroll tick; `camera.on('updated')` called `window.history.replaceState` at 60Hz; `OverlayCanvas` redrew up to 3$\times$ per frame; 2D overlays executed ~100+ `ctx.shadowBlur` Gaussian passes per frame; hover mutated Graphology node attributes triggering full `sigma.refresh()` rebuilds; and sector focus emitted $N + E$ individual Graphology attribute events.
3. **Trapped Focus on Zoom-Out**: Focusing a sector or cluster hid all outside nodes, but zooming out never released focus, shrinking the isolated sector into an empty black void unless the user manually clicked `"Return to Full Galaxy"`.

## Decision Outcome

### 1. Pixel-Accurate SDF Stellar Shader (`web/src/lib/galaxy/stellarShader.ts`)

* Convert framed-graph distances into true CSS screen pixels using `onePx = max(u_correctionRatio * 2.0, 1e-6)` (`radiusPx = v_radius / onePx`, `distPx = dist / onePx`).
* Separate a crisp anti-aliased photosphere disc (`aaPx = 1.15`, `discRadiusPx = max(radiusPx - coronaPx, radiusPx * 0.55)`) from the pixel-bounded outer corona (`coronaPx = clamp(radiusPx * 0.18, 1.5, 3.5)`).
* Blend zoom-adaptive WebGL1 surface fidelity (`zoomDetail = smoothstep(7.0, 22.0, radiusPx)`): spherical limb darkening, thin chromosphere edge, and deterministic procedural photosphere granulation without WebGL2 intrinsics or texture lookups, preserving `PICKING_MODE` parity.

### 2. Log-Space Smooth Camera & Momentum Wheel (`web/src/lib/galaxy/smoothCamera.ts`)

* Interpolate programmatic flights exponentially in log-zoom space (`ratio = r0 * (r1 / r0)^p`) with van Wijk & Nuij zoom-compensated pan weighting (`w = (ratio - r0) / (r1 - r0)`) and `cubicInOut` / `cubicOut` easings.
* Intercept wheel events via Sigma's `mouseCaptor.on('wheel')` + `preventSigmaDefault()`, normalizing pixel/line/page/pinch deltas and driving framerate-independent exponential damping (`1 - exp(-dt / 65)`) anchored to cursor via `sigma.getViewportZoomedState`.
* Set `hideEdgesOnMove: false`, `hideLabelsOnMove: false`, `minCameraRatio: 0.004`, `maxCameraRatio: 1.5`, and debounce `history.replaceState` by `250ms`.

### 3. Bidirectional 2-Step Auto-Unfocus (`web/src/lib/galaxy/focusZoomExit.ts`)

* Record adaptive anchor ratios when focusing a cluster or sector (`calculateFocusExitRatio`).
* On user zoom-out (never during programmatic entry flights or while reading an active note constellation):
  * `Cluster + Sector` $\rightarrow$ first releases `focusedCluster` (revealing sibling clusters in the sector);
  * Further zoom-out releases `focusedSector` $\rightarrow$ **Full Galaxy** in place (preserving `(x, y, ratio)` with no forced `animatedReset()` yank) and immediately restores Semantic Zoom LOD.

### 4. 60fps Hot-Path & Deep-Module Decomposition (`web/src/lib/galaxy/`)

* Decompose monolithic reducer, visibility, hover, camera, and shader logic out of `App.tsx` and `GalaxyController.tsx` into deep, narrow-interface modules under `web/src/lib/galaxy/` (`stellarShader.ts`, `smoothCamera.ts`, `focusZoomExit.ts`, `galaxyReducers.ts`, `sectorVisibility.ts`, `hoverController.ts`).
* Eliminate `ctx.shadowBlur` from 60fps 2D Canvas loops (`orbitalRings.ts`, `superNodeRenderer.ts`, `trafficSimulation.ts`, `canvasMath.ts`), coalesce `OverlayCanvas` to $\le 1\times$ per frame (pausing when `document.hidden`), batch Graphology updates via `updateEachNodeAttributes` / `updateEachEdgeAttributes`, use `partialGraph` + `skipIndexation: true` for O(deg) hover, and short-circuit hidden edges in `edgeReducer` with a shared frozen `HIDDEN_EDGE` sentinel.

## Consequences

* **Positive**: Stars stay razor-sharp with rich spherical detail at arbitrary deep zoom; wheel and focus flights animate smoothly at 60fps; zooming out naturally transitions from Cluster $\rightarrow$ Sector $\rightarrow$ Full Galaxy; and each subsystem is independently unit-testable in milliseconds with minimal AI context overhead.
* **Negative**: Adds modular files under `web/src/lib/galaxy/` that must remain synchronized with `internal/webfs/dist` via `npm run build` on any frontend change.
