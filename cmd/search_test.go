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
	searchSemantic = false
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

func TestSearchCmd_SemanticFlagAndFallback(t *testing.T) {
	resetSearchFlagsForTest()
	defer resetSearchFlagsForTest()

	flag := searchCmd.Flags().Lookup("semantic")
	if flag == nil {
		t.Fatal("expected --semantic flag to be registered")
	}
	if flag.DefValue != "false" {
		t.Fatalf("expected default --semantic=false, got %s", flag.DefValue)
	}

	tempVault := t.TempDir()
	notesDir := filepath.Join(tempVault, "notes")
	_ = os.MkdirAll(notesDir, 0750)
	_ = os.WriteFile(filepath.Join(notesDir, "hybrid.md"), []byte("# Hybrid Note\n## Storage\nUniqueTokenHybridFallback.\n"), 0600)

	cfg = &config.Config{
		VaultPath:  tempVault,
		DBPath:     filepath.Join(tempVault, ".daneel", "cache", "vault_index.db"),
		Vault:      config.VaultConfig{IndexedDirs: []string{"notes"}},
		Embeddings: config.EmbeddingsConfig{Enabled: false, Provider: "none"},
	}

	searchSemantic = true
	searchForceSync = true
	jsonOut = true
	out, err := captureStdout(t, func() error {
		return searchCmd.RunE(searchCmd, []string{"UniqueTokenHybridFallback"})
	})
	if err != nil {
		t.Fatalf("searchCmd --semantic fallback failed: %v", err)
	}
	if !strings.Contains(out, "hybrid.md") {
		t.Fatalf("expected hybrid.md via chunks_fts fallback, got: %s", out)
	}
}

func TestSearchCmd_SemanticConfigAndFlagPrecedence(t *testing.T) {
	tests := []struct {
		name         string
		embCfg       config.EmbeddingsConfig
		setFlag      bool
		flagVal      string
		wantSemantic bool
	}{
		{
			name:         "config enabled without flag runs semantic search",
			embCfg:       config.EmbeddingsConfig{Enabled: true, Provider: "rembed", Model: "fake"},
			setFlag:      false,
			wantSemantic: true,
		},
		{
			name:         "config disabled without flag uses lexical search",
			embCfg:       config.EmbeddingsConfig{Enabled: false, Provider: "rembed", Model: "fake"},
			setFlag:      false,
			wantSemantic: false,
		},
		{
			name:         "provider none without flag uses lexical search",
			embCfg:       config.EmbeddingsConfig{Enabled: true, Provider: "none"},
			setFlag:      false,
			wantSemantic: false,
		},
		{
			name:         "explicit --semantic=false overrides config enabled",
			embCfg:       config.EmbeddingsConfig{Enabled: true, Provider: "rembed", Model: "fake"},
			setFlag:      true,
			flagVal:      "false",
			wantSemantic: false,
		},
		{
			name:         "explicit --semantic=true overrides config disabled",
			embCfg:       config.EmbeddingsConfig{Enabled: false, Provider: "rembed", Model: "fake"},
			setFlag:      true,
			flagVal:      "true",
			wantSemantic: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resetSearchFlagsForTest()
			defer resetSearchFlagsForTest()

			semanticFlag := searchCmd.Flags().Lookup("semantic")
			semanticFlag.Changed = false
			defer func() { semanticFlag.Changed = false }()

			origFactory := newEmbedderFromConfig
			defer func() { newEmbedderFromConfig = origFactory }()

			called := false
			var receivedCfg config.EmbeddingsConfig
			newEmbedderFromConfig = func(c config.EmbeddingsConfig) (search.Embedder, error) {
				called = true
				receivedCfg = c
				return &fakeCmdEmbedder{}, nil
			}

			tempVault := t.TempDir()
			notesDir := filepath.Join(tempVault, "notes")
			_ = os.MkdirAll(notesDir, 0750)
			_ = os.WriteFile(filepath.Join(notesDir, "hybrid.md"), []byte("# Hybrid Note\n## Storage\nUniqueTokenSemanticPrecedence.\n"), 0600)

			cfg = &config.Config{
				VaultPath:  tempVault,
				DBPath:     filepath.Join(tempVault, ".daneel", "cache", "vault_index.db"),
				Vault:      config.VaultConfig{IndexedDirs: []string{"notes"}},
				Embeddings: tc.embCfg,
			}

			searchForceSync = true
			jsonOut = true
			if tc.setFlag {
				if err := searchCmd.Flags().Set("semantic", tc.flagVal); err != nil {
					t.Fatalf("failed to set --semantic=%s: %v", tc.flagVal, err)
				}
			}

			out, err := captureStdout(t, func() error {
				return searchCmd.RunE(searchCmd, []string{"UniqueTokenSemanticPrecedence"})
			})
			if err != nil {
				t.Fatalf("searchCmd.RunE failed: %v", err)
			}
			if !strings.Contains(out, "hybrid.md") {
				t.Fatalf("expected hybrid.md in search results, got: %s", out)
			}
			if called != tc.wantSemantic {
				t.Fatalf("embedder called = %v, want %v", called, tc.wantSemantic)
			}
			if tc.wantSemantic && !receivedCfg.Enabled {
				t.Fatalf("expected effective EmbeddingsConfig.Enabled=true, got false")
			}
		})
	}
}
