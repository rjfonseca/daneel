# ADR 0035: Supported Release Provenance Event

* Status: accepted
* Date: 2026-10-23
* Amends: the merged-PR execution path in ADR 0034

## Context and Problem Statement

The automatic release workflow receives `pull_request_target` events after a
squash merge. The GitHub Actions `workflow/v1` provenance build type explicitly
supports `create`, `release`, `push` and `workflow_dispatch`, excluding merged-PR
events. Fixing dependency order alone does not resolve this contract mismatch.
Using only `push` would lose the existing guarantee that commit-message skip
directives do not suppress automatic releases.

## Decision Drivers

- Use an event supported by the declared provenance build type.
- Preserve automatic releases for every semantic squash merge, including skip directives.
- Preserve the exact merged source SHA, signed-source checks and immutable tags.
- Keep the built-in token and existing publication gates.
- Avoid recursive dispatch and waiting on a run that shares the same release queue.

## Considered Options

- Continue building directly under `pull_request_target` with an unsupported build type.
- Replace merged-PR events with `push` to `main`, accepting skipped releases.
- Dispatch the release workflow from a small merged-PR job.

## Decision Outcome

Use a merged-PR job in the existing release workflow to request a new
`workflow_dispatch` execution of `release.yml` on `main`. Its inputs contain
`dry_run=false` and the exact `merge_commit_sha`. Only merged PRs dispatch;
closed unmerged PRs do nothing. Dispatch executions cannot dispatch themselves.

The dispatcher checks out the pinned executing workflow revision, with no
persisted credentials. Only this job receives `actions: write`, along with
`contents: read`. It runs no artifact build, tag preparation or publication.
The dispatch API's successful HTTP 204 response is accepted without JSON parsing;
API errors fail the job. No additional token or secret is introduced.

Only `workflow_dispatch` executions can enter release planning and its dependent
CI, tag, artifact, attestation, native archive and publication jobs. Explicit
manual dry runs and historical source retries retain their existing behavior.
The provenance generator rejects merged-PR events and emits only the build type's
defined `workflow` external parameter. The actual release tag and source remain
recorded as a separate resolved dependency after the signing workflow dependency.

Both executions share the existing release queue. The dispatcher returns after
the API accepts the request; it never waits for the child run while holding the
queue. Source-based version assignment remains independent of queue order.

### Consequences

- Each merged PR produces a short dispatch run followed by the full release run.
- Repository policies must allow `actions: write` for the dispatcher and both
  existing release events. The checked-in event allowlist already includes them.
- Dispatch acceptance does not establish downstream success; inspect the child run.
- The real dispatch, attestation and native archive gates still require hosted
  evidence after merge. Branch CI cannot prove the workflow-token permissions.

## References

- [GitHub Actions workflow build type](https://github.com/actions/buildtypes/blob/main/workflow/v1/README.md)
- [Triggering a workflow from a workflow](https://docs.github.com/en/actions/how-tos/write-workflows/choose-when-workflows-run/trigger-a-workflow)
