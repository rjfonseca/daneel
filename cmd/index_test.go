//nolint:revive // package comment not required
package cmd

import (
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
