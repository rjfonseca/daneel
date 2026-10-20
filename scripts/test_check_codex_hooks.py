#!/usr/bin/env python3
"""Focused regression checks for the Codex hook verification harness."""

from __future__ import annotations

import importlib.util
import os
import tempfile
import time
import unittest
from unittest import mock
from pathlib import Path


SCRIPT = Path(__file__).with_name("check_codex_hooks.py")
SPEC = importlib.util.spec_from_file_location("check_codex_hooks", SCRIPT)
assert SPEC is not None and SPEC.loader is not None
check = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(check)


class ProcessCleanupTests(unittest.TestCase):
    def _write_fake_launcher(self, path: Path) -> None:
        child_code = (
            "import os, pathlib, time; time.sleep(0.5); "
            "pathlib.Path(os.environ['DESCENDANT_MARKER']).write_text('alive')"
        )
        path.write_text(
            "#!/usr/bin/env python3\n"
            "import subprocess, sys\n"
            f"code = {child_code!r}\n"
            "subprocess.Popen([sys.executable, '-c', code], stdout=sys.stdout, stderr=sys.stderr)\n",
            encoding="utf-8",
        )
        path.chmod(0o700)

    def test_timeout_kills_descendant_after_leader_exits(self) -> None:
        with tempfile.TemporaryDirectory(prefix="dch-test-") as temp:
            root = Path(temp)
            cwd = root / "p"
            cwd.mkdir()
            binary = root / "fake-daneel"
            marker = root / "descendant-ran"
            self._write_fake_launcher(binary)
            env = os.environ.copy()
            env["DESCENDANT_MARKER"] = str(marker)
            old_ceiling = check.PROCESS_CEILING_SECONDS
            old_target = check.CAPTURE_TARGET_SECONDS
            check.PROCESS_CEILING_SECONDS = 0.1
            check.CAPTURE_TARGET_SECONDS = 0.2
            try:
                started = time.monotonic()
                with self.assertRaisesRegex(check.CheckError, "three-second ceiling"):
                    check._run_capture(str(binary), env, cwd, "synthetic-session", None)
                elapsed = time.monotonic() - started
                self.assertLess(elapsed, 0.35, "timeout cleanup exceeded its bounded allowance")
                time.sleep(0.55)
                self.assertFalse(marker.exists(), "a descendant survived the timed-out hook")
            finally:
                check.PROCESS_CEILING_SECONDS = old_ceiling
                check.CAPTURE_TARGET_SECONDS = old_target

    def test_app_server_cleanup_kills_descendant_after_leader_exits(self) -> None:
        with tempfile.TemporaryDirectory(prefix="dch-test-") as temp:
            root = Path(temp)
            binary = root / "fake-codex"
            marker = root / "descendant-ran"
            self._write_fake_launcher(binary)
            env = os.environ.copy()
            env["DESCENDANT_MARKER"] = str(marker)
            old_timeout = check.APP_SERVER_REQUEST_SECONDS
            check.APP_SERVER_REQUEST_SECONDS = 0.1
            app_server = check.AppServer(str(binary), root)
            started = time.monotonic()
            try:
                with mock.patch.dict(os.environ, env):
                    with self.assertRaises(check.CheckError):
                        app_server.__enter__()
                elapsed = time.monotonic() - started
                self.assertLess(elapsed, 0.35, "app-server cleanup exceeded its bounded allowance")
                time.sleep(0.55)
                self.assertFalse(marker.exists(), "an app-server descendant survived cleanup")
            finally:
                check.APP_SERVER_REQUEST_SECONDS = old_timeout


if __name__ == "__main__":
    unittest.main()
