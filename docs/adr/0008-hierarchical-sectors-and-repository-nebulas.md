# ADR 0008: Hierarchical Sectors and Repository Nebulas

* Status: accepted
* Date: 2026-09-14

## Context and Problem Statement

Daneel indexes and visualizes knowledge graphs and markdown notes across heterogeneous sources (local notes, harvested repositories from GitLab, GitHub, etc.) inside a 2D cosmic metaphor powered by Sigma.js and Graphology.

As vaults scale to include multiple Git repositories organized under multi-group or sub-team structures (such as GitLab organizations with `saas/` and `apis/` subgroups):
1. **Misidentified Repository Roots**: The backend `repoInternalDirs` table previously contained generic code and application directories such as `frontend`, `tools`, `backend`, `src`, `app`, `apps`, `cmd`, `internal`, `scripts`, etc. When a repository was named `frontend` or `tools`, the category detection algorithm stripped the repository name, misidentifying the parent folder as the repository root.
2. **Flat Galaxy Clutter**: In large vaults with dozens of repositories, treating every repository as an independent top-level nebula hub caused severe galactic sprawl and lack of cohesive structure. Repositories belonging to the same product or architectural sector (e.g. `SAAS` or `APIS`) were scattered arbitrarily across the layout.
3. **Label Crowding & Cognitive Overload**: At overview zoom levels, dozens of repository labels competed for visual prominence, causing collision culling to drop critical architectural landmarks.

## Decision Drivers

* Accurate, deterministic Git repository root identification regardless of whether a repository is named `frontend`, `tools`, or `backend`.
* Clear architectural hierarchy: group repositories by organizational or architectural sectors (e.g., `SAAS`, `APIS`).
* Physics-based ForceAtlas2 clustering without visual clutter: repos in the same sector should physically congregate in space.
* Camera Level-of-Detail (LOD) contract:
  - Overview zoom (`camera.ratio >= 0.65`): Render prominent Sector Anchor callouts (`__SECTOR_`) and suppress granular repository labels.
  - Medium/deep zoom (`camera.ratio < 0.65`): Yield sector labels and render individual repository labels (`__CAT_`).
* Strict open-source and English-first UI conventions with no leaks of company-specific or personal identities.

## Considered Options

* **Option 1: Flat category layout with hardcoded exclusion filters**.
* **Option 2: Hierarchical Sectors with ForceAtlas2 Gravitational Anchors and Camera LOD Switching**.

## Decision Outcome

Chosen option: **Option 2: Hierarchical Sectors with ForceAtlas2 Gravitational Anchors and Camera LOD Switching**, because it delivers cohesive architectural clustering and a fluid multi-scale visual experience.

### Implementation Details

1. **Vault Sanitization (`internal/vault/note.go`)**:
   - Sanitized `repoInternalDirs` to retain exclusively documentation, specification, metadata, and dependency directories (`docs`, `specs`, `plans`, `conductor`, `runbooks`, `adrs`, `tracks`, `archive`, `superpowers`, `vendor`, `node_modules`, `dist`, `build`).
   - Enhanced `DetermineCategory` with pattern matching for GitLab structures under `saas/` and `apis/` (e.g., `sources/gitlab/saas/<repo>/...` and `sources/gitlab/<org>/saas/<repo>/...`).
2. **Category & Sector Resolution (`web/src/lib/nebulas.ts`)**:
   - Synchronized `REPO_INTERNAL_DIRS` with the Go backend.
   - Updated `resolveProjectCategory` to return `{ category, displayName, sector, sectorDisplayName, isProject }`.
   - Added `getSectorDisplayName` and updated `calculateNebulaAllocations` to track sector hierarchies.
   - **Strategic Level-1 Notes Invariant**: All direct subdirectories under `notes/` (`isRootNoteCategory(cat) === true`, e.g., `notes/arquitetura`, `notes/ia`, `notes/carreira`) represent core second-brain knowledge pillars and are unconditionally designated as mandatory core sectors (`isOverviewVisible: true`), visible at overview zoom without arbitrary note-count thresholds.
   - **Sector Label Budgeting**: Sectors group project repositories and aggregate their total note counts. Sectors are sorted descending by total note volume and allocated to `overviewVisibleSectors` up to the `maxNebulas` limit (default: 6, configurable via HUD stepper from 1 to 32).
3. **Gravitational Clustering (`web/src/components/galaxy/GalaxyController.tsx`)**:
   - Introduced Sector Anchor nodes (`__SECTOR_${sector}`) in Graphology with high priority and neon styling.
   - Connected repository category hubs (`__CAT_${cat}`) to their corresponding sector anchor with invisible, high-weight gravitational edges (`weight: 25`) in ForceAtlas2.
   - Aligned sector anchor coordinates to the true mathematical barycenter of their member category hubs upon layout stabilization.
   - Dynamically synchronized sector anchor attributes (`label`, `labelDensityWeight`, `TOP` vs `OTHER`) whenever `maxNebulas` changes.
