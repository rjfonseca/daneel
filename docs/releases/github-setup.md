# GitHub release setup

Run these steps from the reviewed checkout with `gh` authenticated as a repository
administrator. The JSON files describe the intended controls; committing them does
not activate settings on GitHub.

## Publish the reviewed history

Confirm that the recreated repository has no old branches, tags or pull-request
refs. Consolidate the initial public source into one reviewed, signed root commit.
Push it to a `release/**` branch and inspect its hosted CI results before creating
`main` at that same commit. Activate main's required checks after this bootstrap.
Keep candidate and stable tags on the validated source commit; retain hosted
evidence in the GitHub release notes and workflow runs.

Set the repository explicitly:

```bash
release_repo=rjfonseca/daneel
gh repo view "$release_repo" --json visibility,defaultBranchRef,url
```

After reviewing the published tree, enable public access and security reporting:

```bash
gh repo edit "$release_repo" --visibility public --accept-visibility-change-consequences
gh api --method PUT "repos/$release_repo/vulnerability-alerts"
gh api --method PUT "repos/$release_repo/private-vulnerability-reporting"
```

Enable secret scanning and push protection through the repository's Security and
analysis settings, and confirm the public Security tab offers private reporting.

## Activate branch and tag controls

The checked-in rulesets require signed commits and all shared CI jobs on `main`,
prevent its deletion and force updates, allow only administrators to create
`v*` tags, and prohibit updates or deletion of existing release tags for everyone.
No actor bypasses the main or immutable-tag controls.

List existing rulesets before applying changes:

```bash
gh api "repos/$release_repo/rulesets" --jq '.[] | {id, name, enforcement}'
```

For the initial setup, create the three rulesets:

```bash
gh api --method POST "repos/$release_repo/rulesets" --input .github/rulesets/main.json
gh api --method POST "repos/$release_repo/rulesets" --input .github/rulesets/release-tag-creation.json
gh api --method POST "repos/$release_repo/rulesets" --input .github/rulesets/immutable-release-tags.json
```

When a matching ruleset already exists, use `PUT` with its returned ID and the
same input file rather than creating a duplicate. Check the actual CI job names
before changing required contexts.

Verify server-side state after applying the settings:

```bash
gh api "repos/$release_repo/rules/branches/main"
gh api "repos/$release_repo/rulesets" --jq '.[] | {id, name, enforcement}'
gh api "repos/$release_repo/private-vulnerability-reporting"
gh api "repos/$release_repo/vulnerability-alerts" --include
gh run list --repo "$release_repo" --branch main
```

Native macOS and Windows checks, artifact attestation issuance and the installed
candidate still require observed hosted results. Follow the
[release checklist](checklist.md) before creating candidate or stable tags.
