# Knowledge UX Implementation Plan

> Workers: use subagent-driven-development, implement only assigned files, report tests and changes. User authorized implementation and delegation after the exploratory UX review.

**Goal:** Make knowledge retrieval, reading, navigation and maintenance usable while preserving the galaxy metaphor.
**Architecture:** Keep existing React/Sigma/ConnectRPC boundaries. Add small UI components and pure helpers. Persistent navigation preferences stay local; vault capture uses a bounded local API. Keep lazy panels and generated WebFS.
**Tech stack:** React, TypeScript, Sigma, Vitest, Playwright, Go, ConnectRPC.
**Spec:** The accepted UX recommendations in this conversation, operationalized below.

## Global constraints
- English UI and comments; neutral fixtures and docs. No personal paths or corporate names.
- Preserve camera contracts and default 440px drawer; no manual dist edits.
- Tests before behavior changes; focused tests during iteration, full checks at integration.
- Workers do not commit or regenerate dist; controller owns integration and release checks.
- No real vault mutations during verification; use temporary test vaults.

## Tasks
- [x] 1. Search: accept short terms, handle pending/error/empty and stale requests, deduplicate by normalized path, classify satellites correctly, group results, safely render highlighted snippets, show paths, remove silent eight-result truncation. Files: useNoteSearch.ts, CommandSearchDialog.tsx, types/galaxy.ts (optional status fields only), corresponding tests. Keep hook/component backwards-compatible until integration.
- [x] 2. Reading: parse frontmatter into collapsed metadata, GFM tables, avoid duplicate first title, font controls and local table of contents, explain incoming/outgoing links when supplied. Files: FloatingNoteCard.tsx, new note document helpers, package dependencies, corresponding tests. Optional props for relations; do not change App.tsx. Preserve 440px compact width until shared camera support.
- [x] 3. Immediate ergonomic fixes: truthful top statistics, platform-aware search shortcut, larger toolbar controls, simpler cockpit explanations and human schedules/last/next run, navigation to previous/next week and date picker in temporal bar. Files: TopCosmicBar.tsx, BottomTemporalHotbar.tsx, CockpitModal.tsx, dedicated helpers/tests. Do not change App.tsx or public required props.
- [x] 4. Navigation and exploration: history restoring selection/camera/filter/scroll, deep links, recent notes and saved searches, searchable list synchronized with selected sector/constellation, optional dock controls; integrate search states, relations and reader props. Files: App.tsx, new navigation/explorer hooks/components, tests; ADR 0029.
- [x] 5. View controls and responsive reader: unified View with Explore/Read/Present presets, reduced-motion support, preserved label hierarchy, resizable drawer with shared width used by camera math. Preserve original 440px default and existing focus tests. Files: view components, camera controller/context, note card optional width callbacks, tests; ADR 0029.
- [x] 6. Capture: title/content/source form; explicit save to inbox via local bounded API; no overwrite, traversal or symlink escape; successful save returns path and refreshes graph. New RPC and generated code, backend tests, UI component; ADR 0030.
- [x] 7. Review: local graph-based orphan review and candidate connections with transparent reasons, accept as saved review decision or ignore, no fabricated semantic intelligence; show read-only broken-link diagnostics if available from existing services. Persist review decisions locally; explain that accepted suggestions do not edit notes. Integrate in explorer; ADR 0029.
- [x] 8. Integration and verification: task reviews, fix findings, lint/typecheck/unit tests, Go tests for backend changes, production build, isolated browser checks, WebFS generation, final branch review.

## Review focus
- Rapid search/navigation must never replace current content with stale responses.
- Frontmatter, malicious snippets and malformed localStorage must not cause unsafe rendering or crashes.
- Browser history, deep links and drawer resize preserve camera behavior.
- Dates cross year/month and DST boundaries without selecting the wrong day.
- Capturing a note never overwrites or escapes the inbox even through symlinks.

## Verification commands
`cd web && npm test && npm run lint && npm run typecheck`
`task web-build`
`cd web && DANEEL_E2E_PREVIEW=1 npx playwright test`
`cd web && DANEEL_E2E_EMBEDDED=1 npx playwright test` (while development uses port 5173)
`go test ./cmd/...` (and full Go checks if backend interfaces change).

## Final validation — 2026-10-16
All implementation tasks are complete in the feature worktree. Independent reviews and isolated browser checks found and verified corrections for Markdown headings, search snippets, midnight rollover, capture indexing warnings, camera normalization after graph reload, reduced-motion camera state, browser history, mobile reader bounds, and structural-review evidence.

- Web: 47 files / 288 unit tests passed; full TypeScript and ESLint passed.
- Backend: full Go suite passed; strict Go lint reported zero issues; capture regressions passed; vulnerability scan reported no affected code.
- Production: all 7 Playwright tests passed against a temporary neutral vault. Coverage includes camera invariants, lazy chunks, capture/save/search, note history and deep links, drawer resizing, a 390px viewport, local review decisions, and console guardrails.
- Functional document workflows use the operating-system reduced-motion preference to avoid decorative software-GPU load. Camera invariants exercise the default animation mode. The history test also verifies finite camera snapshots and distinct entries for successive notes.
- Desktop and narrow reader/overview screenshots were visually inspected.
- `task --force web-build` and `task --force web-verify-sync` passed. Reproducibility used a temporary Git index containing the freshly generated assets; the worktree index was left untouched.

## Scope and delivery
View controls preserve existing label LOD rules; no new label hierarchy was introduced. Structural review records local decisions only and describes missing targets as unindexed, because the graph cannot prove a link is broken. The frontmatter parser uses bounded YAML parsing; the earlier gray-matter dependency was removed.

The feature worktree is preserved on `feat/knowledge-ux`. The existing application process was not restarted, and verification did not write to a real vault.

## Completion revalidation — 2026-10-16

The complete current worktree was revalidated, including the later date-picker,
navigation-menu and search-modal feedback. The capture dialog now has bounded
height and internal scrolling on short screens. Search delegates initial focus to
Base UI and preserves its dialog lifecycle so Escape restores the opener's focus.

- Web: 48 files / 301 unit tests passed without warnings or errors; ESLint and application/test TypeScript checks passed.
- Browser: the full 24-test suite passed against the embedded production WebFS and a temporary neutral vault. A subsequent improvement made the zoom test select a real connected note instead of a synthetic sector; its focused rerun passed with nine constellation nodes. Capture, dates, history, resizing, review, keyboard focus and console guardrails passed.
- Legacy visual tests now exercise the current unified View controls and spectrum help. Neutral fixtures include long reading content and repositories, so scrolling, sector and repository checks cannot pass by skipping absent data. Functional workflows use reduced motion; camera and photon-default invariants still exercise the default animation mode.
- Backend: full Go tests, strict lint, vulnerability checks, formatting of 164 Go files, installation smoke, search benchmark and GoReleaser configuration validation passed. The vulnerability scan found zero affected symbols.
- Build and forced WebFS reproducibility checks passed using a temporary Git index. Workflow, Taskfile and hook YAML parsed successfully; whitespace checks passed.
- Desktop, narrow-reader and short-screen capture screenshots were inspected. CI now runs all browser tests. The development server remains available on port 5173; the embedded E2E mode uses port 8088 without touching the real vault. Browser MCP artifacts are ignored by Git.

All planned adjustments and final review findings are resolved on
`feat/knowledge-ux`; no push or service restart was performed.
