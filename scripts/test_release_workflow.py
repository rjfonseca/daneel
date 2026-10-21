"""Protect release queue behavior that the pinned actionlint cannot parse yet."""
import json
from pathlib import Path
import unittest


class ReleaseWorkflowTests(unittest.TestCase):
    def test_concurrent_merges_are_queued_without_cancellation(self):
        workflow = Path(__file__).resolve().parent.parent / ".github" / "workflows" / "release.yml"
        # JSON is valid YAML and lets the standard library validate the one new
        # schema object that actionlint currently treats as an unknown key.
        lines = [line.removeprefix("concurrency: ") for line in workflow.read_text(encoding="utf-8").splitlines()
                 if line.startswith("concurrency: ")]
        self.assertEqual(len(lines), 1, "release concurrency must be a separately validated JSON object")
        queue = json.loads(lines[0])
        self.assertEqual(set(queue), {"group", "queue", "cancel-in-progress"})
        self.assertTrue(isinstance(queue["group"], str) and queue["group"].strip())
        self.assertNotIn("${{", queue["group"], "all merges must share a fixed release queue")
        self.assertEqual(queue["queue"], "max", "a new merge must not replace a pending release")
        self.assertIs(queue["cancel-in-progress"], False)

    def test_squash_messages_preserve_semantic_title_and_breaking_body(self):
        settings = Path(__file__).resolve().parent.parent / ".github" / "repository-settings.json"
        data = json.loads(settings.read_text(encoding="utf-8"))
        self.assertIs(data["allow_squash_merge"], True)
        self.assertIs(data["allow_merge_commit"], False)
        self.assertIs(data["allow_rebase_merge"], False)
        self.assertEqual(data["squash_merge_commit_title"], "PR_TITLE")
        self.assertEqual(data["squash_merge_commit_message"], "PR_BODY")


if __name__ == "__main__":
    unittest.main()
