# Release checklist

Latest local results: [verification report](verification.md).

## Required local evidence

- [ ] `go test ./...` and `golangci-lint run ./...` pass.
- [ ] `govulncheck ./...` reports no affected reachable symbols; review module-only findings separately.
- [ ] `cd web && npm ci && npm test && npm run lint` succeeds.
- [ ] `cd web && npm run audit` reports zero known advisories across production and development dependencies.
- [ ] `cd web && npm run typecheck` succeeds.
- [ ] `task --force web-build` then `cd web && DANEEL_E2E_EMBEDDED=1 npx playwright test` passes the complete suite against the isolated fixture and generated assets, including the bundled interface font.
- [ ] `task web-verify-sync` reports no embedded-asset drift.
- [ ] `goreleaser check` succeeds with GoReleaser 2.17.1.
- [ ] With Syft 1.54.1 installed, `goreleaser release --snapshot --clean --skip=publish` produces all archives, SBOMs and checksums without publishing.
- [ ] Extract the matching snapshot archive and run `python scripts/smoke.py --binary <extracted-binary>`.
- [ ] Review the compatibility policy, recovery guide and release notes.

## One-time repository setup

Apply [GitHub setup](github-setup.md) after creating or recreating the remote.

- [ ] Create reviewed, signed source on `main` and make it the default branch.
- [ ] Apply `.github/repository-settings.json`: squash-only merges, PR title/body as commit message.
- [ ] Apply main protection and require all CI checks plus `Conventional PR title`.
- [ ] Remove legacy tag-creation restrictions; prohibit tag updates/deletion with no bypass.
- [ ] Apply the workflow-specific event policy and confirm Actions policies allow the required events, job permissions and actions.
- [ ] Verify native Linux, macOS and Windows checks; cross-compilation alone is insufficient.
- [ ] Enable security reporting, dependency alerts and secret scanning before public publication.

## Automatic releases

- [ ] Run the initial workflow dry run and inspect the planned initial `1.0.0`.
- [ ] For the first real run, verify the tag identifies the intended signed source.
- [ ] Verify all six archives, SBOMs and checksums are staged in a draft.
- [ ] Verify native draft-archive download/checksum/installation jobs on Linux, macOS and Windows.
- [ ] Verify public provenance attestations; private test runs explicitly skip this plan-dependent feature.
- [ ] Confirm publication occurs only after all required gates succeed.
- [ ] Exercise a repeat run: the version stays fixed and a published release is not rebuilt.
- [ ] Verify a candidate installation and an upgrade of a backed-up neutral vault before public announcement.

Every subsequent semantic squash merge into `main` chooses its own version and
runs the pipeline automatically. Features increment minor; all other supported
maintenance types increment patch; breaking markers increment major. Do not edit
version counters, manually push release tags or move an existing tag. See
[automatic releases](automation.md) and [ADR 0034](../adr/0034-automatic-semantic-releases.md).

Local tests and branch CI do not establish real tag-write permissions, draft
publication or artifact attestation issuance. Inspect those results on a real
release run. Repository deletion or recreation is never a release workflow step.
