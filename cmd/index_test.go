//nolint:revive // package comment not required
package cmd

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/rjfonseca/daneel/internal/config"
	"github.com/rjfonseca/daneel/internal/search"
)

func TestIndexCmd_NoArgs_IndexesEntireVault(t *testing.T) {
	tempVault := t.TempDir()

	_ = os.MkdirAll(filepath.Join(tempVault, "notes"), 0750)
	_ = os.MkdirAll(filepath.Join(tempVault, "sources"), 0750)

	note1 := filepath.Join(tempVault, "notes", "architecture.md")
	_ = os.WriteFile(note1, []byte("---\ntitle: Architecture\n---\n# Architecture\nEvent-driven microservices with Kafka.\n"), 0600)

	note2 := filepath.Join(tempVault, "sources", "reference.md")
	_ = os.WriteFile(note2, []byte("---\ntitle: Reference\n---\n# Reference\nPostgres relational database optimization.\n"), 0600)

	cfg = &config.Config{
		VaultPath: tempVault,
		DBPath:    filepath.Join(tempVault, ".daneel", "cache", "vault_index.db"),
		Vault:     config.VaultConfig{IndexedDirs: []string{"notes", "sources"}},
	}
	indexForce = false

	// Run index command without arguments
	err := indexCmd.RunE(indexCmd, []string{})
	if err != nil {
		t.Fatalf("indexCmd without args failed: %v", err)
	}

	// Verify database indexed both notes
	engine, err := search.Open(cfg.DBPath, cfg.VaultPath)
	if err != nil {
		t.Fatalf("failed to open search engine: %v", err)
	}
	defer engine.Close() //nolint:errcheck,gosec

	res1, err := engine.Search(t.Context(), "microservices", "all", 10)
	if err != nil || len(res1) == 0 {
		t.Errorf("expected to find 'microservices', got %d results, err: %v", len(res1), err)
	}

	res2, err := engine.Search(t.Context(), "Postgres", "all", 10)
	if err != nil || len(res2) == 0 {
		t.Errorf("expected to find 'Postgres', got %d results, err: %v", len(res2), err)
	}
}

func TestIndexCmd_SpecificDir(t *testing.T) {
	tempVault := t.TempDir()

	notesDir := filepath.Join(tempVault, "notes")
	_ = os.MkdirAll(notesDir, 0750)

	notePath := filepath.Join(notesDir, "security.md")
	_ = os.WriteFile(notePath, []byte("# Security\nOAuth2 and mTLS authentication.\n"), 0600)

	cfg = &config.Config{
		VaultPath: tempVault,
		DBPath:    filepath.Join(tempVault, ".daneel", "cache", "vault_index.db"),
		Vault:     config.VaultConfig{IndexedDirs: []string{"notes"}},
	}
	indexForce = false

	err := indexCmd.RunE(indexCmd, []string{notesDir})
	if err != nil {
		t.Fatalf("indexCmd with directory failed: %v", err)
	}

	engine, err := search.Open(cfg.DBPath, cfg.VaultPath)
	if err != nil {
		t.Fatalf("failed to open search engine: %v", err)
	}
	defer engine.Close() //nolint:errcheck,gosec

	res, err := engine.Search(t.Context(), "OAuth2", "all", 10)
	if err != nil || len(res) == 0 {
		t.Errorf("expected to find 'OAuth2', got %d results", len(res))
	}
}

func TestIndexCmd_SingleFile(t *testing.T) {
	tempVault := t.TempDir()

	notesDir := filepath.Join(tempVault, "notes")
	_ = os.MkdirAll(notesDir, 0750)

	filePath := filepath.Join(notesDir, "single.md")
	_ = os.WriteFile(filePath, []byte("# Single Note\nUniqueTokenSingleIndexing.\n"), 0600)

	cfg = &config.Config{
		VaultPath: tempVault,
		DBPath:    filepath.Join(tempVault, ".daneel", "cache", "vault_index.db"),
		Vault:     config.VaultConfig{IndexedDirs: []string{"notes"}},
	}

	err := indexCmd.RunE(indexCmd, []string{filePath})
	if err != nil {
		t.Fatalf("indexCmd with file failed: %v", err)
	}

	engine, err := search.Open(cfg.DBPath, cfg.VaultPath)
	if err != nil {
		t.Fatalf("failed to open search engine: %v", err)
	}
	defer engine.Close() //nolint:errcheck,gosec

	res, err := engine.Search(t.Context(), "UniqueTokenSingleIndexing", "all", 10)
	if err != nil || len(res) == 0 {
		t.Errorf("expected to find single note, got %d results", len(res))
	}
}

