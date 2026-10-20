package safefile

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestWritePreservesOriginalAndPermissions(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "notes", "note.md")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("original"), 0640); err != nil { //nolint:gosec // verifies preservation of existing group-readable permissions
		t.Fatal(err)
	}
	if err := WriteFile(root, path, []byte("updated"), 0600); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "updated" {
		t.Fatalf("update: %s %v", data, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	// Windows does not expose POSIX group permission bits.
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0640 {
		t.Fatalf("changed permissions: %v", info.Mode())
	}
	backups, err := filepath.Glob(filepath.Join(root, ".daneel", "backups", "*", "notes", "note.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != 1 {
		t.Fatalf("expected one backup, got %v", backups)
	}
	original, err := os.ReadFile(backups[0])
	if err != nil || string(original) != "original" {
		t.Fatalf("backup: %s %v", original, err)
	}
	if err := WriteFile(root, path, []byte("updated"), 0600); err != nil {
		t.Fatal(err)
	}
	again, _ := filepath.Glob(filepath.Join(root, ".daneel", "backups", "*", "notes", "note.md"))
	if len(again) != 1 {
		t.Fatal("unchanged writes should not create backups")
	}
}

func TestWriteFailsClosedWhenBackupCannotBeCreated(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "note.md")
	if err := os.WriteFile(path, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".daneel"), []byte("blocked"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := WriteFile(root, path, []byte("updated"), 0600); err == nil {
		t.Fatal("expected backup failure")
	}
	data, _ := os.ReadFile(path)
	if string(data) != "original" {
		t.Fatal("original changed after backup failure")
	}
}

func TestWriteRejectsExternalPaths(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	target := filepath.Join(outside, "note.md")
	if err := WriteFile(root, target, []byte("unexpected"), 0600); err == nil {
		t.Fatal("external path accepted")
	}
	if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := WriteFile(root, filepath.Join(root, "linked", "note.md"), []byte("unexpected"), 0600); err == nil {
		t.Fatal("external symlink accepted")
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatal("external target was touched")
	}
}
