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

## Recreated GitHub repository

Apply and verify the [GitHub setup](github-setup.md), including the checked-in
branch and tag rulesets.

- [ ] Enable private vulnerability reporting, dependency alerts and branch protection requiring CI.
- [ ] Configure tag rulesets to restrict creation and updates of `v*` release tags.
- [ ] Confirm the repository supports GitHub artifact attestations and workflow token permissions.
- [ ] Verify actual Linux, macOS and Windows jobs; local cross-compilation alone is insufficient.
- [ ] Review the first candidate's installed binaries on supported systems and verify artifact provenance.

## Candidate and stable release

Create a signed `v1.0.0-rc.1` tag only after reviewing the candidate commit. The
release workflow reruns reusable CI on the tagged commit before publication.
GoReleaser marks prerelease tags as prereleases. Artifacts are staged as a draft, the native archive is smoke-tested, and GitHub
attests archive, SBOM and checksum provenance before the draft is published.
Failures leave a draft for maintainers to inspect. See
[GoReleaser SBOMs](https://goreleaser.com/customization/sbom/) and
[GitHub's provenance action](https://github.com/actions/attest-build-provenance).

Verify a candidate installation and an upgrade of a backed-up vault before
creating the signed `v1.0.0` tag. Never move or replace a published release tag.
Creating or pushing release tags and activating repository settings are separate
maintainer actions; implementing the workflow does not perform them.

For the first public release, consolidate the reviewed source into one signed
root commit. Validate it on a `release/**` branch before creating `main`, then
place the candidate and stable tags on that same commit if all gates pass. CI
runs on both release branches and `main`. Never rewrite a published release tag.

The changelog describes the intended 1.0 content. Record publication status and
hosted verification in the GitHub release notes and linked workflow runs. This
keeps the initial public source history at one commit without adding documentation
commits between candidate and stable validation.