func TestIndexCmd_EmbedFlags(t *testing.T) {
	tests := []struct {
		name     string
		defValue string
	}{
		{"embed", "false"},
		{"scope", "all"},
		{"batch-size", "64"},
		{"embed-limit", "0"},
	}
	for _, tc := range tests {
		flag := indexCmd.Flags().Lookup(tc.name)
		if flag == nil {
			t.Fatalf("expected --%s flag to be registered on indexCmd", tc.name)
		}
		if flag.DefValue != tc.defValue {
			t.Errorf("flag --%s: expected default %q, got %q", tc.name, tc.defValue, flag.DefValue)
		}
	}

	tempVault := t.TempDir()
	notesDir := filepath.Join(tempVault, "notes")
	_ = os.MkdirAll(notesDir, 0750)
	_ = os.WriteFile(filepath.Join(notesDir, "chunked.md"), []byte("# Chunked Note\n## Section\nContent for chunking.\n"), 0600)

	cfg = &config.Config{
		VaultPath:  tempVault,
		DBPath:     filepath.Join(tempVault, ".daneel", "cache", "vault_index.db"),
		Vault:      config.VaultConfig{IndexedDirs: []string{"notes"}},
		Embeddings: config.EmbeddingsConfig{Enabled: false, Provider: "none"},
	}
	indexForce = false
	indexEmbed = true
	indexEmbedScope = "all"
	indexBatchSize = 64
	indexEmbedLimit = 0
	defer func() { indexEmbed = false }()

	if err := indexCmd.RunE(indexCmd, []string{}); err != nil {
		t.Fatalf("indexCmd --embed with disabled embeddings failed: %v", err)
	}
}

type fakeCmdEmbedder struct{}

func (f *fakeCmdEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i := range texts {
		out[i] = []float32{1, 0, 0, 0}
	}
	return out, nil
}

func (f *fakeCmdEmbedder) Model() string   { return "fake-test-model" }
func (f *fakeCmdEmbedder) Dimensions() int { return 4 }
func (f *fakeCmdEmbedder) Close() error    { return nil }

func TestIndexCmd_EmbeddingsConfigAndFlagPrecedence(t *testing.T) {
	tests := []struct {
		name         string
		embCfg       config.EmbeddingsConfig
		setFlag      bool
		flagVal      string
		wantEmbedded bool
	}{
		{
			name:         "config enabled without flag runs embeddings",
			embCfg:       config.EmbeddingsConfig{Enabled: true, Provider: "rembed", Model: "fake"},
			setFlag:      false,
			wantEmbedded: true,
		},
		{
			name:         "config disabled without flag skips embeddings",
			embCfg:       config.EmbeddingsConfig{Enabled: false, Provider: "rembed", Model: "fake"},
			setFlag:      false,
			wantEmbedded: false,
		},
		{
			name:         "provider none without flag skips embeddings",
			embCfg:       config.EmbeddingsConfig{Enabled: true, Provider: "none"},
			setFlag:      false,
			wantEmbedded: false,
		},
		{
			name:         "explicit --embed=false overrides config enabled",
			embCfg:       config.EmbeddingsConfig{Enabled: true, Provider: "rembed", Model: "fake"},
			setFlag:      true,
			flagVal:      "false",
			wantEmbedded: false,
		},
		{
			name:         "explicit --embed=true overrides config disabled",
			embCfg:       config.EmbeddingsConfig{Enabled: false, Provider: "rembed", Model: "fake"},
			setFlag:      true,
			flagVal:      "true",
			wantEmbedded: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
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
			_ = os.WriteFile(filepath.Join(notesDir, "doc.md"), []byte("# Doc\n## Section\nVector indexing test content.\n"), 0600)

			cfg = &config.Config{
				VaultPath:  tempVault,
				DBPath:     filepath.Join(tempVault, ".daneel", "cache", "vault_index.db"),
				Vault:      config.VaultConfig{IndexedDirs: []string{"notes"}},
				Embeddings: tc.embCfg,
			}

			indexForce = false
			indexEmbed = false
			indexEmbedScope = "all"
			indexBatchSize = 64
			indexEmbedLimit = 0
			embedFlag := indexCmd.Flags().Lookup("embed")
			embedFlag.Changed = false
			defer func() {
				indexEmbed = false
				embedFlag.Changed = false
			}()

			if tc.setFlag {
				if err := indexCmd.Flags().Set("embed", tc.flagVal); err != nil {
					t.Fatalf("failed to set --embed=%s: %v", tc.flagVal, err)
				}
			}

			if err := indexCmd.RunE(indexCmd, []string{}); err != nil {
				t.Fatalf("indexCmd.RunE failed: %v", err)
			}

			if called != tc.wantEmbedded {
				t.Fatalf("embedder called = %v, want %v", called, tc.wantEmbedded)
			}
			if tc.wantEmbedded && !receivedCfg.Enabled {
				t.Fatalf("expected effective EmbeddingsConfig.Enabled=true, got false")
			}

			engine, err := search.Open(cfg.DBPath, cfg.VaultPath)
			if err != nil {
				t.Fatalf("search.Open failed: %v", err)
			}
			defer func() { _ = engine.Close() }()

			if got := engine.HasAnyEmbeddings(t.Context()); got != tc.wantEmbedded {
				t.Fatalf("HasAnyEmbeddings() = %v, want %v", got, tc.wantEmbedded)
			}
		})
	}
}
