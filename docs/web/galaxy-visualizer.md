# Web Galaxy Visualizer

The Daneel Galaxy Visualizer transforms thousands of notes, cross-references, and infrastructure components into an interactive WebGL cosmos powered by **Sigma.js** and **Graphology**.

## 1. Core Architecture

The frontend follows a strictly typed, modular design:

- **`GalaxyController.tsx`**: Manages Sigma camera events, hover handlers, label click detection, and animation synchronization.
- **`OverlayCanvas.tsx`**: High-performance 2D Canvas layer overlaid on the WebGL canvas, rendering orbital perimeter rings, dynamic nebula labels, and cosmic photon particles.
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

## 3. WebGL Stellar Shader Invariants

Located in [`web/src/lib/nodeStellarProgram.ts`](../../web/src/lib/nodeStellarProgram.ts):

> [!IMPORTANT]
> **Anti-Blur Coronal Clamping**: Optical effects (glow, bloom, coronal gas) must **never** scale linearly with camera zoom. Coronal gas thickness is strictly clamped to:
> ```glsl
> float corona = clamp(v_radius * 0.18, 1.5, 3.5);
> ```
> This guarantees that under deep zoom, adjacent fusion cores maintain needle-sharp separation without melting into blurry blobs.

## 4. Loading boundaries

The graph, camera and routine execution hooks remain mounted in App. Search,
shortcuts, cockpit and note inspectors are loaded on first use through local
`DeferredPanel` boundaries. Modal state survives closing, and a failed download
shows a reload action while leaving the galaxy available. Markdown parsing is a
deferred dependency. React and JSX runtime modules stay in the initial React
vendor chunk rather than forcing a Markdown preload.

See [ADR 0028](../adr/0028-deferred-web-panels-and-typed-boundaries.md) for the
boundary and typing decisions.
