"""Protect release queue behavior that the pinned actionlint cannot parse yet."""
import json
from pathlib import Path
import re
import unittest


class ReleaseWorkflowTests(unittest.TestCase):
    def job(self, name):
        workflow = Path(__file__).resolve().parent.parent / ".github" / "workflows" / "release.yml"
        match = re.search(r"^  " + re.escape(name) + r":\n(.*?)(?=^  [\w-]+:|\Z)",
                          workflow.read_text(encoding="utf-8"), re.MULTILINE | re.DOTALL)
        self.assertIsNotNone(match, f"missing release job: {name}")
        return match.group(1)

    def test_merged_pr_only_dispatches_trusted_main_workflow(self):
        job = self.job("dispatch")
        self.assertRegex(job, r"(?m)^    if: github.event_name == 'pull_request_target' && github.event.pull_request.merged == true$")
        self.assertIn("actions: write", job)
        self.assertIn("contents: read", job)
        self.assertIn("ref: ${{ github.workflow_sha }}", job)
        self.assertIn("persist-credentials: false", job)
        self.assertIn("RELEASE_SHA: ${{ github.event.pull_request.merge_commit_sha }}", job)
        self.assertIn("run: python scripts/release_github.py dispatch", job)
        self.assertNotIn("contents: write", job)
        self.assertNotIn("id-token: write", job)

    def test_release_planning_requires_dispatch_and_preserves_manual_source(self):
        job = self.job("plan")
        self.assertRegex(job, r"(?m)^    if: github.event_name == 'workflow_dispatch'$")
        self.assertIn("ref: ${{ inputs.source_ref || github.sha }}", job)
        self.assertNotIn("pull_request", job)
        self.assertNotIn("needs:", job, "a skipped dispatcher must not skip manual releases")

    def test_manual_dry_run_cannot_prepare_a_tag(self):
        job = self.job("prepare")
        self.assertIn("needs: [plan, checks]", job)
        self.assertIn("if: ${{ !inputs.dry_run }}", job)

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
