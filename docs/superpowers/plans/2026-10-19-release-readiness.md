# Open-source 1.0 Release Readiness Implementation Plan

> **For agentic workers:** Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Resolve the audited release blockers and validate a signed release candidate before stable publication.

**Initial publication policy:** Consolidate the complete reviewed tree into one signed root commit before the first code push. Validate it on a `release/**` branch before creating `main`. Candidate and stable tags share that commit when gates pass; record hosted evidence in GitHub Releases and Actions rather than additional source commits.

**Architecture:** Retain the existing local-first application and the reusable CI/release contract. Update vulnerable development dependencies, enforce npm auditing, and validate the actual distributed artifacts. Publish only the sanitized history to the recreated repository.

**Tech Stack:** Go, React, TypeScript, Vite, Vitest, Tailwind CSS, Playwright, GitHub Actions, GoReleaser, Syft.

**Spec:** `docs/adr/0027-stable-release-contract-and-validation.md` and the approved five-item release-readiness audit.

## Global Constraints

- English project text and neutral fixtures; no private vault contents or credentials.
- Signed commits/tags and GitHub no-reply authorship.
- Generate WebFS assets exclusively through the frontend build.
- Keep the cosmic HUD, keyboard behavior and note-reading workflows intact.
- Reuse shared CI for candidate and stable tags; never replace a published tag.
- Require actual hosted OS and provenance results before claiming release validation.

## Review Focus

- Dependency major upgrades must preserve rendering, responsive controls and keyboard focus.
- Full development dependency auditing must fail CI for known advisories without silent suppression.
- Clean-install checks must use temporary configuration and vaults.
- Generated assets must reproduce from the committed lockfile.
- Repository publication must retain only the sanitized history and expose a private security reporting channel.

### Task 1: Secure the frontend toolchain

**Files:** `web/package.json`, `web/package-lock.json`, frontend build/style configuration, `.github/workflows/ci.yml`, `lefthook.yml`, `docs/adr/0027-stable-release-contract-and-validation.md`.

**Interfaces:** Consumes the current lockfile and existing UI tests. Produces an audited toolchain and generated WebFS assets for distribution.

- [x] Reproduce the dependency audit failures from a clean installation.
- [x] Select patched versions and migrate only configuration/style compatibility required by those upgrades.
- [x] Add `npm run audit` and enforce it in CI and pre-push checks.
- [x] Pass full frontend unit tests, lint, types, production build, asset reproducibility and isolated production browser checks.

### Task 2: Validate artifacts and document the candidate

**Files:** `CHANGELOG.md`, `docs/releases/checklist.md`, `docs/releases/verification.md`, `docs/development/quality-and-testing.md`, release tooling configuration when required.

**Interfaces:** Consumes Task 1's verified source/assets. Produces current release notes, snapshot archives, SBOMs, checksums and installation evidence.

- [x] Run Go tests, lint, vulnerability scanning and workflow/configuration validation.
- [x] Generate six snapshot archives and SBOMs using pinned GoReleaser/Syft; verify checksums and smoke-test the native archive.
- [x] Exercise an upgrade against a backed-up neutral vault.
- [x] Update release notes and record exact current results and external limits.
- [x] Review and create one verified signed root commit for the initial public history.

### Task 3: Publish and protect the sanitized repository

**Files:** Hosted repository settings and workflow evidence; update the checklist with observed results.

**Interfaces:** Consumes the reviewed signed commit and clean local history. Produces a public repository with working security/reporting controls and hosted CI results.

- [x] Verify the recreated repository and absence of old branches, tags and PR refs.
- [ ] Validate the signed root on a release branch, create main at that same commit, and configure public visibility, private vulnerability reporting, dependency alerts, branch protection and release-tag rules.
- [ ] Observe native Linux/macOS/Windows checks and resolve actual failures.

### Task 4: Validate candidate and stable publication

**Files:** Signed release tags, published release artifacts and final verification evidence.

**Interfaces:** Consumes green hosted CI and configured release controls. Produces a validated candidate, then stable release only when candidate gates pass.

- [ ] Create a signed `v1.0.0-rc.1` tag on the reviewed commit and observe the release workflow.
- [ ] Verify downloaded checksums, SBOMs, attestations and native installation/upgrade behavior.
- [ ] Record candidate evidence in GitHub Releases and publish signed `v1.0.0` on the same root commit only after the gates pass.
- [ ] Independently review the full change and report any remaining external blockers precisely.
