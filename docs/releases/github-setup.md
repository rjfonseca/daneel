# GitHub setup for automatic releases

This setup is performed once per remote repository. Repository settings and
rulesets are not restored merely by pushing the source. Repeat these steps if a
private test repository is deleted and recreated. Do not delete or recreate an
existing repository through these instructions without a separate decision.

Use `gh` authenticated as a repository administrator. The release workflow itself
uses only the automatically supplied `GITHUB_TOKEN`; do not add a PAT or private
signing key as a repository secret.

## Bootstrap main

For a new empty remote, disable GitHub Actions before the initial source push so
publication cannot start before setup is complete. Set the repository explicitly:

```bash
release_repo=example/daneel
gh api --method PUT "repos/$release_repo/actions/permissions" -F enabled=false
git push origin HEAD:refs/heads/main
```

Push the reviewed, signed source containing the release policy. A recreated
remote can retain the existing Git history. If consolidating source into a new
root before the first public release, keep the root commit signed and give it a
Conventional Commit subject. Never rewrite published release history.

Apply the tracked settings after `main` exists:

```bash
gh api --method PATCH "repos/$release_repo" --input .github/repository-settings.json
```

These settings make `main` the default, allow squash merges only and preserve the
PR title/body in the squash message. The title determines the version increment;
`BREAKING CHANGE:` / `BREAKING-CHANGE:` footers in the body determine major bumps.

## Activate branch and tag controls

List existing rulesets before creating any:

```bash
gh api "repos/$release_repo/rulesets" --jq '.[] | {id, name, enforcement}'
```

For an empty ruleset list, apply the two tracked definitions:

```bash
gh api --method POST "repos/$release_repo/rulesets" --input .github/rulesets/main.json
gh api --method POST "repos/$release_repo/rulesets" --input .github/rulesets/immutable-release-tags.json
```

For an existing matching ruleset, use `PUT repos/$release_repo/rulesets/<id>` with
the same JSON file rather than adding a duplicate. The main rules require signed
commits, pull requests, linear history, the shared CI checks and `Conventional PR
title`. Main deletion and force pushes are blocked. These controls have no bypass.

The immutable-tag rule forbids updates and deletion for everyone, including the
release bot. Remove the old **Restrict release tag creation** ruleset, if present:

```bash
gh api --method DELETE "repos/$release_repo/rulesets/<legacy-creation-ruleset-id>"
```

Select that ID from the ruleset listing; never delete the immutable-tag or main
ruleset. The built-in GitHub Actions integration cannot reliably be installed as
a ruleset bypass actor. Keeping the old administrator-only creation restriction
blocks automatic tags. The release job instead uses its explicit contents-write
permission, verifies the signed main commit and refuses tag conflicts. Tag objects
themselves are not separately signed. Normal source changes still require PRs and
signed squash commits through protected main.

Private repository plans may restrict branch/ruleset availability. Confirm the
settings supported by the test plan, then reapply and verify every control before
public publication. Do not claim a checked-in JSON file activates a GitHub rule.

## Enable and test Actions

Public repositories can block `pull_request_target` through Actions event policies.
Before enabling Actions, list the policies (including inherited policies):

```bash
gh api "repos/$release_repo/actions/policies"
```

Apply `.github/release-events-policy.json` to allow the trusted merged-PR and
title-validation workflows specifically. For a new policy:

```bash
gh api --method POST "repos/$release_repo/actions/policies" --input .github/release-events-policy.json
```

If that named policy already exists, use `PUT repos/$release_repo/actions/policies/<id>`
instead. Inspect Actions policy Insights to confirm both workflows are allowed;
an inherited restriction can still block them. The title workflow reads trusted
base code with a read-only token. The release workflow runs only for a merged PR
or an explicit dispatch and grants write permissions only to publication jobs.
Do not enable `pull_request_target` globally for arbitrary workflows.
See [GitHub event protections](https://docs.github.com/en/actions/reference/security/securely-using-pull_request_target)
and the [Actions policies API](https://docs.github.com/en/rest/actions/policies).

Enable Actions after setup. A restricted repository action policy must permit
all actions used in `.github/workflows`; release jobs explicitly request
`contents: write`, and the public provenance job also requires `id-token: write`
and `attestations: write`. Read-only defaults can remain in place.

```bash
gh api --method PUT "repos/$release_repo/actions/permissions" -F enabled=true
gh api "repos/$release_repo/rules/branches/main"
gh api "repos/$release_repo/rulesets" --jq '.[] | {id, name, enforcement}'
gh workflow run release.yml --repo "$release_repo" --ref main -f dry_run=true
```

A dry run plans the version and executes CI without creating a tag or release.
To create the initial release after inspecting the dry run, use:

```bash
gh workflow run release.yml --repo "$release_repo" --ref main -f dry_run=false
```

After bootstrap, each semantic squash merge into `main` automatically releases;
no version or tag command is required. Inspect the first real run for tag creation,
six archives, SBOMs, checksums, all native archive smoke jobs and final publication.
Private test runs explicitly skip GitHub attestations. Public runs require them.
See [automatic releases](automation.md) for retries and version examples.

## Public security settings

Before switching the repository public, enable dependency alerts, private
vulnerability reporting, secret scanning and push protection. For example:

```bash
gh api --method PUT "repos/$release_repo/vulnerability-alerts"
gh api --method PUT "repos/$release_repo/private-vulnerability-reporting"
```

Verify secret scanning and push protection in GitHub's Security settings. Public
visibility is a separate maintainer decision. Recheck protections and public
provenance on the actual public release; private tests do not establish those gates.
