package safefile

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// ReadFile reads a local relative path without following symlinks outside root.
func ReadFile(rootPath, relPath string) ([]byte, error) {
	if !filepath.IsLocal(relPath) || filepath.Clean(relPath) == "." {
		return nil, fmt.Errorf("path must be local to the vault: %w", fs.ErrPermission)
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()
	return root.ReadFile(relPath)
}
