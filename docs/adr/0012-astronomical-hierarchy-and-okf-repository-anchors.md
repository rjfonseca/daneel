# ADR 0012: Astronomical Hierarchy and OKF Repository Anchors

* Status: accepted
* Date: 2026-09-16

## Context and Problem Statement

Daneel visualizes the second brain graph of notes and infrastructure satellites within a 2D cosmic metaphor using Sigma.js and Graphology.

In production engineering vaults with nested GitLab or GitHub organizations (such as `sources/gitlab/acme/saas/pocs/...`), category detection heuristics were fragile:
1. **False Repository Classification**: Fixed-depth slicing (introduced in ADR 0008 for known subgroups like `saas/` and `apis/`) sliced paths at a fixed index (e.g. `parts[:5]`), mistaking organizational subgroups like `pocs` for repositories. In reality, `pocs` was an organizational group containing further subgroups and repositories.
2. **Lack of Deterministic Anchors**: Without explicit repository markers, the system could not unequivocally distinguish an organizational namespace (a folder of repositories) from a repository root (a software project containing technical documentation and code subfolders).
3. **Astrophysical Metaphor Ambiguity**: The previous visualization treated all directories either as generic "nebulas" or "sectors", without distinguishing between individual code repositories, multi-repo organizational groups, and high-level knowledge domains.

## Decision Drivers

* **Deterministic Boundaries via OKF MOCs**: Establish unequivocal, on-disk Open Knowledge Framework (OKF) Maps of Content (`index.md`) marking repository roots and group directories.
* **Strict Separation of Concerns**:
  - **Vault Markdown Notes**: 100% literal software engineering terms (`type: repository`, `type: group`, `repository: "<name>"`, `group: "<namespace>"`). Never leak cosmic or astronomical jargon (`cluster`, `nebula`, `solar_system`) into Markdown frontmatter.
  - **Web Galaxy Visualizer**: Transforms literal entities into an astrophysically sound, multi-scale cosmic hierarchy.
* **4-Tier Cosmic Hierarchy & Literal Domain Entity**:
  1. **Estrela (Star)**: Individual markdown file / note.
  2. **Aglomerado Estelar (Star Cluster)**: Individual repository, anchored on its `index.md` (Alpha Star / Núcleo do Cluster).
  3. **Nebulosa (Nebula)**: Organizational group or subgroup containing clusters (e.g. `saas/pocs`).
  4. **Setor / Domínio Galáctico (Galactic Domain / Sector)**: Macro knowledge domain (`sources/gitlab`, `notes/arquitetura`, `daily`, etc.), marked as `domain` in schema.
* **Physics-Based Gravitational Cohesion**: Repositories must physically cluster together in WebGL space naturally via ForceAtlas2 physics rather than artificial coordinate clamps.
* **Retroactive Reconciliation**: Allow existing harvested vaults to gain standard OKF MOCs without triggering network re-downloads.
* **Open-Source & English Invariant**: Maintain English-first badges, HUD cards, tooltips, and documentation.

## Considered Options

* **Option 1: Deep Static Heuristics**: Add more hardcoded exclusion arrays and regex patterns for team names.
  - *Drawbacks*: Fragile, unmaintainable, breaks across different organizational conventions.
* **Option 2: Astro-Frontmatter in User Notes**: Write `type: cluster` and `type: nebula` directly in vault Markdown files.
  - *Drawbacks*: Contaminates pure software engineering notes with presentation-layer metaphors; violates OKF interoperability with tools like Obsidian, Logseq, and Hugo.
* **Option 3: Literal OKF MOCs on Disk + Presentation-Time Astronomical Hierarchy (Chosen)**: Use standard `type: repository` and `type: group` MOCs (`index.md`) on disk, resolve taxonomy deterministically in Go backend, and project the 4-tier astronomical hierarchy in the Web Galaxy Visualizer.

## Decision Outcome

Chosen option: **Option 3: Literal OKF MOCs on Disk + Presentation-Time Astronomical Hierarchy**.

### Implementation Details

