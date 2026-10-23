"""Describe actual release inputs separately from the executing workflow revision."""
import argparse
import json
import os
from pathlib import Path
import re

from release_version import Version, git


def predicate(env, observed_sha):
    source = env.get("RELEASE_SHA", "")
    workflow_sha = env.get("GITHUB_WORKFLOW_SHA", "")
    for sha in (source, workflow_sha):
        if not re.fullmatch(r"[a-f0-9]{40}", sha):
            raise ValueError("Source and workflow must identify complete commit SHAs")
    if observed_sha != source:
        raise ValueError("Release checkout does not match the planned source")
    tag = env.get("RELEASE_TAG", "")
    if not tag.startswith("v"):
        raise ValueError("A stable release tag is required")
    Version.parse(tag[1:])
    repo = env.get("GITHUB_REPOSITORY", "")
    if not re.fullmatch(r"[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+", repo):
        raise ValueError("A GitHub repository is required")
    workflow_ref = env.get("GITHUB_WORKFLOW_REF", "")
    prefix = repo + "/.github/workflows/release.yml@"
    if not workflow_ref.startswith(prefix) or not workflow_ref[len(prefix):]:
        raise ValueError("The release workflow reference is required")
    server = env.get("GITHUB_SERVER_URL", "")
    if not re.fullmatch(r"https://[A-Za-z0-9.-]+", server):
        raise ValueError("An HTTPS GitHub server is required")
    run, attempt = env.get("GITHUB_RUN_ID", ""), env.get("GITHUB_RUN_ATTEMPT", "")
    if not run.isdecimal() or not attempt.isdecimal():
        raise ValueError("Workflow run and attempt identifiers are required")
    execution = {
        "event_name": env.get("GITHUB_EVENT_NAME", ""),
        "repository_id": env.get("GITHUB_REPOSITORY_ID", ""),
        "repository_owner_id": env.get("GITHUB_REPOSITORY_OWNER_ID", ""),
        "runner_environment": env.get("RUNNER_ENVIRONMENT", ""),
    }
    if (execution["event_name"] != "workflow_dispatch"
            or not execution["repository_id"].isdecimal()
            or not execution["repository_owner_id"].isdecimal()
            or execution["runner_environment"] != "github-hosted"):
        raise ValueError("The GitHub release execution identity is required")
    url = server + "/" + repo
    return {
        "buildDefinition": {
            "buildType": "https://actions.github.io/buildtypes/workflow/v1",
            "externalParameters": {
                "workflow": {"repository": url, "path": ".github/workflows/release.yml",
                             "ref": workflow_ref[len(prefix):]},
            },
            "internalParameters": {"github": execution},
            "resolvedDependencies": [
                # GitHub validates the first dependency against the signing workflow.
                {"uri": "git+" + url + "@" + workflow_ref[len(prefix):],
                 "digest": {"gitCommit": workflow_sha}},
                {"uri": "git+" + url + "@refs/tags/" + tag, "digest": {"gitCommit": source}},
            ],
        },
        "runDetails": {
            "builder": {"id": server + "/" + workflow_ref},
            "metadata": {"invocationId": url + "/actions/runs/" + run + "/attempts/" + attempt},
        },
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    try:
        data = predicate(os.environ, git(Path.cwd(), "rev-parse", "HEAD"))
        args.output.write_text(json.dumps(data, indent=2) + "\n", encoding="utf-8")
    except ValueError as error:
        parser.exit(1, str(error) + "\n")


if __name__ == "__main__":
    main()
