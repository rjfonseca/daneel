# Automatic Releases Implementation Plan

**Goal:** Generate one semantic version and gated release per squash merge to main.

**Architecture:** Deterministic first-parent version planning, a small GitHub API
adapter and the existing GoReleaser/native verification pipeline. The built-in
workflow token creates tags; source commits remain signed.

**Tech stack:** Python standard library, Git, GitHub Actions, GoReleaser.

**Spec:** [Automatic releases](automatic-releases.md), ADR 0034.

## Constraints and review focus

- Bootstrap at `1.0.0`; maintenance types produce patch releases.
- No version commits, extra credentials, history rewrites or moved tags.
- Public provenance is required; private test limitations are explicit.
- Pin source SHA in every checkout and retain all native/quality gates.
- Test retries, reordered runs, malformed commits, unsigned source and conflicts.

## Tasks

- [x] Write failing disposable-repository tests for version planning, immutable
  policy, breaking changes, squash history and retry invariance; implement the
  planner and CLI until those tests pass.
- [x] Write failing fake-API tests for verified source, tag conflicts, existing
  drafts, published no-ops and latest-version selection; implement the API adapter.
- [x] Wire trusted PR-title checks, pinned reusable CI, queued automatic releases,
  dry runs and private/public provenance handling. Validate with current pinned
  actionlint and GoReleaser; retain checksum and installation regression checks.
- [x] Update contribution/ADR/release documentation and repository bootstrap
  configuration. Run Python, Go and frontend gates plus snapshot/native Linux
  smoke. Review independently.
- [ ] Validate the committed branch in hosted CI. Do not merge or publish a
  release as part of implementation.
