"""Ensure downloaded release archives reach the smoke test only after verification."""
import hashlib
from pathlib import Path
import subprocess
import unittest
from unittest.mock import patch

import release_archive_smoke


class ReleaseArchiveSmokeTests(unittest.TestCase):
    def exercise_release(self, checksum_contents, *, system="Linux", machine="x86_64"):
        archive_bytes = b"native release archive fixture"
        downloaded_dirs = []
        smoked_archives = []

        def run(command, **kwargs):
            if command[0] == "gh":
                self.assertEqual(command[:4], ["gh", "release", "download", "v1.0.0"])
                self.assertEqual(command[command.index("--repo") + 1], "example/daneel")
                patterns = [command[i + 1] for i, value in enumerate(command) if value == "--pattern"]
                archive_name = next(name for name in patterns if name != "checksums.txt")
                directory = Path(command[command.index("--dir") + 1])
                downloaded_dirs.append(directory)
                (directory / archive_name).write_bytes(archive_bytes)
                digest = hashlib.sha256(archive_bytes).hexdigest()
                (directory / "checksums.txt").write_text(
                    checksum_contents(archive_name, digest), encoding="utf-8")
            else:
                self.assertEqual(command[1:3], [str(Path(release_archive_smoke.__file__).with_name("smoke.py")), "--archive"])
                archive = Path(command[3])
                self.assertEqual(archive.read_bytes(), archive_bytes)
                smoked_archives.append(archive.name)
            return subprocess.CompletedProcess(command, 0)

        with patch.object(release_archive_smoke.platform, "system", return_value=system), \
                patch.object(release_archive_smoke.platform, "machine", return_value=machine), \
                patch.object(release_archive_smoke.subprocess, "run", side_effect=run):
            try:
                release_archive_smoke.verify_release("v1.0.0", "example/daneel")
            finally:
                self.assertTrue(downloaded_dirs, "release download was never attempted")
                self.assertTrue(all(not directory.exists() for directory in downloaded_dirs),
                                "downloaded release files were not cleaned up")
                self.smoked_archives = smoked_archives
        return smoked_archives

    def test_valid_checksum_allows_native_archive_smoke(self):
        for system, machine, name in [
            ("Linux", "x86_64", "daneel_1.0.0_linux_amd64.tar.gz"),
            ("Darwin", "arm64", "daneel_1.0.0_darwin_arm64.tar.gz"),
            ("Windows", "AMD64", "daneel_1.0.0_windows_amd64.zip"),
        ]:
            with self.subTest(system=system):
                smoked = self.exercise_release(
                    lambda archive, digest: f"{'0' * 64}  unrelated.zip\n{digest}  {archive}\n",
                    system=system, machine=machine)
                self.assertEqual(smoked, [name])

    def test_tampered_archive_never_reaches_smoke(self):
        with self.assertRaisesRegex(ValueError, "SHA256 mismatch"):
            self.exercise_release(lambda archive, digest: f"{'0' * 64}  {archive}\n")
        self.assertEqual(self.smoked_archives, [])

    def test_missing_checksum_entry_never_reaches_smoke(self):
        with self.assertRaisesRegex(ValueError, "exactly one checksum entry"):
            self.exercise_release(lambda archive, digest: f"{digest}  other.tar.gz\n")
        self.assertEqual(self.smoked_archives, [])

    def test_duplicate_checksum_entries_never_reach_smoke(self):
        with self.assertRaisesRegex(ValueError, "exactly one checksum entry"):
            self.exercise_release(lambda archive, digest: f"{digest}  {archive}\n" * 2)
        self.assertEqual(self.smoked_archives, [])


if __name__ == "__main__":
    unittest.main()
