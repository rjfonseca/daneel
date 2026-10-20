# ADR 0004: Astrophysical Taxonomy & Dynamic LOD Edge Filtering

* Status: accepted
* Date: 2026-09-09

## Context and Problem Statement

Large knowledge graphs with thousands of notes and connections can suffer from two classic graph layout pathologies in the WebGL visualizer:
1. **Luminance Collapse ("White Blob")**: Densely connected hubs and young notes competed at identical brightness levels, blinding the user and obscuring local cluster structures.
2. **"Hairball" Edge Overload**: In panoramic overview zoom, thousands of internal cluster micro-edges created blinding visual noise, drowning out the high-level semantic bridges connecting disparate domains.

How should the visualizer scale to tens of thousands of nodes while maintaining aesthetic clarity and semantic hierarchy?

## Decision Drivers

* Instant visual differentiation between active super-hubs, mature conceptual pillars, and peripheral leaf notes.
* Elimination of visual clutter at high zoom altitudes without losing local detail at low zoom altitudes.
* Maintain 60 FPS WebGL rendering performance.

## Considered Options

* **Option 1: Uniform Node Coloring with Standard ForceAtlas2**
* **Option 2: Astrophysical Taxonomy (Wien's Law + Gravitational Mass) & Dynamic LOD Edge Filtering**
* **Option 3: Manual tagging and hardcoded category colors**

## Decision Outcome

Chosen option: **Option 2: Astrophysical Taxonomy & Dynamic LOD Edge Filtering**, because:
- Maps physical thermodynamics ($\lambda_{\text{peak}} \propto 1/T$) to note recency (`mtime`) and gravitational mass to connectivity degree (`connections`).
- Establishes an 8-class stellar taxonomy (Quasar, Pulsar, Blue Giant, White Star, Yellow Dwarf, Orange Star, Red Dwarf, Brown Dwarf).
- Dynamically hides intra-cluster micro-edges in panoramic overview while preserving inter-cluster gravity bridges, rendering full detail only when zooming deep into a sector.

### Positive Consequences

* Less connected, older notes are categorized as Brown Dwarfs (`size: 0.40`, muted bronze), converting former white blobs into clean, organic starfields.
* Inter-cluster bridges become immediately apparent at macroscopic scale.
* Deep zoom maintains needle-sharp fusion core definition via pixel-clamped coronal shaders.

### Negative Consequences

* Requires custom WebGL shader implementation (`nodeStellarProgram.ts`) and zoom-aware edge reducers.