1. **Protocol Buffers Extension (`api/v1/graph.proto`)**:
   - Added `domain = 11;`, `group = 12;`, `repository = 13;` to `message Node`.
   - Added `focused_domain` to `ReportActiveContextRequest`.
   - Added `ACTION_FOCUS_DOMAIN = 2` with `ACTION_FOCUS_SECTOR = 2` preserved as protobuf enum alias.
   - Regenerated Go and TypeScript bindings via `task proto-gen`.

2. **Harvester & Catalog Reconciliation Engine (`internal/harvester/catalog.go`, `cmd/harvest.go`)**:
   - `EnsureRepositoryIndex`: Generates or updates repository `index.md` with:
     ```yaml
     ---
     type: repository
     title: "<name>"
     repository: "<repo_slug>"
     group: "<namespace>"
     platform: "gitlab" | "github"
     ---
     ```
     Preserves existing repository content and appends catalog links to child documentation.
   - `EnsureGroupIndex`: Generates or updates organizational group `index.md` with:
     ```yaml
     ---
     type: group
     title: "<group_name>"
     group: "<full_namespace>"
     platform: "gitlab" | "github"
     ---
     ```
   - CLI command `daneel harvest catalog` (and `--catalog` flag): walks local `sources/` directories and generates/repairs MOCs for all repositories and intermediate groups without making network requests.

3. **Backend Taxonomy Resolver (`internal/vault/note.go` & `cmd/serve.go`)**:
   - `ResolveTaxonomy(relPath, vaultRoot)`: Prioritizes ancestor `index.md` inspection on disk. If an ancestor has `type: repository`, that folder is unequivocally the repository root. If an ancestor has `type: group`, it is treated as a group nebula.
   - Populates `Node.Domain`, `Node.Group`, and `Node.Repository`.
   - In `ReportActiveContext`: maps `focused_domain` to `ActiveContext.FocusedDomain` (with `FocusedSector` backward compatibility).
   - **Gravitational ForceAtlas2 Cohesion**: Synthesizes structural links between each repository's `index.md` (Alpha Star) and its child document stars, naturally binding the cluster together under ForceAtlas2 layout.

4. **Galaxy Visualizer & Stellar Classification (`web/src/lib`)**:
   - In `stellarClassification.ts`: Nodes with `noteType === 'repository'` or the repo root `index.md` are classified with badge `☀️ CLUSTER CORE` (`size: 2.2`, `spectralClass: 'α-CORE'`). Group MOCs receive badge `🌌 NEBULA CORE`.
   - In `canvasCustomLabels.ts` & `canvasMath.ts`: Multi-scale Level-of-Detail (LOD):
     - Overview Zoom ($\ge 0.40$): High-level Domain HUD Glass Callouts with `◈ DOMAIN` micro-header and neon diamond reticle.
     - Medium Zoom ($0.15 - 0.40$): Cosmic Nebulas (organizational groups).
     - Deep Zoom ($< 0.15$): Star Clusters (repositories anchored on `index.md`) and individual stars.

5. **MCP Server Integration (`internal/mcp/server.go`)**:
   - Registered `ui_focus_domain` (with `ui_focus_sector` as backward-compatible alias) supporting camera framing on domain, group, or repository.

## Pros and Cons of the Chosen Option

### Pros
* **100% Unambiguous Detection**: Any directory containing `type: repository` in `index.md` is definitively a repository, completely eliminating misclassification of subgroups like `pocs`.
* **Clean Separation of Concerns**: Vault markdown files adhere to standard software engineering and OKF semantics, while the Web visualizer retains its cosmic aesthetic.
* **Natural Graph Layout**: Child documentation notes cluster organically around their Alpha Star (`index.md`) via ForceAtlas2 physics without artificial coordinate clamping.
* **Offline Reconciliation**: Vaults can be updated anytime using `daneel harvest catalog`.

### Cons
* Introduces additional `index.md` files at group and repository roots across `sources/`. (Mitigated: these serve as useful entrypoint MOCs in Obsidian and Daneel TUI).
