"""The attestation must describe the built source, including historical retries."""
import unittest

from release_provenance import predicate


class ProvenanceTests(unittest.TestCase):
    def environment(self):
        return {
            "GITHUB_REPOSITORY": "example/daneel", "GITHUB_SERVER_URL": "https://github.com",
            "GITHUB_WORKFLOW_SHA": "b" * 40,
            "GITHUB_WORKFLOW_REF": "example/daneel/.github/workflows/release.yml@refs/heads/main",
            "GITHUB_RUN_ID": "123", "GITHUB_RUN_ATTEMPT": "2",
            "RELEASE_SHA": "a" * 40, "RELEASE_TAG": "v1.2.3",
        }

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
