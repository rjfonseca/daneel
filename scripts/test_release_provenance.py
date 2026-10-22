"""The attestation must describe the built source, including historical retries."""
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import tempfile
import textwrap
import unittest

from release_provenance import predicate


class ProvenanceTests(unittest.TestCase):
    def environment(self):
        return {
            "GITHUB_REPOSITORY": "example/daneel", "GITHUB_SERVER_URL": "https://github.com",
            "GITHUB_WORKFLOW_SHA": "b" * 40,
            "GITHUB_WORKFLOW_REF": "example/daneel/.github/workflows/release.yml@refs/heads/main",
            "GITHUB_RUN_ID": "123", "GITHUB_RUN_ATTEMPT": "2",
            "GITHUB_EVENT_NAME": "workflow_dispatch", "GITHUB_REPOSITORY_ID": "456",
            "GITHUB_REPOSITORY_OWNER_ID": "789", "RUNNER_ENVIRONMENT": "github-hosted",
            "RELEASE_SHA": "a" * 40, "RELEASE_TAG": "v1.2.3",
        }

    def test_github_build_type_contains_required_execution_identity(self):
        env = self.environment()
        data = predicate(env, env["RELEASE_SHA"])
        self.assertEqual(data["buildDefinition"]["internalParameters"]["github"], {
            "event_name": "workflow_dispatch", "repository_id": "456",
            "repository_owner_id": "789", "runner_environment": "github-hosted",
        })
        self.assertEqual(data["runDetails"]["builder"]["id"],
                         "https://github.com/" + env["GITHUB_WORKFLOW_REF"])

    def test_historical_retry_distinguishes_source_and_workflow(self):
        data = predicate(self.environment(), "a" * 40)
        deps = data["buildDefinition"]["resolvedDependencies"]
        self.assertEqual(deps[0]["digest"]["gitCommit"], "a" * 40)
        self.assertEqual(deps[1]["digest"]["gitCommit"], "b" * 40)
        self.assertTrue(deps[0]["uri"].endswith("@refs/tags/v1.2.3"))
        self.assertTrue(data["runDetails"]["metadata"]["invocationId"].endswith("/123/attempts/2"))

    def test_checkout_mismatch_fails(self):
        with self.assertRaisesRegex(ValueError, "checkout"):
            predicate(self.environment(), "c" * 40)

    def test_missing_execution_identity_fails_before_signing(self):
        for key in ("GITHUB_EVENT_NAME", "GITHUB_REPOSITORY_ID",
                    "GITHUB_REPOSITORY_OWNER_ID", "RUNNER_ENVIRONMENT"):
            env = self.environment()
            del env[key]
            with self.subTest(key=key), self.assertRaisesRegex(ValueError, "execution identity"):
                predicate(env, env["RELEASE_SHA"])

    @unittest.skipUnless(os.name == "posix" and shutil.which("bash"), "The provenance shell job runs on Ubuntu")
    def test_historical_retry_executes_pinned_workflow_generator(self):
        project = Path(__file__).resolve().parent.parent
        with tempfile.TemporaryDirectory(prefix="dnl-provenance-") as directory:
            root = Path(directory) / "source"
            root.mkdir()
            env = {key: value for key, value in os.environ.items() if not key.startswith("GIT_")}
            env.update(GIT_CONFIG_GLOBAL=os.devnull, GIT_CONFIG_NOSYSTEM="1")

            def git(*args):
                return subprocess.check_output(["git", *args], cwd=root, env=env, text=True).strip()

            git("init", "--quiet")
            git("config", "user.name", "Ada Lovelace")
            git("config", "user.email", "ada@example.com")
            git("config", "commit.gpgsign", "false")
            (root / "scripts").mkdir()
            shutil.copyfile(project / "scripts/release_version.py", root / "scripts/release_version.py")
            generator = root / "scripts/release_provenance.py"
            generator.write_text('raise RuntimeError("Retired source generator must not run")\n', encoding="utf-8")
            git("add", ".")
            git("commit", "--quiet", "-m", "feat: initial source")
            source = git("rev-parse", "HEAD")
            shutil.copyfile(project / "scripts/release_provenance.py", generator)
            git("add", ".")
            git("commit", "--quiet", "-m", "fix: repair provenance")
            workflow = git("rev-parse", "HEAD")
            git("checkout", "--quiet", source)

            yaml = (project / ".github/workflows/release.yml").read_text(encoding="utf-8")
            step = yaml.split("- name: Describe the actual build source and executing workflow\n", 1)[1]
            command = re.search(r"        run: \|\n((?:          .*\n)+)", step)
            self.assertIsNotNone(command, "provenance must use the pinned workflow generator")
            env.update(self.environment(), RELEASE_SHA=source, GITHUB_WORKFLOW_SHA=workflow,
                       GITHUB_WORKSPACE=str(root), RUNNER_TEMP=directory, PYTHONDONTWRITEBYTECODE="1")
            subprocess.run(["bash", "-e", "-c", textwrap.dedent(command.group(1))],
                           cwd=root, env=env, check=True, capture_output=True, text=True)
            data = json.loads((Path(directory) / "release-provenance.json").read_text(encoding="utf-8"))
            deps = data["buildDefinition"]["resolvedDependencies"]
            self.assertEqual(deps[0]["digest"]["gitCommit"], source)
            self.assertEqual(deps[1]["digest"]["gitCommit"], workflow)
            self.assertEqual(data["buildDefinition"]["internalParameters"]["github"]["repository_id"], "456")
            self.assertEqual(git("status", "--porcelain"), "")

    def test_invalid_identifiers_fail(self):
        for key, value in [("RELEASE_SHA", "bad"), ("GITHUB_WORKFLOW_SHA", "bad"),
                           ("RELEASE_TAG", "v01.2.3"), ("GITHUB_RUN_ID", "../123"),
                           ("GITHUB_REPOSITORY", "../example"), ("GITHUB_WORKFLOW_REF", "other/repo@main")]:
            env = self.environment()
            env[key] = value
            with self.subTest(key=key), self.assertRaises(ValueError):
                predicate(env, env["RELEASE_SHA"])


if __name__ == "__main__":
    unittest.main()
