# ADR 0044: Semantic Zoom Tiers, Progressive Disclosure & Super-Node Aggregation

* Status: accepted
* Date: 2026-11-25

## Context and Problem Statement

In large knowledge vaults containing thousands of harvested third-party documents under `sources/` alongside curated personal knowledge under `notes/`, `daily/`, `meta/`, and `inbox/`, the Web Galaxy visualizer previously rendered **100% of all star nodes simultaneously** during panoramic overview and free pan/zoom navigation. While edge filtering (`edgeFiltering.ts`) and HUD callout labels (`canvasCustomLabels.ts`) already implemented Level of Detail (LOD), `nodeReducer` in `App.tsx` kept every memory node visible unless an explicit sector, cluster, or constellation was clicked.

Because harvested repositories and issue trackers commonly account for ~85–90% of total Markdown files, dense clusters of low-degree leaf stars obscured the curated Second Brain core and degraded visual scannability.

## Decision Drivers

* **Progressive Visual Clarity**: Start clean in panoramic overview and unfold external `sources/` detail strictly on demand as the user zooms in or pans over a region, re-collapsing automatically on zoom-out or pan-out.
* **Alignment with ADR 0042 Trust Tiers**: Mirror the backend `vault_index.db` (curated knowledge) vs. `sources_index.db` (external sources) boundary in the Web Galaxy via explicit `Curated`, `Hybrid` (default), and `All` Knowledge Scope controls.
* **Zero ForceAtlas2 & Search Disruption**: Avoid mutating Graphology node visibility or injecting artificial super-edges into the ForceAtlas2 simulation so physics convergence, global search (`Cmd+K`), and k-hop constellation isolation remain fast and intact.
* **60fps Interactive Frame Budget**: Evaluate LOD in O(1) per node inside Sigma's `nodeReducer` using precomputed attributes and coalesced `requestAnimationFrame` camera updates, with hysteresis and asymmetric viewport margins to eliminate flicker.

## Considered Options

* **Option 1: Backend-Filtered Subgraph Loading on Pan/Zoom**:
  * *Disadvantages*: Breaks instantaneous client-side k-hop constellation isolation, requires repeated network round-trips and ForceAtlas2 re-stabilization on camera motion, and increases backend coupling.
* **Option 2: Frontend-First Semantic Zoom Engine with Collapsed Super-Nodes & Aggregated Bridges (Chosen)**:
  * Precomputes `trustScope`, `sectorKey`, `clusterKey`, `SuperNodeMeta`, and `SuperEdgeIndex` once at graph load.
  * Evaluates a 3-tier state machine (`overview` $\leftrightarrow$ `sector` $\leftrightarrow$ `cluster`) with camera ratio hysteresis (`0.50/0.58` and `0.20/0.26`), asymmetric viewport frustum bounds (`±10%` enter / `±35%` exit), and bounded expansion budgets (`MAX_EXPANDED_SECTORS = 8`, `MAX_EXPANDED_CLUSTERS = 12`).
  * Renders collapsed `sources/` groups as proportional WebGL cores + 2D Canvas radial nebula halos, concentric orbital rings, activity arcs, count pills, and curved Bézier aggregated super-edges.

## Decision Outcome

Chosen option: **Option 2: Frontend-First Semantic Zoom Engine with Collapsed Super-Nodes & Aggregated Bridges**.

### 1. Knowledge Scope Layering (`semanticZoomLod.ts`, `ViewControlsPanel.tsx`)

* **`curated`**: Displays only curated vault notes (`notes/`, `daily/`, `meta/`, `inbox/`, `public/`) and ARMS perimeter stations; hides `sources/*` stars, super-nodes, and super-edges.
* **`hybrid` *(default)***: Keeps curated stars permanently visible while applying 3-tier Semantic Zoom progressive disclosure to `sources/*`.
* **`all`**: Disables tier collapsing so all stars are rendered directly.

### 2. Three-Tier Semantic Zoom & Viewport Frustum Culling

* **Tier 0 — Overview (`ratio >= 0.50`, hysteresis return `> 0.58`)**: All `sources/*` stars collapse into Sector Super-Nodes (`__SECTOR_*`).
* **Tier 1 — Sector (`0.20 <= ratio < 0.50`, hysteresis return `> 0.26`)**: Only sectors inside the viewport unfold into Repository Cluster Super-Nodes (`__CAT_*`) and high-signal anchor stars (Alpha cores, Quasars, degree $\ge 5$).
* **Tier 2 — Cluster (`ratio < 0.20`)**: Only clusters inside the viewport unfold to reveal 100% of their individual stars. Panning out beyond the `35%` exit margin or zooming out smoothly re-collapses them.

### 3. Override Precedence

1. **ARMS Perimeter Toggle**: Hides/shows outer foundation stations.
2. **Constellation Isolation (`computeConstellation`)**: Always reveals the selected note and its 2-hop neighborhood regardless of scope or LOD tier.
3. **Explicit Sector / Cluster Focus**: Clicking a Super-Node or HUD callout frames (`65%–90%` viewport) and unfolds all member stars of that sector or cluster.

## Consequences

* **Positive**: Reduces initial overview node clutter by ~85–90% in large multi-source vaults while preserving full graph search, ForceAtlas2 layout stability, and instant drill-down.
* **Positive**: Weighted aggregated super-edges reveal structural cross-links between curated architecture/engineering notes and collapsed external repositories without hairball overlap.
* **Negative**: Client still downloads the full `GetGalaxy` graph payload once on initial load; future vaults exceeding ~50k notes may layer incremental streaming on top of this frontend LOD model.
