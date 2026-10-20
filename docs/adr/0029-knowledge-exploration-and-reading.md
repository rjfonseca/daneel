# ADR 0029: Knowledge Exploration and Reading

* Status: accepted
* Date: 2026-10-14

## Context and Problem Statement

The galaxy emphasizes spatial exploration, but retrieval, reading and repeat visits need direct controls. Search mixes operational nodes with memory notes, while narrow readers and visual overlays compete with content.

## Decision Drivers

- Preserve the cosmic identity, lazy loading and existing camera invariants.
- Make short technical queries, readable Markdown and keyboard navigation reliable.
- Keep navigation and review state local without adding a remote account subsystem.
- Avoid implying semantic evidence that the graph does not contain.

## Considered Options

- Replace the galaxy with a document browser.
- Add an independent document application.
- Add complementary exploration and reading controls using existing graph data.

## Decision Outcome

Add complementary controls. Search distinguishes memory notes, clusters and automations, deduplicates normalized paths and renders match highlights as React text. The reader separates metadata, supports GFM, a table of contents and adjustable typography. Drawer width is a single shared value, defaulting to 440 CSS pixels, used by rendering and camera compensation; satellite anchoring and dynamic cluster framing remain unchanged.

Navigation snapshots retain the selected note, category cluster or sector, camera state, view filters and reading offset. URL fragments identify each target separately (`#note=`, `#cluster=` and `#sector=`) for local deep links; existing cluster fragments continue to address category clusters. A bounded, validated local browser store holds recent notes, saved searches and review decisions; stored values are untrusted on reload. Graph lists are scoped to the active sector or constellation and use the same selection action as search.

View presets coordinate existing overlay toggles and edge modes. Reduced motion disables decorative activity without hiding navigation. Existing pinning remains available. Daily navigation uses calendar arithmetic in local time.

Graph review identifies disconnected notes and suggests connections only from explicit structural evidence such as shared neighbors. Every candidate displays its reason. Accept and ignore record a local review decision; they do not write links to notes. Missing graph endpoints are described as unindexed targets, not conclusively broken files. Link creation and semantic inference remain outside this structural review contract.

### Consequences

- Local history and preferences do not synchronize between devices.
- Review acceptance is a saved decision, not a vault mutation; the UI must state this.
- Changes to drawer width require camera regression coverage.
- The graph remains the source of structural relationships; source excerpts require reading the relevant note.
