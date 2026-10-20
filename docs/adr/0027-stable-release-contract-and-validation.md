# ADR 0027: Stable Release Contract and Validation

* Status: accepted
* Date: 2026-10-14

## Context and Problem Statement

A first stable release needs explicit compatibility guarantees and validation of the tagged commit. PR checks alone do not prove a tag or distributed binary was validated.

## Decision Drivers

- Publish only after quality gates pass.
- Test a clean installation without private environment dependencies.
- Distinguish stable interfaces from evolving automation.
- Make distribution dependencies and provenance inspectable.

## Considered Options

- Publish tags directly and rely on previous branch CI.
- Duplicate checks in independent PR and release workflows.
- Reuse CI as a prerequisite for release and publish provenance evidence.

## Decision Outcome

Reuse CI as a prerequisite for release and publish provenance evidence. The release workflow calls the same CI workflow on the tagged commit, including OS tests, clean installation smoke checks and isolated browser invariants. Pin Go, Node, lint, vulnerability-scanner, GoReleaser and SBOM generator versions. Produce archive SBOMs and GitHub build-provenance attestations. Stage artifacts as a draft, smoke-test the native archive and attest its provenance before promotion. Mark candidate tags as prereleases. Define 1.x compatibility in docs/releases/compatibility.md.

### Consequences

- GitHub settings, private reporting and tag rules require separate maintainer configuration.
- Attestation issuance can only be verified in GitHub Actions.
- The browser fixture is temporary and does not depend on a developer vault or globally installed Chrome.
- Experimental agent/telemetry/RPC internals retain room to evolve without silently weakening user-data protection.

### Native distribution gate — 2026-10-20

After staging and attesting the draft, run an independent Linux/macOS/Windows
matrix against the uploaded archive matching each runner's actual architecture.
Each job downloads the archive and checksum manifest, rejects missing, duplicate
or mismatched checksum entries, then runs the clean-installation smoke against
the extracted binary. Promote the draft only after every native job succeeds.
These jobs require repository write access to read the authenticated draft
([GitHub restricts draft listings to callers with push access](https://docs.github.com/en/rest/releases/releases?apiVersion=latest#list-releases)); they
run only from administrator-created release tags. Failures retain the draft for
inspection. Native source tests alone do not satisfy this distribution gate.

### Validation runner maintenance — 2026-10-20

Pin Task 3.54.0 and clear Git's repository-local environment variables before
running Go tests or benchmarks from Task. Git hooks export their repository
context; fixture commands must initialize their own temporary repositories rather
than changing the caller's index, configuration or history. Exercise this boundary
with a disposable parent repository on each native CI platform.

### Initial public history — 2026-10-20

Publish 1.0 from one reviewed, signed root commit. Run shared CI on a `release/**`
branch before creating `main` at that commit. If validation requires source
corrections, prepare and validate a replacement candidate before creating stable
tags; do not rewrite a published release tag. Candidate and stable tags may point
to the same source commit. Record hosted validation and publication evidence in
GitHub release notes and workflow runs so documenting their results does not add
commits to the initial stable source history.
