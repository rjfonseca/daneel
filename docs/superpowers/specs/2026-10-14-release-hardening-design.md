# Daneel 1.0 release hardening

## Intent
Prepare the existing local-first application for its first stable release. Preserve the CLI, MCP and visualizer workflows while making vault boundaries, recovery and publishing explicit and testable. The user authorized this scope after reviewing the release-readiness findings.

## Decisions
- Keep HTTP strictly on IPv4 loopback. Reject foreign Origin and Host headers, including requests without Origin using an unexpected Host. Permit the local Vite development origin explicitly. Remote access is unsupported; do not add an unauthenticated network mode.
- Read requested notes through Go's traversal-resistant `os.Root`. Accept local relative paths only and prevent symlink escapes. Preserve ARMS virtual note handling.
- Use a shared atomic file writer for Dream output. Preserve existing file permissions, stage in the destination directory, sync and close before replacement, and retain an immutable backup of replaced content. Fail visibly on persistence errors. Honor dry-run throughout the cycle.
- Quarantine pruned source directories inside the vault instead of recursively deleting them. Plan all moves before mutation; a dry-run does not create directories. Report failures and document restoration. Never follow source-directory symlinks outside the vault.
- Reuse the same required quality workflow for PRs and tags. Include Go platform tests, lint, vulnerability checks, frontend tests, embedded-asset drift, portable fixture-backed browser invariants, and installation smoke tests.
- Pin release tools, produce SBOMs and GitHub artifact attestations, publish prerelease tags as prereleases, stage artifacts as a draft until native archive smoke and provenance issuance succeed, and validate release configuration locally. Do not publish a tag as part of this work.
- Define stable CLI/config/MCP contracts and mark evolving agent/visual internals experimental. Correct installation, version requirements, retrieval terminology and security support documentation.

## Verification
Use regression tests for traversal, CORS/Host enforcement, failed writes, backups, dry-run and quarantine recovery. Run full Go and frontend suites, frontend lint, browser invariants, static analysis, vulnerability checks, clean WebFS build and GoReleaser validation/snapshot when tools are available. Report environment or external validation limits separately.

## Constraints
English project text and fictional fixtures only. Preserve user data and unrelated changes. Record architectural decisions in MADR. No direct edits of generated WebFS assets. No remote publication or activation of user services.