4. **Camera LOD Label Rendering & Strict Budgeting (`web/src/renderers/canvasCustomLabels.ts`)**:
   - Implemented dynamic LOD switching based on `camera.ratio`.
   - At overview zoom (`ratio >= 0.65`), sector anchor callouts are rendered while individual repository labels are suppressed.
   - Sectors strictly respect the `maxNebulas` label budget, incrementing `frameLabelsCount` and suppressing sectors beyond the active threshold (`frameLabelsCount >= maxNebulas`). Strategic level-1 notes (`isRoot`) remain permanently visible as foundational landmarks.
   - Non-top sectors (`!isTop`) are early-culled in overview zoom.
   - At closer inspection (`ratio < 0.65`), sector callouts yield and individual repository labels are displayed.
5. **Interactive Navigation & Filtering**:
   - Updated `enterNode`, `clickStage`, `clickNode`, and camera framing (`zoomToCategoryCluster`) to handle both sector anchors and repository category hubs.
   - Updated the Sector Visibility Filter in `GalaxyController.tsx` and `App.tsx` to maintain visibility of all member repositories when an architectural sector is focused.
6. **Distinct Visual Identity & Cybernetic HUD Callouts (`web/src/lib/canvasMath.ts`, `web/src/renderers/canvasCustomLabels.ts`)**:
   - **Visual Paradigm for Sectors**: Sector anchors (`__SECTOR_`) represent high-level macro territories spanning multiple Git repositories, distinct from local repository or topic nebulas (`__CAT_`).
   - **Cybernetic HUD Glass Badge**: Rendered with deep-space glass styling (`rgba(10, 15, 30, 0.92)`), corner tick accents (`⌜ ⌝ ⌞ ⌟`), dedicated neon micro-header (`◈ SECTOR`), glowing uppercase sector title, and aggregate telemetry (`N repos · M notes`).
   - **Spatial Beacon Reticle**: At the anchor's barycentric coordinates, a spatial beacon reticle (`◈` diamond with a faint radar halo) is drawn to anchor the macro territory in space.
   - **Interactive Gravitational Tethers**: On hovering a sector anchor (`enterNode`), the gravitational edges connecting the sector anchor to member repository hubs illuminate in the sector's neon color (`size: 1.2`), revealing the territorial cluster topology. On `leaveNode`, tether edges reset to hidden.
7. **Dynamic Subgroup Sector Detection (`internal/vault/note.go`, `web/src/lib/nebulas.ts`)**:
   - Replaced hardcoded checks for `saas`/`apis` with dynamic subgroup sector discovery (`knownSectorSubgroups` supporting `stores`, `saas`, `apis`, `platform`, `infra`, `tools`, etc.).
   - Automatically clusters GitLab subgroups at depth 1 or 2 into their corresponding architectural sectors.
8. **Particle Simulator Isolation Guard (`web/src/lib/trafficSimulation.ts`, `OverlayCanvas.tsx`)**:
   - Integrated node visibility predicates (`isNodeVisible`) into `TrafficSimulator.update` and `TrafficSimulator.render`.
   - When a constellation filter or sector isolation is active, ships navigating along hidden edges are immediately culled, and no new photons are spawned on occluded links, preventing phantom particles from navigating empty black space.
9. **Cybernetic Temporal Hotbar (`web/src/components/galaxy/BottomTemporalHotbar.tsx`)**:
   - Bottom-docked HUD widget displaying the current week's timeline (Mon–Sun).
   - Dynamically scans the vault's graph for daily logs (`daily/YYYY-MM-DD.md`), rendering glowing emerald indicators for existing logs.
   - Emphasizes the current day with a cyan Pulsar glow and pulsing badge.
   - Binds global hotkeys `[1]` (jump to today's daily log) and `[2]` (frame the week's daily logs cluster).

### Positive Consequences

* Repositories with names like `frontend`, `tools`, or `backend` are accurately identified at their true repository root.
* Dynamic detection of GitLab subgroups (e.g., `stores`, `platform`, `apis`, `saas`) without manual hardcoding.
* Cohesive, aesthetic clustering of related services and repositories in galactic space.
* Zero phantom photons navigating through empty space when constellation or sector filtering is active.
* Quick, fluid temporal navigation between daily logs and weekly clusters via the Cybernetic HUD hotbar.
* Crisp, readable labels across all camera zoom tiers without visual clutter or label sprawl.
* Immediate visual distinction between high-level architectural sectors and local repository/topic nebulas.
* Strict enforcement of the `maxNebulas` label budget while preserving strategic level-1 note nebulas as permanent cognitive pillars.
