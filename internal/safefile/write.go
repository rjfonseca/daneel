// Package safefile persists vault files with recoverable, atomic replacements.
package safefile

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"
)

var maxBackupDirs = 50

// WriteFile stages a file inside vaultRoot and keeps the old content under
// .daneel/backups/<unique-id>/<relative-path> before replacing an existing file.
// path may be absolute, but must resolve inside vaultRoot. Unchanged files are skipped.
func WriteFile(vaultRoot, path string, data []byte, mode fs.FileMode) error {
	rootPath, err := filepath.Abs(vaultRoot)
	if err != nil {
		return err
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(rootPath, absPath)
	if err != nil || !filepath.IsLocal(rel) || rel == "." {
		return fmt.Errorf("file must be inside vault")
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	old, err := root.ReadFile(rel)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read original: %w", err)
	}
	if err == nil {
		if bytes.Equal(old, data) {
			return nil
		}
		info, err := root.Stat(rel)
		if err != nil {
			return err
		}
		mode = info.Mode().Perm()
		backup := filepath.Join(".daneel", "backups", rand.Text(), rel)
		if err := root.MkdirAll(filepath.Dir(backup), 0700); err != nil {
			return fmt.Errorf("create backup directory: %w", err)
		}
		if err := stageFile(root, backup, old, 0600); err != nil {
			return fmt.Errorf("save backup: %w", err)
		}
		pruneBackups(filepath.Join(rootPath, ".daneel", "backups"), maxBackupDirs)
	}
	if err := root.MkdirAll(filepath.Dir(rel), 0750); err != nil {
		return err
	}
	return stageFile(root, rel, data, mode)
}

type backupEntry struct {
	name    string
	modTime time.Time
}

func pruneBackups(backupsDir string, keep int) {
	if keep <= 0 {
		return
	}
	entries, err := os.ReadDir(backupsDir)
	if err != nil || len(entries) <= keep {
		return
	}
	var dirs []backupEntry
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		dirs = append(dirs, backupEntry{name: entry.Name(), modTime: info.ModTime()})
	}
	if len(dirs) <= keep {
		return
	}
	sort.Slice(dirs, func(i, j int) bool {
		if !dirs[i].modTime.Equal(dirs[j].modTime) {
			return dirs[i].modTime.After(dirs[j].modTime)
		}
		return dirs[i].name > dirs[j].name
	})
	for _, d := range dirs[keep:] {
		_ = os.RemoveAll(filepath.Join(backupsDir, d.name))
	}
}

func stageFile(root *os.Root, path string, data []byte, mode fs.FileMode) error {
	temp := filepath.Join(filepath.Dir(path), ".daneel-write-"+rand.Text())
	f, err := root.OpenFile(temp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	defer func() { _ = root.Remove(temp) }()
	if err = f.Chmod(mode); err != nil {
		_ = f.Close()
		return err
	}
	if _, err = f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = root.Rename(temp, path); err != nil {
		return fmt.Errorf("replace file: %w", err)
	}
	return nil
}
