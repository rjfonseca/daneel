//nolint:revive // package comment not required
package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rjfonseca/daneel/internal/config"
	"github.com/rjfonseca/daneel/internal/search"
)

func resetSearchFlagsForTest() {
	searchScope = "all"
	searchType = ""
	searchTag = ""
	searchLimit = 10
	searchReindex = false
	searchNoSync = false
	searchForceSync = false
	searchMinSyncAge = defaultMinimumSyncAge
	jsonOut = false
	toonOut = false
	verbose = false
}

func TestSearchCmd_NoSyncAndForceSyncConflict(t *testing.T) {
	resetSearchFlagsForTest()
	defer resetSearchFlagsForTest()

	tempVault := t.TempDir()
	cfg = &config.Config{
		VaultPath: tempVault,
		DBPath:    filepath.Join(tempVault, ".daneel", "cache", "vault_index.db"),
		Vault:     config.VaultConfig{IndexedDirs: []string{"notes"}},
	}

	searchNoSync = true
	searchForceSync = true

	err := searchCmd.RunE(searchCmd, []string{"architecture"})
	if err == nil {
		t.Fatal("expected error when both --no-sync and --force-sync are set, got nil")
	}
	if !strings.Contains(err.Error(), "--no-sync") || !strings.Contains(err.Error(), "--force-sync") {
		t.Fatalf("expected explicit conflict error mentioning flags, got: %v", err)
	}
}

func TestSearchCmd_DefaultMinSyncAgeFlag(t *testing.T) {
	flag := searchCmd.Flags().Lookup("minimum-sync-age")
	if flag == nil {
		t.Fatal("expected --minimum-sync-age flag to be registered")
	}
	if flag.DefValue != (2*time.Hour + 30*time.Minute).String() {
		t.Fatalf("expected default --minimum-sync-age=%s, got %s", (2*time.Hour + 30*time.Minute).String(), flag.DefValue)
	}
}

func TestSearchCmd_SyncModes(t *testing.T) {
	resetSearchFlagsForTest()
	defer resetSearchFlagsForTest()

	tempVault := t.TempDir()
	notesDir := filepath.Join(tempVault, "notes")
	_ = os.MkdirAll(notesDir, 0750)
	_ = os.WriteFile(filepath.Join(notesDir, "alpha.md"), []byte("# Alpha Note\nUniqueTokenAlphaSync.\n"), 0600)

	dbPath := filepath.Join(tempVault, ".daneel", "cache", "vault_index.db")
	cfg = &config.Config{
		VaultPath: tempVault,
		DBPath:    dbPath,
		Vault:     config.VaultConfig{IndexedDirs: []string{"notes"}},
	}

	// 1. --no-sync on fresh DB must NOT index alpha.md
	searchNoSync = true
	jsonOut = true
	out, err := captureStdout(t, func() error {
		return searchCmd.RunE(searchCmd, []string{"UniqueTokenAlphaSync"})
	})
	if err != nil {
		t.Fatalf("searchCmd --no-sync failed: %v", err)
	}
	if strings.Contains(out, "alpha.md") {
		t.Fatalf("expected --no-sync to skip indexing on fresh DB, got output: %s", out)
	}

	// 2. Default smart sync on never-synced DB must index alpha.md
	resetSearchFlagsForTest()
	jsonOut = true
	out, err = captureStdout(t, func() error {
		return searchCmd.RunE(searchCmd, []string{"UniqueTokenAlphaSync"})
	})
	if err != nil {
		t.Fatalf("searchCmd default smart sync failed: %v", err)
	}
	if !strings.Contains(out, "alpha.md") {
		t.Fatalf("expected initial smart sync to index alpha.md, got output: %s", out)
	}

	// 3. Add beta.md; default smart sync within 2h30m must skip indexing
	_ = os.WriteFile(filepath.Join(notesDir, "beta.md"), []byte("# Beta Note\nUniqueTokenBetaSync.\n"), 0600)
	resetSearchFlagsForTest()
	jsonOut = true
	out, err = captureStdout(t, func() error {
		return searchCmd.RunE(searchCmd, []string{"UniqueTokenBetaSync"})
	})
	if err != nil {
		t.Fatalf("searchCmd stale-skip failed: %v", err)
	}
	if strings.Contains(out, "beta.md") {
		t.Fatalf("expected smart sync within minimum-sync-age to skip beta.md, got output: %s", out)
	}

	// 4. --force-sync must index beta.md immediately
	resetSearchFlagsForTest()
	searchForceSync = true
	jsonOut = true
	out, err = captureStdout(t, func() error {
		return searchCmd.RunE(searchCmd, []string{"UniqueTokenBetaSync"})
	})
	if err != nil {
		t.Fatalf("searchCmd --force-sync failed: %v", err)
	}
	if !strings.Contains(out, "beta.md") {
		t.Fatalf("expected --force-sync to index beta.md, got output: %s", out)
	}

	// Sanity check DB state
	engine, err := search.Open(dbPath, tempVault)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = engine.Close() }()
	res, err := engine.Search(t.Context(), "UniqueTokenBetaSync", "all", 10)
	if err != nil || len(res) != 1 {
		t.Fatalf("expected beta.md in index, got %d results (err=%v)", len(res), err)
	}
}
