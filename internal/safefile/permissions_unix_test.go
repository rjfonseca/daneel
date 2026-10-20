//go:build !windows

package safefile

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
)

func TestReplacementPreservesPermissionsWithRestrictiveUmask(t *testing.T) {
	if root := os.Getenv("DANEEL_PERMISSION_TEST_ROOT"); root != "" {
		syscall.Umask(0077)
		path := filepath.Join(root, "note.md")
		if err := WriteFile(root, path, []byte("updated"), 0600); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0640 {
			t.Fatalf("lost original group permissions: %v", info.Mode())
		}
		return
	}
	root := t.TempDir()
	path := filepath.Join(root, "note.md")
	if err := os.WriteFile(path, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0640); err != nil { //nolint:gosec // verifies preservation of existing group-readable permissions
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	child := exec.Command(executable, "-test.run=^TestReplacementPreservesPermissionsWithRestrictiveUmask$")
	child.Env = append(os.Environ(), "DANEEL_PERMISSION_TEST_ROOT="+root)
	if output, err := child.CombinedOutput(); err != nil {
		t.Fatalf("replacement subprocess: %s %v", output, err)
	}
}
