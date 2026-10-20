# ADR 0028: Deferred Web Panels and Typed Boundaries

* Status: accepted
* Date: 2026-10-14

## Context and Problem Statement

The initial Web entry bundled search, the operational cockpit and Markdown
readers even before a user opened them. The entry was 610,411 bytes and its
initial JavaScript dependencies totaled 850,967 bytes. Explicit `any` types in
rendering boundaries, mocked RPC responses and browser inspection obscured
contracts, while lint accepted warnings and test files were not type-checked.

## Decision Drivers

- Reduce initial JavaScript without changing the graph, camera or shader contracts.
- Preserve panel state and shared routine execution after closing a panel.
- Make delayed loading and failed downloads understandable without hiding the galaxy.
- Validate application, test and generated-build boundaries with meaningful checks.

## Considered Options

- Split vendor files while keeping every panel eagerly loaded.
- Defer the whole galaxy, delaying the primary interface.
- Load secondary panels on demand and retain each after first use.

## Decision Outcome

Load search, shortcuts, cockpit and note inspectors with React lazy imports.
`DeferredPanel` defers the first mount, retains modal state after closing, and
provides local loading and error feedback. Routine execution remains owned by
App so panel loading cannot reset active operations. Notes retain their existing
selection lifecycle. No camera or shader behavior changes.

Assign React, graph and protocol vendor modules explicitly. Keep Markdown out of
the initial dependency graph; shared JSX/CommonJS runtime modules belong to the
React chunk. Do not raise chunk warning thresholds to conceal large output.

Replace broad type assertions with domain types, protobuf constructors and typed
browser inspection. Run TypeScript on both application and tests, and enforce
zero lint warnings. Ignore generated coverage output alongside existing build
artifacts, while continuing to lint source and tests.

### Consequences

- First use of a secondary panel may require another download; subsequent opens retain its state.
- A failed panel download leaves the galaxy accessible and offers a page reload.
- CI browser checks use a production preview and verify actual deferred requests, cockpit state and note rendering in addition to camera and console invariants.
- Embedded WebFS files must be regenerated whenever source changes.
