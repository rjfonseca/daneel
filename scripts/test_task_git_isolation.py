"""Exercise Task's test runner with the repository context inherited from a hook."""
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest


class TaskGitIsolationTests(unittest.TestCase):
    def test_nested_git_uses_its_own_repository(self):
        for tool in ("task", "go", "git"):
            self.assertIsNotNone(shutil.which(tool), f"{tool} is required")

        with tempfile.TemporaryDirectory(prefix="dnl-hook-isolation-") as directory:
            root = Path(directory)
            parent = root / "parent"
            project = root / "project"
            project.mkdir()
            env = {key: value for key, value in os.environ.items() if not key.startswith("GIT_")}
            env.update(GIT_CONFIG_GLOBAL=os.devnull, GIT_CONFIG_NOSYSTEM="1")
            subprocess.run(["git", "init", "--quiet", str(parent)], env=env, check=True)
            (parent / "sentinel.txt").write_text("Preserve the hook repository.\n", encoding="utf-8")
            subprocess.run(["git", "-C", str(parent), "add", "sentinel.txt"], env=env, check=True)
            git_dir = parent / ".git"
            original_head = (git_dir / "HEAD").read_bytes()
            original_index = (git_dir / "index").read_bytes()
            original_config = (git_dir / "config").read_bytes()

            taskfile = Path(__file__).resolve().parent.parent / "Taskfile.yml"
            shutil.copyfile(taskfile, project / "Taskfile.yml")
            (project / "go.mod").write_text("module example.com/task-isolation\n\ngo 1.27\n", encoding="utf-8")
            (project / "isolation_test.go").write_text('''package isolation

import (
    "os"
    "os/exec"
    "path/filepath"
    "testing"
)

func TestNestedRepository(t *testing.T) {
    dir := t.TempDir()
    output, err := exec.Command("git", "init", "--quiet", dir).CombinedOutput()
    if err != nil { t.Fatalf("git init: %v: %s", err, output) }
    info, err := os.Stat(filepath.Join(dir, ".git"))
    if err != nil || !info.IsDir() {
        t.Fatalf("git did not initialize the requested temporary repository: %v", err)
    }
}
''', encoding="utf-8")

            env.update(GIT_DIR=str(git_dir), GIT_WORK_TREE=str(parent),
                       GIT_INDEX_FILE=str(git_dir / "index"), GIT_OBJECT_DIRECTORY=str(git_dir / "objects"))
            result = subprocess.run(["task", "test"], cwd=project, env=env, text=True,
                                    capture_output=True, timeout=120)
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            self.assertEqual((git_dir / "HEAD").read_bytes(), original_head)
            self.assertEqual((git_dir / "index").read_bytes(), original_index)
            self.assertEqual((git_dir / "config").read_bytes(), original_config)


if __name__ == "__main__":
    unittest.main()
