# ADR 0034: Automatic Semantic Releases

* Status: accepted
* Date: 2026-10-21
* Supersedes: the manual tag creation and initial single-commit release procedure in ADR 0027

## Context and Problem Statement

Conventional Commits are documented, but publishing requires a maintainer to
choose a version and push a signed tag. Each merged pull request must instead
produce an automatic version increment and a release, including maintenance
changes. The repository is temporarily private for testing and may be recreated.

## Decision Drivers

- Publish only validated source and tested native archives.
- Give every merge a deterministic semantic version, including retries.
- Avoid extra bot credentials and automated source commits.
- Preserve release immutability, signed source and public provenance.
- Make repository setup reproducible after recreation.

## Considered Options

- Continue manual versioning and tag pushes.
- Generate a separate version-bump pull request for maintainers to merge.
- Release directly after semantic squash merges, reusing the existing gates.

## Decision Outcome

Adopt automatic releases from merged-PR events after squash merges to `main`, as specified in
[the design](../design/automatic-releases.md). Breaking changes increment the
major version, features increment minor, and every other supported conventional
type increments patch. Bootstrap at `1.0.0`. A fixed initial-version policy and
first-parent commit history determine each commit's version independently of
tags, failed publication attempts and workflow ordering.

Use the built-in workflow token to create immutable tags identifying GitHub-
verified signed commits. The bot creates no version commits and does not depend
on a tag push to trigger packaging. Preserve reusable CI, draft staging, SBOMs,
checksums and native archive verification before publication. Attestations are
mandatory for public releases; private test repositories explicitly skip this
plan-dependent feature. The SLSA predicate distinguishes built source from the
workflow revision and verifies the checkout before signing. Apply a targeted
Actions event policy for the two trusted `pull_request_target` workflows before
public operation. Queue release runs rather than canceling pending merges.

### Consequences

- Squash-only merges and semantic PR titles are required repository settings.
- Legacy tag-creation restrictions must be removed for the built-in token;
  immutable-tag rules retain no bypass. No bot integration is added as an invalid
  bypass actor.
- Tags are no longer manually signed; signed source and public build provenance
  establish origin without distributing a maintainer's signing key to CI.
- Failed drafts are retried at the same version; published releases are not rebuilt.
- Initial-version changes, rewritten history and conflicting tags fail closed.
- Repository recreation requires reapplying settings and rulesets. Source history
  may be consolidated before the first public release, never after publication.
