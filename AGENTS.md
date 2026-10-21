# Daneel Engineering Guide & Invariants (AGENTS.md)

This document establishes architectural directives, coordinate contracts, shader rules, open-source language conventions, and regression safeguards for any agent or engineer working on **Daneel**.

---

## 1. Documentation Map of Content (MOC)

Daneel employs a **Progressive Disclosure** documentation model. Agents must load specialized docs on demand based on task context:

| Context / System | Reference File | When to Load |
|---|---|---|
| **System Overview** | [`docs/architecture/overview.md`](docs/architecture/overview.md) | Understanding data stores, SQLite FTS5 engine, and core subsystems |
| **Search & Storage** | [`docs/architecture/search-and-storage.md`](docs/architecture/search-and-storage.md) | Modifying SQLite FTS5, BM25 scoring, schema, or tokenizers |
| **Security Guardrails** | [`docs/architecture/security-guardrails.md`](docs/architecture/security-guardrails.md) | Working with `daneel guard` or the Four Laws of Robotics audit rules |
| **Coordinate Math** | [`docs/architecture/coordinate-spaces.md`](docs/architecture/coordinate-spaces.md) | Working with camera zoom (`camera.ratio`), lateral drawer offset ($dx$), or cluster framing |
| **Daemon Architecture** | [`docs/daemon/lifecycle.md`](docs/daemon/lifecycle.md) | Working with `daneel daemon`, Unix socket IPC, PID lockfile, or `systemd --user` |
| **Daemon Debugging** | [`docs/daemon/debugging.md`](docs/daemon/debugging.md) | Troubleshooting socket errors, stale locks, netcat probes, or systemd logs |
| **Spatial Presence** | [`docs/ambient-intelligence/spatial-context.md`](docs/ambient-intelligence/spatial-context.md) | Working with `ActiveContext`, ConnectRPC streaming, or viewport synchronization |
| **Agent Hooks** | [`docs/ambient-intelligence/agent-hooks.md`](docs/ambient-intelligence/agent-hooks.md) | Passive agent telemetry spooler, raw-to-inbox compiler, and ambient presence |
| **MCP Tool Suite** | [`docs/ambient-intelligence/mcp-tools.md`](docs/ambient-intelligence/mcp-tools.md) | Working with `ui_get_active_context`, `ui_open_note`, `ui_focus_sector`, `ui_flash_notification` |
| **Cognitive Dream Cycle**| [`docs/cognitive/dream-cycle.md`](docs/cognitive/dream-cycle.md) | Working with `daneel dream`, Zettelkasten triage, or cognitive learning loop |
| **Configuration** | [`docs/configuration/schema.md`](docs/configuration/schema.md) | Modifying `.daneel.yaml` schemas, archetypes, or multi-vault resolution |
| **Operations Suite** | [`docs/operations/telemetry-and-briefing.md`](docs/operations/telemetry-and-briefing.md) | Working with `dossier`, `prep`, `workspace`, `clone`, `ingest`, or `log` |
| **Terminal UI (TUI)** | [`docs/tui/terminal-interface.md`](docs/tui/terminal-interface.md) | Modifying Bubble Tea terminal views, keybindings, or rendering |
| **CLI & Delegation** | [`docs/cli/delegation.md`](docs/cli/delegation.md) | Adding CLI commands or modifying `RunOrDelegate` and live reload flows |
| **CLI Reference** | [`docs/cli/reference.md`](docs/cli/reference.md) | Checking command flags, usage syntax, and global options |
| **Visualizer & Shaders** | [`docs/web/galaxy-visualizer.md`](docs/web/galaxy-visualizer.md) | Modifying Sigma WebGL rendering, ForceAtlas2, or `nodeStellarProgram.ts` |
| **Astrophysics & LOD** | [`docs/web/lod-and-astrophysics.md`](docs/web/lod-and-astrophysics.md) | Modifying the 8-class stellar taxonomy or dynamic zoom edge filtering |
| **Harvesters** | [`docs/harvester/pipeline.md`](docs/harvester/pipeline.md) | Working with GitHub, GitLab, Google Drive, or Jira scraping engines |
| **Dev & Quality Gates** | [`docs/development/quality-and-testing.md`](docs/development/quality-and-testing.md) | Running Lefthook checks, Vitest suites, or Playwright E2E invariant tests |
| **Architectural Decisions** | [`docs/adr/README.md`](docs/adr/README.md) | Consulting or creating Architectural Decision Records (MADR format) |

