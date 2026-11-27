# Web Galaxy Visualizer

The Daneel Galaxy Visualizer transforms thousands of notes, cross-references, and infrastructure components into an interactive WebGL cosmos powered by **Sigma.js** and **Graphology**.

## 1. Core Architecture

The frontend follows a strictly typed, modular design:

- **`GalaxyController.tsx`**: Coordinates Sigma lifecycle, ForceAtlas2 stabilization, LOD recomputation triggers, and event delegation.
- **`web/src/lib/galaxy/` Deep Modules**:
  - **`stellarShader.ts`** (wrapped by `nodeStellarProgram.ts`): Pixel-accurate WebGL1 SDF shader converting framed-graph `v_radius` via `u_correctionRatio * 2.0` into CSS pixels, with crisp photosphere AA, pixel-bounded corona, spherical limb darkening, and procedural granulation on deep zoom.
  - **`smoothCamera.ts`**: Logarithmic zoom-space programmatic camera interpolation (van Wijk & Nuij) and framerate-independent momentum-damped cursor-anchored wheel zoom.
  - **`focusZoomExit.ts`**: Pure 2-step hierarchical auto-unfocus state machine (`Cluster → Sector → Full Galaxy`) on zoom-out without camera reset yank.
  - **`galaxyReducers.ts`**: Extracted `nodeReducer` and zero-allocation `edgeReducer` using precomputed sector/cluster keys and `HIDDEN_EDGE`.
  - **`sectorVisibility.ts`**: Batched Graphology `updateEachNodeAttributes` / `updateEachEdgeAttributes` visibility transitions.
  - **`hoverController.ts`**: O(deg) hover highlighting using `sigmaContext.hoveredNeighborIds` and `partialGraph` + `skipIndexation: true`.
- **`OverlayCanvas.tsx`**: Coalesced 2D Canvas layer (<= 1 draw/frame, shadowBlur-free translucent glows, auto-paused on `document.hidden`) rendering orbital perimeter rings, collapsed super-nodes/bridges, and cosmic photon particles.
- **`BottomTemporalHotbar.tsx`**: Cybernetic HUD hotbar docked at the bottom of the galaxy, providing a weekly timeline (Mon–Sun), activity indicators for daily logs, active day highlight, Weekly Synthesis resolution (`[◈ W{week}]`), and Seldon Vault Anchors for pinned notes with Asimovian overflow popover (`[+N Anchors ▾]`). Supported by global hotkeys (`1` for Today's note, `2` for Weekly Synthesis note).
- **`trafficSimulation.ts`**: Simulates cosmic photon ships navigating edges, strictly respecting constellation and sector visibility filters so zero phantom photons fly in empty space.
- **`nodeStellarProgram.ts`**: Custom WebGL vertex and fragment shaders rendering astrophysical star cores, limb darkening, and coronal gas bloom.
- **`useGalaxyData.ts`**: ConnectRPC data-fetching hook with caching and live reload capabilities.
- **`usePinnedAnchors.ts`**: Dual-origin anchor state hook merging declarative `.daneel.yaml` anchors with client-side `localStorage` pins.
- **`useUIEventSync.ts`**: Streaming event subscriber coordinating remote assistant actions (note opening, sector framing, HUD notifications, galaxy reloading, and real-time hotbar pinning/unpinning) with active context reporting.

## 2. ForceAtlas2 Stabilization Lifecycle

- ForceAtlas2 simulates gravitational attraction and electrostatic repulsion for ~8.5 seconds on initial load.
- **Premature Interaction Guard**: An overlay (`data-testid="cosmic-loading-overlay"`) prevents user clicks during layout convergence to avoid corrupted node positions.
- **Preserve Navigation**: Once layout finishes, camera reset animations are suppressed if the user has already initiated a search or selected a node.

## 3. WebGL Stellar Shader & Smooth Camera Invariants

Located in [`web/src/lib/galaxy/stellarShader.ts`](../../web/src/lib/galaxy/stellarShader.ts) and [`web/src/lib/nodeStellarProgram.ts`](../../web/src/lib/nodeStellarProgram.ts):

> [!IMPORTANT]
> **Pixel-Space SDF & Anti-Blur Coronal Clamping**: In Sigma.js v3 `NodeCircleProgram`, `v_radius` and `v_diffVector` are in normalized framed-graph units, where `1 CSS px = u_correctionRatio * 2.0`. Always convert to CSS pixels first, then clamp coronal gas thickness:
> ```glsl
> float onePx = max(u_correctionRatio * 2.0, 1e-6);
> float radiusPx = v_radius / onePx;
> float coronaPx = clamp(radiusPx * 0.18, 1.5, 3.5);
> ```
> This guarantees that under deep zoom, stars maintain needle-sharp anti-aliased photospheres with spherical limb darkening and procedural surface detail instead of melting into blurry blobs.

- **Log-Space Smooth Zoom (`smoothCamera.ts`)**: Programmatic flights interpolate `ratio` exponentially (`r0 * (r1 / r0)^p`) with zoom-compensated pan weights, while wheel/trackpad events accumulate momentum without frame drops, with `hideEdgesOnMove: false`, `hideLabelsOnMove: false`, and `250ms` debounced `history.replaceState`.

## 4. Loading boundaries

The graph, camera and routine execution hooks remain mounted in App. Search,
shortcuts, cockpit and note inspectors are loaded on first use through local
`DeferredPanel` boundaries. Modal state survives closing, and a failed download
shows a reload action while leaving the galaxy available. Markdown parsing is a
deferred dependency. React and JSX runtime modules stay in the initial React
vendor chunk rather than forcing a Markdown preload.

See [ADR 0028](../adr/0028-deferred-web-panels-and-typed-boundaries.md) for the
boundary and typing decisions.
