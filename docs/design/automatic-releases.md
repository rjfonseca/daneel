# Automatic releases from merged pull requests

## Intent

Every squash merge into `main` automatically determines a semantic version and
publishes a GitHub release after the existing quality and native archive gates.
Contributors do not edit version files, create release pull requests or push tags.
The initial release is `1.0.0`. Subsequent merges each receive a distinct version.

## Version contract

The pull request title becomes the squash commit title; its body becomes the
commit body. A trusted-base workflow validates the title before merge. Supported
types are `feat`, `fix`, `chore`, `docs`, `refactor`, `perf`, `test`, `build`, `ci`,
`style` and `revert`. A scope is optional. A nonempty description is required.

| Change | Increment |
| --- | --- |
| `!` after type/scope, or a `BREAKING CHANGE:` / `BREAKING-CHANGE:` footer | Major |
| `feat` | Minor |
| Every other supported type | Patch |

Maintenance merges intentionally produce patch releases. This extends the
Conventional Commits specification, which does not prescribe a version increment
for these types. An invalid commit stops release planning instead of silently
discarding a merge.

`.github/release-policy.json` records the initial version. Its first appearance
on the first-parent history anchors versioning. That commit receives the initial
version; each subsequent first-parent commit applies its own increment. The
initial version cannot change later. Planning needs full Git history and rejects
merge commits after the anchor: the repository must allow squash merges only.

This gives a commit the same version even after a retry, a failed draft build or
out-of-order workflow scheduling. Tags never determine the next number and must
never be moved. A recreated repository with a new root starts a new release
history; keeping the Git history preserves the same version assignments.

## Pipeline

1. A merged-PR event targeting `main` dispatches `release.yml` on `main` with
   `dry_run=false` and `source_ref` pinned to its squash commit. Closed but
   unmerged PRs do nothing. This event is not suppressed by commit-message skip
   directives. Only the dispatch job has `actions: write`; it uses trusted pinned
   workflow code and returns without waiting for the new run. Only the resulting
   `workflow_dispatch` event enters release planning and publication, as required
   by the provenance build type. A manual dry run may plan and
   validate a selected commit without writing tags or releases.
2. Compute the source SHA, version, tag and release notes from the pinned commit.
3. Run the reusable CI against that exact SHA: Go, all npm dependencies, types,
   lint, source installation, browser and release-script regression checks.
4. Require the real release target to be on `main`'s first-parent history and
   verified as signed by GitHub. Create an immutable tag with `GITHUB_TOKEN`.
   A matching tag is reusable; a tag pointing elsewhere is an error. An already
   published release is a successful no-op.
5. Build all six archives with GoReleaser, SBOMs and checksums into a draft.
   Retries reuse an unpublished draft and replace its incomplete artifacts.
6. Public repositories require build provenance attestations. Private test
   repositories explicitly skip this unsupported-on-many-plans feature; all
   other gates remain mandatory. No public attestation failure is ignored.
7. Download the matching draft archive on Linux, macOS and Windows, check SHA-256
   and exercise the installed binary.
8. Publish only after all native jobs succeed. An older queued release must not
   replace a higher semantic version as the repository's latest release.

Use one concurrency group with `cancel-in-progress: false` and `queue: max`.
GitHub supports up to 100 waiting runs; runs beyond its queue limit require
manual retry. The version assignment does not depend on execution order.

The workflow performs tag creation and packaging directly. It does not rely on a
second tag-push event: `GITHUB_TOKEN` pushes do not trigger another workflow.
The bot creates no source commits and therefore cannot cause a version-bump loop.
Runtime versions continue to come from GoReleaser linker flags, not the private
frontend package's version field.

## Repository setup

Create `main` at the reviewed commit and make it the default branch. Enable
squash merges only, with PR title and body as the squash message. Require signed
commits, pull requests, linear history and all CI/title checks on `main`.
Retain an immutable-tag ruleset with no bypass actors. Remove legacy tag-creation
restrictions: the built-in Actions integration cannot reliably bypass those
rulesets. Tag creation uses the release job's narrowly scoped contents-write permission. Grant write permissions
only to the release jobs that need them; use the built-in token, with no PAT or
signing-key secret. Tags identify signed source commits; the tag objects themselves
are not separately signed. This replaces the old manual signed-tag process.

Document the settings and checked-in rulesets so deleting and recreating the
remote repository does not require rediscovering them. Do not delete or recreate
a repository as part of implementation or verification.

## Verification

Use disposable Git repositories to test initial versions, all commit types,
breaking footers, scope parsing, maintenance-only merges, nonlinear history,
policy edits, retries and scheduling independent of tag creation. Test GitHub
operations through a fake API: unsigned sources, wrong tags, existing drafts,
published-release no-ops and latest-version preservation must all be covered.
Validate the workflows with a pinned actionlint supporting queued concurrency.
Retain source installation, native archive checksum and full quality gates.
Local tests and a branch CI do not prove real token permissions or first release
publication; report that boundary explicitly until a merge exercises it.
