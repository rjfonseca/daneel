# Release hardening implementation plan

> Execution: inline, following the user's authorization to proceed. Use superpowers:executing-plans and test-first regressions for behavioral changes.

**Goal:** Prepare a reviewable and locally verified 1.0 candidate without publishing it.
**Spec:** ../specs/2026-10-14-release-hardening-design.md
**Architecture:** Harden existing HTTP boundaries and persistence paths; reuse CI for release gates. Keep local interfaces and generated assets intact.
**Tech Stack:** Go 1.27, React/Vite/Vitest, Playwright, GitHub Actions, GoReleaser.

## Global constraints
English text; neutral fixtures; MADR records; no publication; preserve unrelated files.

## Tasks
- [x] HTTP safety: add failing GetNote traversal/symlink and HTTP Origin/Host tests in cmd/serve_security_test.go; run them; implement os.Root reads and loopback request middleware in cmd/serve_security.go; rerun cmd tests.
- [x] Persistence: add failing backup/atomic-write and dry-run regressions; implement internal/safefile writer and integrate all internal/dream writes, propagating errors; verify dream and safefile suites.
- [x] Pruning: add failing recovery and external-symlink tests in internal/harvester/prune_test.go; replace deletion with vault-local quarantine moves; propagate scan errors; verify harvester suite.
- [x] Release gates: make CI reusable through workflow_call; require it before GoReleaser; pin tools, add SBOM/attestation and per-platform install smoke checks. Validate YAML and GoReleaser config.
- [x] Portable onboarding: add neutral temporary test vault, portable Playwright browser settings and fixture setup; run essential browser invariants and installed-binary smoke checks with isolated XDG paths.
- [x] Contracts and docs: record MADRs, stable/experimental API policy, recovery instructions, RC checklist and corrected README/security/development requirements.
- [x] Final verification: full Go and frontend suites, lint/security checks, browser invariants, WebFS drift and release snapshot; review diff and report exact limits.

## Review focus
- Requests with missing or foreign Origin/Host never bypass localhost boundaries.
- Traversal and escaping symlinks cannot read outside the vault.
- Failed replacements preserve the original and recovery copy; first writes do not invent backups.
- Dry-run has no vault writes, including derived dashboard/people/report outputs.
- Pruning a symlink cannot move or modify an external directory.

## Execution record
- Clean main baseline; working on release/1.0-hardening in the current checkout. No worktree created; no unrelated changes present.
- Ruling: proceed with the authorized scope without another approval handoff; no remote publishing is included.

- Initial integration decision: retain the working branch while the remote is recreated. The user subsequently authorized signed local commits and a merge into local main. No remote push, release tag or publication is included.
- Ruling: remove implicit catalog reconciliation from pruning. Its independent mutations could delete retained notes or follow symlinks; pruning now only plans and quarantines selected paths.
- Review: independent reviewer identified cache precedence, umask preservation and implicit catalog mutations. Added regressions, corrected all three, and received a second review without Critical/Important findings.
- Verification: full Go suite, 203 frontend tests, 2 essential browser tests, Go lint, frontend lint (warnings only), actionlint, clean frontend build, WebFS equality, GoReleaser config/snapshot, 6 archive SBOM/checksum checks and extracted Linux archive smoke all succeeded. Details and limitations: docs/releases/verification.md.
