//nolint:revive // package comment not required
package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rjfonseca/daneel/internal/config"
	"github.com/rjfonseca/daneel/internal/search"
)

func setupToonTestVault(t *testing.T) (*config.Config, *search.Engine) {
	t.Helper()
	tempDir := t.TempDir()

	notesDir := filepath.Join(tempDir, "notes")
	_ = os.MkdirAll(notesDir, 0750)
	_ = os.WriteFile(filepath.Join(notesDir, "test.md"), []byte("# Architecture Decision\n\nLink to [[target]]"), 0600)

	peopleDir := filepath.Join(notesDir, "pessoas")
	_ = os.MkdirAll(peopleDir, 0750)
	personContent := "---\ntype: person\nrole: Staff Engineer\nsquad: Platform\nemail: ada@example.com\n---\n# Ada Lovelace\n"
	_ = os.WriteFile(filepath.Join(peopleDir, "ada.md"), []byte(personContent), 0600)

	dbPath := filepath.Join(tempDir, "cache.db")
	engine, err := search.Open(dbPath, tempDir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	_, _, _ = engine.IndexVault(context.Background(), []string{"notes"}, true)

	c := &config.Config{
		VaultPath: tempDir,
		DBPath:    dbPath,
		Vault: config.VaultConfig{
			IndexedDirs: []string{"notes"},
		},
		Workspace: config.WorkspaceConfig{
			WorkDir: tempDir,
		},
	}
	return c, engine
}

func TestToonFlag_Search(t *testing.T) {
	c, engine := setupToonTestVault(t)
	defer engine.Close() //nolint:errcheck,gosec

	origCfg := cfg
	origToon := toonOut
	cfg = c
	toonOut = true
	defer func() {
		cfg = origCfg
		toonOut = origToon
	}()

	out, err := captureStdout(t, func() error {
		return searchCmd.RunE(searchCmd, []string{"Architecture"})
	})
	if err != nil {
		t.Fatalf("searchCmd failed: %v", err)
	}

	if !strings.Contains(out, "results[") || !strings.Contains(out, "Architecture Decision") {
		t.Errorf("expected TOON format with results[...], got: %s", out)
	}
}

func TestToonFlag_People(t *testing.T) {
	c, engine := setupToonTestVault(t)
	defer engine.Close() //nolint:errcheck,gosec

	origCfg := cfg
	origToon := toonOut
	cfg = c
	toonOut = true
	defer func() {
		cfg = origCfg
		toonOut = origToon
	}()

	out, err := captureStdout(t, func() error {
		return peopleCmd.RunE(peopleCmd, []string{"Ada"})
	})
	if err != nil {
		t.Fatalf("peopleCmd failed: %v", err)
	}

	if !strings.Contains(out, "people[") || !strings.Contains(out, "Ada Lovelace") {
		t.Errorf("expected TOON format with people[...], got: %s", out)
	}
}

func TestToonFlag_Projects(t *testing.T) {
	c, engine := setupToonTestVault(t)
	defer engine.Close() //nolint:errcheck,gosec

	origCfg := cfg
	origToon := toonOut
	cfg = c
	toonOut = true
	defer func() {
		cfg = origCfg
		toonOut = origToon
	}()

	out, err := captureStdout(t, func() error {
		return projectsCmd.RunE(projectsCmd, []string{})
	})
	if err != nil {
		t.Fatalf("projectsCmd failed: %v", err)
	}

	if !strings.Contains(out, "projects[") {
		t.Errorf("expected TOON format with projects[...], got: %s", out)
	}
}

func TestToonFlag_Backlinks(t *testing.T) {
	c, engine := setupToonTestVault(t)
	defer engine.Close() //nolint:errcheck,gosec

	origCfg := cfg
	origToon := toonOut
	cfg = c
	toonOut = true
	defer func() {
		cfg = origCfg
		toonOut = origToon
	}()

	out, err := captureStdout(t, func() error {
		return backlinksCmd.RunE(backlinksCmd, []string{"notes/target.md"})
	})
	if err != nil {
		t.Fatalf("backlinksCmd failed: %v", err)
	}

	if !strings.Contains(out, "backlinks[") {
		t.Errorf("expected TOON format with backlinks[...], got: %s", out)
	}
}

func TestToonFlag_Doctor(t *testing.T) {
	c, engine := setupToonTestVault(t)
	defer engine.Close() //nolint:errcheck,gosec

	origCfg := cfg
	origToon := toonOut
	cfg = c
	toonOut = true
	defer func() {
		cfg = origCfg
		toonOut = origToon
	}()

	out, err := captureStdout(t, func() error {
		return doctorCmd.RunE(doctorCmd, []string{})
	})
	if err != nil {
		t.Fatalf("doctorCmd failed: %v", err)
	}

	if !strings.Contains(out, "diagnostics[") {
		t.Errorf("expected TOON format with diagnostics[...], got: %s", out)
	}
}

func TestToonFlag_Workspace(t *testing.T) {
	c, engine := setupToonTestVault(t)
	defer engine.Close() //nolint:errcheck,gosec

	origCfg := cfg
	origToon := toonOut
	cfg = c
	toonOut = true
	defer func() {
		cfg = origCfg
		toonOut = origToon
	}()

	out, err := captureStdout(t, func() error {
		return workspaceCmd.RunE(workspaceCmd, []string{})
	})
	if err != nil {
		t.Fatalf("workspaceCmd failed: %v", err)
	}

	if !strings.Contains(out, "workspace_repos[") {
		t.Errorf("expected TOON format with workspace_repos[...], got: %s", out)
	}
}

func TestToonFlag_Dream(t *testing.T) {
	c, engine := setupToonTestVault(t)
	defer engine.Close() //nolint:errcheck,gosec

	origCfg := cfg
	origToon := toonOut
	origNoAgent := dreamNoAgent
	cfg = c
	toonOut = true
	dreamNoAgent = true
	defer func() {
		cfg = origCfg
		toonOut = origToon
		dreamNoAgent = origNoAgent
	}()

	out, err := captureStdout(t, func() error {
		return dreamCmd.RunE(dreamCmd, []string{})
	})
	if err != nil {
		t.Fatalf("dreamCmd failed: %v", err)
	}

	if !strings.Contains(out, "dream_summary[") {
		t.Errorf("expected TOON format with dream_summary[...], got: %s", out)
	}
}
