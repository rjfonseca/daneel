"""Release retries and safety gates against a fake GitHub API and real Git."""
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

import release_github


class FakeGitHub:
    def __init__(self, sha, tag):
        self.sha, self.tag = sha, tag
        self.calls = []
        self.ref = None
        self.release = None
        self.published = []
        self.signed = True
        self.repository = {"default_branch": "main", "allow_squash_merge": True,
                           "allow_merge_commit": False, "allow_rebase_merge": False}
        self.race = False

    def request(self, method, path, body=None, *, missing=False):
        self.calls.append((method, path, body))
        if method == "POST" and path == "/git/refs":
            self.ref = {"object": {"type": "commit", "sha": body["sha"]}}
            if self.race:
                raise release_github.APIError(422)
            return self.ref
        if method == "PATCH" and path.startswith("/releases/"):
            self.release.update(body)
            return self.release
        if path == "":
            return self.repository
        if path == "/commits/" + self.sha:
            return {"commit": {"verification": {"verified": self.signed}}}
        if path == "/git/ref/tags/" + self.tag:
            return self.ref
        if path == "/releases/tags/" + self.tag:
            # GitHub's tag lookup exposes published releases, not drafts.
            return self.release if self.release and not self.release["draft"] else None
        if path.startswith("/releases?per_page=100&page="):
            page = int(path.rsplit("=", 1)[1])
            releases = self.published + ([self.release] if self.release else [])
            return releases[(page - 1) * 100:page * 100]
        raise AssertionError(f"Unexpected API operation: {method} {path}")

    def mutations(self):
        return [call for call in self.calls if call[0] not in ("GET", "HEAD")]


class GitHubReleaseTests(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory(prefix="dnl-release-api-")
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)
        self.env = {k: v for k, v in os.environ.items() if not k.startswith("GIT_")}
        self.env.update(GIT_CONFIG_GLOBAL=os.devnull, GIT_CONFIG_NOSYSTEM="1")
        self.git("init", "--quiet", "--initial-branch=main")
        self.git("config", "user.name", "Ada Lovelace")
        self.git("config", "user.email", "ada@example.com")
        self.git("config", "commit.gpgsign", "false")
        (self.root / ".github").mkdir()
        (self.root / ".github" / "release-policy.json").write_text('{"initial_version":"1.0.0"}', encoding="utf-8")
        self.git("add", ".")
        self.git("commit", "--quiet", "-m", "feat: automate releases")
        self.sha = self.git("rev-parse", "HEAD")
        self.git("update-ref", "refs/remotes/origin/main", self.sha)
        self.tag = "v1.0.0"
        self.api = FakeGitHub(self.sha, self.tag)

    def git(self, *args):
        return subprocess.check_output(["git", "-C", str(self.root), *args], env=self.env, text=True).strip()

    def prepare(self):
        return release_github.prepare(self.api, self.root, self.sha, self.tag)

    def existing_tag(self, sha=None):
        self.api.ref = {"object": {"type": "commit", "sha": sha or self.sha}}

    def draft(self):
        self.existing_tag()
        self.api.release = {"id": 7, "tag_name": self.tag, "draft": True, "prerelease": False}

    def test_verified_main_commit_creates_exactly_one_tag(self):
        self.assertTrue(self.prepare())
        self.assertEqual(self.api.mutations(), [("POST", "/git/refs", {"ref": "refs/tags/v1.0.0", "sha": self.sha})])
        self.assertTrue(self.prepare())
        self.assertEqual(len(self.api.mutations()), 1)

    def test_unsigned_source_is_rejected_before_mutation(self):
        self.api.signed = False
        with self.assertRaisesRegex(ValueError, "signed"):
            self.prepare()
        self.assertEqual(self.api.mutations(), [])

    def test_version_mismatch_is_rejected_before_mutation(self):
        with self.assertRaisesRegex(ValueError, "version"):
            release_github.prepare(self.api, self.root, self.sha, "v1.0.1")
        self.assertEqual(self.api.mutations(), [])

    def test_tag_cannot_move_to_another_commit(self):
        self.existing_tag("0" * 40)
        with self.assertRaisesRegex(ValueError, "different commit"):
            self.prepare()
        self.assertEqual(self.api.mutations(), [])

    def test_a_matching_draft_can_be_retried(self):
        self.draft()
        self.assertTrue(self.prepare())
        self.assertEqual(self.api.mutations(), [])

    def test_a_release_without_its_original_tag_is_not_silently_retagged(self):
        self.draft()
        self.api.ref = None
        with self.assertRaisesRegex(ValueError, "immutable tag"):
            self.prepare()
        self.assertEqual(self.api.mutations(), [])

    def test_published_release_is_a_successful_noop(self):
        self.draft()
        self.api.release["draft"] = False
        self.assertFalse(self.prepare())
        release_github.publish(self.api, self.sha, self.tag)
        self.assertEqual(self.api.mutations(), [])

    def test_matching_tag_creation_race_is_idempotent(self):
        self.api.race = True
        self.assertTrue(self.prepare())

    def test_feature_commit_cannot_be_published_before_merge(self):
        self.git("commit", "--quiet", "--allow-empty", "-m", "fix: repair search")
        sha = self.git("rev-parse", "HEAD")
        with self.assertRaisesRegex(ValueError, "main"):
            release_github.prepare(self.api, self.root, sha, "v1.0.1")
        self.assertEqual(self.api.mutations(), [])

    def test_repository_must_use_semantic_squash_merges(self):
        self.api.repository["allow_merge_commit"] = True
        with self.assertRaisesRegex(ValueError, "squash"):
            self.prepare()
        self.assertEqual(self.api.mutations(), [])

    def test_older_queued_release_does_not_replace_latest(self):
        self.draft()
        self.api.published = [{"tag_name": "v1.1.0", "draft": False, "prerelease": False}]
        release_github.publish(self.api, self.sha, self.tag)
        self.assertEqual(self.api.mutations()[-1], ("PATCH", "/releases/7", {"draft": False, "make_latest": "false"}))

    def test_highest_version_becomes_latest_with_paginated_history(self):
        self.draft()
        self.api.published = [{"tag_name": "v0.0.1", "draft": False, "prerelease": False}] * 100
        self.api.published.append({"tag_name": "v9.0.0", "draft": True, "prerelease": False})
        release_github.publish(self.api, self.sha, self.tag)
        self.assertEqual(self.api.mutations()[-1][2]["make_latest"], "true")
        self.assertTrue(any("page=2" in path for _, path, _ in self.api.calls))

    def test_missing_draft_cannot_be_published(self):
        self.existing_tag()
        with self.assertRaisesRegex(ValueError, "draft"):
            release_github.publish(self.api, self.sha, self.tag)
        self.assertEqual(self.api.mutations(), [])


if __name__ == "__main__":
    unittest.main()
