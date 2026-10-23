"""Dispatch releases, create immutable tags and promote validated GitHub drafts."""
import argparse
import json
import os
from pathlib import Path
import re
import urllib.error
import urllib.request

from release_version import Version, git, plan, write_outputs


class APIError(ValueError):
    def __init__(self, status):
        self.status = status
        super().__init__(f"GitHub API request failed ({status}); check workflow permissions and repository rules")


class GitHub:
    def __init__(self):
        repository = os.environ.get("GITHUB_REPOSITORY", "")
        token = os.environ.get("GH_TOKEN", "")
        if not re.fullmatch(r"[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+", repository) or not token:
            raise ValueError("GITHUB_REPOSITORY and GH_TOKEN are required")
        self.base = os.environ.get("GITHUB_API_URL", "https://api.github.com").rstrip("/") + "/repos/" + repository
        self.token = token

    def request(self, method, path, body=None, *, missing=False):
        data = None if body is None else json.dumps(body).encode("utf-8")
        request = urllib.request.Request(self.base + path, data=data, method=method, headers={
            "Authorization": "Bearer " + self.token,
            "Accept": "application/vnd.github+json", "Content-Type": "application/json",
        })
        try:
            with urllib.request.urlopen(request, timeout=60) as response:
                if response.status == 204:
                    return None
                return json.load(response)
        except urllib.error.HTTPError as error:
            if missing and error.code == 404:
                return None
            raise APIError(error.code) from None
        except urllib.error.URLError:
            raise ValueError("GitHub API unavailable; retry the release workflow") from None


def dispatch(api, sha):
    api.request("POST", "/actions/workflows/release.yml/dispatches", {
        "ref": "main", "inputs": {"dry_run": "false", "source_ref": sha},
    })


def tag_commit(api, tag):
    ref = api.request("GET", "/git/ref/tags/" + tag, missing=True)
    if ref is None:
        return None
    target = ref.get("object", {})
    for _ in range(8):
        if target.get("type") == "commit":
            return target["sha"]
        if target.get("type") != "tag":
            break
        target = api.request("GET", "/git/tags/" + target["sha"]).get("object", {})
    raise ValueError("Release tag does not identify a source commit")


def releases(api):
    page = 1
    while True:
        batch = api.request("GET", f"/releases?per_page=100&page={page}")
        yield from batch
        if len(batch) < 100:
            return
        page += 1


def find_release(api, tag):
    # The tag-name endpoint is published-only. Contents-write access to the list
    # endpoint includes drafts, which are the pipeline's publication boundary.
    matches = [release for release in releases(api) if release.get("tag_name") == tag]
    if len(matches) > 1:
        raise ValueError("Multiple releases identify this tag; inspect the drafts before retrying")
    return matches[0] if matches else None


def prepare(api, root, sha, tag):
    expected = plan(root, sha)
    if expected["sha"] != sha or expected["tag"] != tag:
        raise ValueError("Release version does not match the source history")
    if sha not in git(root, "rev-list", "--first-parent", "refs/remotes/origin/main").splitlines():
        raise ValueError("Release source must belong to main's first-parent history")
    repository = api.request("GET", "")
    if (repository.get("default_branch") != "main" or not repository.get("allow_squash_merge")
            or repository.get("allow_merge_commit") or repository.get("allow_rebase_merge")):
        raise ValueError("Configure main as default with squash-only merges before releasing")
    source = api.request("GET", "/commits/" + sha)
    if source.get("commit", {}).get("verification", {}).get("verified") is not True:
        raise ValueError("Release source must be a GitHub-verified signed commit")
    existing = tag_commit(api, tag)
    if existing is not None and existing != sha:
        raise ValueError("Release tag already identifies a different commit; tags must never move")
    release = find_release(api, tag)
    if release and existing is None:
        raise ValueError("Release exists without its immutable tag; maintainer inspection is required")
    if existing is None:
        try:
            api.request("POST", "/git/refs", {"ref": "refs/tags/" + tag, "sha": sha})
        except APIError as error:
            # A repeated request can race a completed tag creation. Never overwrite.
            if error.status != 422 or tag_commit(api, tag) != sha:
                raise
    return release is None or release.get("draft") is True


def publish(api, sha, tag):
    version = Version.parse(tag.removeprefix("v"))
    if tag_commit(api, tag) != sha:
        raise ValueError("Release tag identifies a different commit")
    release = find_release(api, tag)
    if not release:
        raise ValueError("No staged draft exists for the validated release")
    if release.get("draft") is not True:
        return
    latest = True
    for other in releases(api):
        if other.get("draft") or other.get("prerelease"):
            continue
        try:
            if Version.parse(other["tag_name"].removeprefix("v")) > version:
                latest = False
        except (KeyError, ValueError):
            continue
    api.request("PATCH", f"/releases/{int(release['id'])}", {"draft": False, "make_latest": str(latest).lower()})


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=("dispatch", "prepare", "publish"))
    parser.add_argument("--sha", default=os.environ.get("RELEASE_SHA"))
    parser.add_argument("--tag", default=os.environ.get("RELEASE_TAG"))
    parser.add_argument("--repo-path", type=Path, default=Path.cwd())
    args = parser.parse_args()
    try:
        if not args.sha or not re.fullmatch(r"[a-f0-9]{40}", args.sha):
            raise ValueError("RELEASE_SHA must identify a complete source commit")
        if args.command == "dispatch":
            dispatch(GitHub(), args.sha)
            print("Release workflow dispatched for the merged source")
            return
        if not args.tag or not args.tag.startswith("v"):
            raise ValueError("RELEASE_TAG must be a stable v-prefixed semantic version")
        Version.parse(args.tag[1:])
        api = GitHub()
        if args.command == "prepare":
            needed = prepare(api, args.repo_path, args.sha, args.tag)
            write_outputs(os.environ.get("GITHUB_OUTPUT"), {"release": str(needed).lower()})
            print("Release tag ready" if needed else "Release already published; no changes made")
        else:
            publish(api, args.sha, args.tag)
            print("Validated release is published")
    except ValueError as error:
        parser.exit(1, str(error) + "\n")


if __name__ == "__main__":
    main()
