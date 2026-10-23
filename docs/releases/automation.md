# Automatic releases

Every squash merge into `main` runs CI and publishes a release automatically.
Use a Conventional Commit PR title; GitHub copies the PR title and body into the
squash commit. No manual version edit, release PR, tag push or extra secret is
required. Repository administrators perform the [one-time setup](github-setup.md).

## Version increments

| PR title/body | Increment | Example from `1.2.3` |
| --- | --- | --- |
| `feat(search): add filters` | Minor | `1.3.0` |
| `fix: repair indexing` | Patch | `1.2.4` |
| `chore`, `docs`, `refactor`, `perf`, `test`, `build`, `ci`, `style`, `revert` | Patch | `1.2.4` |
| `feat!: replace a stable contract` or `fix(cli)!: remove a flag` | Major | `2.0.0` |
| A `BREAKING CHANGE:` or `BREAKING-CHANGE:` footer in the PR body | Major | `2.0.0` |

The initial policy commit releases `1.0.0`. Each later commit on `main`'s
first-parent history receives its own increment. The initial version in
`.github/release-policy.json` is immutable after introduction. Keeping this
history preserves versions even if workflows run out of order or tags are not
yet created. Recreating the remote while preserving Git history preserves the
assignments; a new source root starts a new history.

Git tags are the published version identifiers. CLI binaries receive the version
through GoReleaser linker flags. The private frontend package's `0.0.0` field is
not a release counter, and the bot does not push version-bump commits.

## Publication gates

The release workflow listens for merged-PR events, including those whose commit
messages contain CI skip directives; closed but unmerged PRs do nothing. A small
job dispatches a new execution of `release.yml` on `main`, passing the exact
squash SHA as `source_ref` and disabling `dry_run`. Only that job receives
`actions: write`. It returns when GitHub accepts the dispatch, without waiting
for the new run. Only `workflow_dispatch` executions can plan, build or publish
a release, matching the provenance build type's supported events. It pins
every source checkout to the planned merge commit. Tag preparation executes
trusted main code before it inspects the selected source. It requires the
shared CI to pass, verifies that the source is a GitHub-verified signed commit
on `main`, and creates its immutable `vX.Y.Z` tag with the built-in workflow token.
The tag itself is not separately signed; origin is established by the signed
source commit and, for public repositories, build-provenance attestations.

GoReleaser builds Linux, macOS and Windows archives for amd64 and arm64, with
SBOMs and SHA-256 checksums. The draft remains unpublished until native Linux,
macOS and Windows runners download and exercise their matching archives.
Only the highest published semantic version is promoted as `latest`.

Private repositories can test the complete release pipeline without GitHub
attestations, which are unavailable on several private-repository plans. This
skip is explicit in the workflow log. Public releases require successful
attestations; switching public automatically enables that gate. The SLSA predicate
records the actual checked-out release source separately from the executing
workflow revision, including historical retries. A checkout mismatch blocks
attestation. The first resolved dependency identifies the executing workflow;
the immutable artifact source tag follows as a separate dependency. GitHub
validates that first entry against the signing identity. Only `workflow` is
emitted as an external parameter; release source identity stays in resolved
dependencies. Apply the workflow-specific event policy in
[GitHub setup](github-setup.md) before public operation.

## Failures, retries and dry runs

A failed release retains its tag and any draft artifacts. Fix a transient
problem, then rerun the workflow: the same commit keeps the same version, a
matching unpublished draft can be repaired, and an already published release is
a successful no-op. A conflicting tag or unsigned source is an error; never move
or delete a published tag to bypass it. To fix source code, merge a new PR and
release its new version instead of altering the failed commit.

The release queue preserves up to 100 waiting runs without canceling preceding
merges. A run rejected by GitHub's queue limit can be retried manually. Version
assignment remains independent of queue order.

After the workflow exists on the default branch, use **Actions → Release → Run
workflow**. A dry run plans the version and runs CI without creating a tag or
release. To retry a real release, disable `dry_run` and select its source commit;
the commit must already belong to `main`. These operations are also available
through `gh workflow run release.yml --ref main -f dry_run=true` and
`-f dry_run=false -f source_ref=<commit>`.

For a provenance-generator repair, start a new dispatch from the corrected
workflow revision and set `source_ref` to the original release commit. Rerunning
an older run keeps its older workflow. The generator comes from the pinned
executing workflow commit; artifacts still come from the original immutable tag.

Local regression tests need no GitHub credentials:

```bash
python -m unittest discover -s scripts -p 'test_release_*.py'
python scripts/release_version.py plan
go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12
goreleaser check
```

Actionlint 1.7.12 does not yet recognize GitHub's documented `concurrency.queue`
field ([upstream issue](https://github.com/rhysd/actionlint/issues/657)). The
path-scoped exception ignores only that unknown-key diagnostic. Release queue
settings are JSON embedded in YAML and are independently validated by the Python
regressions; all other actionlint diagnostics remain blocking.

See [ADR 0034](../adr/0034-automatic-semantic-releases.md),
[ADR 0035](../adr/0035-supported-release-provenance-event.md) and the
[release checklist](checklist.md) for the contract and validation boundaries.
