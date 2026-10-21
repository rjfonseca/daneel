"""Exercise semantic releases with real disposable Git histories."""
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

import release_version


class SemanticMessageTests(unittest.TestCase):
    def test_supported_types_increment_each_merge(self):
        for kind in ("fix", "chore", "docs", "refactor", "perf", "test", "build", "ci", "style", "revert"):
            with self.subTest(kind=kind):
                self.assertEqual(release_version.bump_for_message(f"{kind}(cli): improve behavior"), "patch")
        self.assertEqual(release_version.bump_for_message("feat: add search"), "minor")

    def test_breaking_changes_take_precedence(self):
        for message in ("feat!: replace search", "fix(cli)!: remove flag",
                        "chore: replace configuration\n\nBREAKING CHANGE: old config is unsupported",
                        "docs: change contract\n\nBREAKING-CHANGE: new contract"):
            with self.subTest(message=message):
                self.assertEqual(release_version.bump_for_message(message), "major")

    def test_prose_and_lowercase_footer_are_not_breaking_markers(self):
        self.assertEqual(release_version.bump_for_message("fix: parse input\n\nDescribe BREAKING CHANGE: in documentation"), "patch")
        self.assertEqual(release_version.bump_for_message("fix: parse input\n\nbreaking change: an example"), "patch")

    def test_invalid_titles_fail_instead_of_skipping_a_merge(self):
        for message in ("", "fix: ", "Merge pull request #1", "feature: add search",
                        "fix(scope: broken", "fix: first\nsecond", " feat: hidden", "fix\n: text"):
            with self.subTest(message=message):
                with self.assertRaises(ValueError):
                    release_version.bump_for_message(message)


class ReleaseHistoryTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="dnl-version-")
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.env = {key: value for key, value in os.environ.items() if not key.startswith("GIT_")}
        self.env.update(GIT_CONFIG_GLOBAL=os.devnull, GIT_CONFIG_NOSYSTEM="1")
        self.git("init", "--quiet", "--initial-branch=main")
        self.git("config", "user.name", "Ada Lovelace")
        self.git("config", "user.email", "ada@example.com")
        self.git("config", "commit.gpgsign", "false")
        self.commit("feat: previous source")
        policy = self.root / ".github" / "release-policy.json"
        policy.parent.mkdir()
        policy.write_text(json.dumps({"initial_version": "1.0.0"}), encoding="utf-8")
        self.anchor = self.commit("feat: automate releases")

    def git(self, *args):
        return subprocess.check_output(["git", "-C", str(self.root), *args], env=self.env, text=True).strip()

    def commit(self, message):
        self.git("add", ".")
        self.git("commit", "--quiet", "--allow-empty", "-m", message)
        return self.git("rev-parse", "HEAD")

    def test_bootstrap_ignores_history_before_policy(self):
        plan = release_version.plan(self.root, self.anchor)
        self.assertEqual((plan["version"], plan["tag"], plan["sha"]), ("1.0.0", "v1.0.0", self.anchor))

    def test_each_first_parent_commit_increments_its_own_version(self):
        patch = self.commit("chore: update dependencies")
        minor = self.commit("feat(search): add filters")
        major = self.commit("fix!: remove legacy flags")
        for sha, version in ((patch, "1.0.1"), (minor, "1.1.0"), (major, "2.0.0")):
            with self.subTest(sha=sha):
                self.assertEqual(release_version.plan(self.root, sha)["version"], version)

    def test_tags_and_reordered_execution_cannot_change_assigned_versions(self):
        first = self.commit("fix: repair search")
        second = self.commit("feat: add filters")
        later = release_version.plan(self.root, second)
        self.git("tag", later["tag"], second)
        self.assertEqual(release_version.plan(self.root, first)["version"], "1.0.1")
        self.assertEqual(release_version.plan(self.root, second), later)
        self.git("tag", "v99.0.0", second)
        self.assertEqual(release_version.plan(self.root, second), later)

    def test_policy_cannot_be_changed_after_its_introduction(self):
        (self.root / ".github" / "release-policy.json").write_text('{"initial_version":"9.0.0"}', encoding="utf-8")
        self.commit("chore: alter initial version")
        with self.assertRaisesRegex(ValueError, "initial version"):
            release_version.plan(self.root, "HEAD")

    def test_nonsemantic_history_is_an_error(self):
        self.commit("update dependencies")
        with self.assertRaisesRegex(ValueError, "Conventional"):
            release_version.plan(self.root, "HEAD")

    def test_nonsquash_merge_is_an_error(self):
        self.git("checkout", "--quiet", "-b", "feature")
        self.commit("feat: introduce feature")
        self.git("checkout", "--quiet", "main")
        self.git("merge", "--quiet", "--no-ff", "feature", "-m", "feat: merge feature")
        with self.assertRaisesRegex(ValueError, "squash"):
            release_version.plan(self.root, "HEAD")

    def test_missing_policy_and_invalid_versions_fail(self):
        before = self.git("rev-parse", self.anchor + "^")
        with self.assertRaisesRegex(ValueError, "policy"):
            release_version.plan(self.root, before)
        for value in ("01.0.0", "v1.0.0", "1.0", "1.0.0-rc.1", "1.0.0\ninjected=true"):
            with self.subTest(value=value), self.assertRaises(ValueError):
                release_version.Version.parse(value)

    def test_inherited_hook_context_does_not_redirect_git(self):
        from unittest.mock import patch
        with patch.dict(os.environ, {"GIT_DIR": str(self.root / "missing"), "GIT_WORK_TREE": "/nonexistent"}):
            self.assertEqual(release_version.plan(self.root, self.anchor)["version"], "1.0.0")


if __name__ == "__main__":
    unittest.main()