---

## 2. Strict Invariants

### 2.1 Mandatory Architectural Decision Records (MADRs)

> **Strict Invariant**: **Always produce ADRs (MADR format) for architectural decisions.**
> - Every non-trivial architectural decision, protocol definition, daemon lifecycle change, IPC design, or graph invariant **MUST** be recorded as a MADR under `docs/adr/`.
> - PRs, feature branches, or commits modifying system architecture or introducing new subsystems without an accompanying ADR are strictly prohibited and will be rejected.
> - ADRs must follow the Markdown Architectural Decision Records (MADR) template as established in [`docs/adr/README.md`](docs/adr/README.md).

### 2.2 Open-Source & English UI Invariant

> **Strict Invariant**: **Daneel is an English-first open-source project.**
> - All user-facing UI labels, button texts, HUD cards, popovers, badges, and **tooltips** MUST be written in clear, concise **English**.
> - Even though personal notes inside a user's vault may be authored in Portuguese or other languages, the application chrome, telemetry, controls, and tooltips are strictly English-only.
> - Never hardcode Portuguese or non-English strings into UI templates, tooltips, or error fallbacks.

### 2.3 Strict Neutrality & Zero Corporate / Personal Data Leak Invariant

> **Strict Invariant**: **Daneel is a clean, neutral open-source project with zero exposure of personal workflow, colleagues, or corporate employers.**
> - Never reference current or former employers or private corporate entities anywhere in code, configuration, tests, fixtures, ADRs, or documentation.
> - Never reference real colleagues, acquaintances, or personal workflow paths (e.g. personal email addresses, local home directory paths like `/home/<username>/Work/...`).
> - In all tests, fixtures, mocks, MCP tool schemas, and architectural documentation, strictly use generic open-source personas (e.g. *Ada Lovelace*, *Alan Turing*, *Alice Smith*, *Bob Jones*) and fictional domains (e.g. `acme.corp`, `example.com`).
> - Git commits must always be authored using each contributor's GitHub no-reply email and signed with a verified signing key.

---

## 3. Visual Architecture Overview

Daneel renders the second-brain graph of notes and infrastructure satellites using **Sigma.js** (WebGL-accelerated) and **Graphology**, organized within a galactic cosmic metaphor:
- **Galactic Core / Sectors (Nebulas)**: Conceptual notes grouped by thematic directory paths (e.g., `notes/arquitetura`, `notes/pessoas`).
- **Seldon Perimeter / Foundation Orbit**: Outer orbital ring with infrastructure satellites (ARMS, Harvesters, Routines, Skills, Preflight, etc.) placed at a fixed orbital radius ($r \approx 7000$ in graph space).
- **Cosmic Photons**: Quantum particles navigating graph edges with velocity compensated by camera zoom altitude.

---

## 4. Camera Invariants & Coordinate Contracts

### 4.1 Coordinate Spaces
There are three distinct coordinate spaces that must never be conflated:
1. **Graphology Space**: Raw node coordinates (`x`, `y`), scaled in thousands of units.
2. **Sigma Framed Graph Space**: Normalized `[0, 1]` coordinates calculated internally by Sigma.js (`sigma.viewportToFramedGraph` and `sigma.framedGraphToViewport`).
3. **Viewport Screen Space**: Physical DOM pixels `[0, width] x [0, height]`.

### 4.2 Semantics of `camera.ratio`
In Sigma.js, `camera.ratio` is **inversely proportional** to zoom magnification:
- `ratio: 1.0` $\rightarrow$ Panoramic view of the entire galaxy.
- `ratio: 0.20 - 0.40` $\rightarrow$ Framing of medium clusters and nebulae.
- `ratio: 0.02 - 0.08` $\rightarrow$ Close-up inspection and individual note reading.

> **Golden Rule**: **NEVER** artificially clamp `camera.ratio` with a rigid `Math.min(0.08, ...)` for clusters. That pushes 80% of stars out of the viewport. Always use the pure math function [`calculateClusterFraming`](web/src/lib/cameraMath.ts) with dynamic range `[0.020, 0.55]`.

### 4.3 Lateral Drawer Compensation
When the right-side detail drawer (440px width) is open:
- The user's visible center is NOT `width / 2`, but `openCenterX = (width - drawerWidth) / 2`.
- Any focused entity (satellite or note) must be shifted along the X-axis using:
  $$dx = (\text{width} / 2 - \text{openCenterX}) \times \frac{\text{camera.ratio}}{\min(\text{width}, \text{height})}$$
- In screen space, the target node must project to `(width - drawerWidth) / 2` with a maximum tolerance of 25 pixels.

### 4.4 Intact Anchor Principle
- **Satellites**: Satellites reside in the periphery ($r \approx 7000$). NEVER calculate a satellite's focus by averaging with neighbors in the core, as the barycenter lands in empty black space. Satellites are isolated anchors with 100% focus on their own coordinates (`ratio: 0.08`).
- **Notes**: For notes with neighbors, filter out neighbors whose normalized graph distance exceeds $> 0.18$ to prevent cross-galaxy edges from distorting local cluster framing.

---

## 5. Stellar WebGL Shader Guidelines

Location: [`web/src/lib/nodeStellarProgram.ts`](web/src/lib/nodeStellarProgram.ts)

### 5.1 Physical Pixel-Bounded Proportions
- **Anti-Blur**: Optical effects (glow, bloom, coronal gas) must **NEVER** scale proportionally to the radius without limits.
- Screen physical radius is provided by `v_radius`.
- Coronal gas thickness must be bounded with `clamp(v_radius * 0.18, 1.5, 3.5)` physical pixels.
- This ensures that under deep zoom, adjacent stars maintain crisp, distinct fusion cores without dissolving into a murky blur.

---

## 6. ForceAtlas2 Stabilization & Lifecycle

- ForceAtlas2 layout executes for ~8.5 seconds on initial galaxy load.
- **Premature Interaction Guard**: The UI displays an overlay (`data-testid="cosmic-loading-overlay"`) until layout completes (`isReady = true`).
- **Preserve User Navigation**: When layout finishes, the `animatedReset` callback must **NOT** trigger if `selectedNodeId` or `focusedSector` is active, avoiding cancelling user-initiated focal animations.

---

## 7. Safeguards & Testing Procedures

### 7.1 Unit Tests
Before any commit or build:
```bash
cd web && npm test
```
- [`CameraMath.test.ts`](web/tests/CameraMath.test.ts) validates ratio bounds, drawer offsets, and bounding box stability.
- [`StellarClassification.test.ts`](web/tests/StellarClassification.test.ts) validates astrophysical classifications.

### 7.2 E2E Invariant Tests
```bash
cd web && npx playwright test tests/e2e/camera-invariants.spec.ts
```
Verifies in a real headless browser:
1. Clicked satellite centered within 25px of `(width - 440) / 2`.
2. Cluster bounding box covering between 65% and 90% of viewport.
3. Descriptive English tooltips on all edge modes (`Auto`, `Bridges`, `Focus`, `All`).
4. Default enablement of cosmic photon simulator.

### 7.3 Mandatory WebFS Synchronization
The embedded Go binary serves `internal/webfs/dist`. After any modification in `web/src`:
```bash
task web-build
# Or verify drift integrity:
task web-verify-sync
```
Direct manual edits to `internal/webfs/dist` are forbidden; all assets must be strictly generated by the Vite build. Both CI and Lefthook reject commits with drift between `web/src` and `dist`.

## Automatic Release Contract

Squash merges into `main` trigger automatic semantic releases. Use Conventional Commit PR titles: `feat` increments minor, every other supported type increments patch, and `!` or a `BREAKING CHANGE:` / `BREAKING-CHANGE:` footer increments major. PR bodies become squash commit bodies. Do not manually edit version counters, create version-bump commits, move release tags or bypass the CI/native-archive gates. Source commits must remain signed. See [automatic releases](docs/releases/automation.md) and [ADR 0034](docs/adr/0034-automatic-semantic-releases.md).
