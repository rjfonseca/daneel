//nolint:revive // package comment not required
package search

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestSearchEngine(t *testing.T) {
	tempDir := t.TempDir()
	ctx := context.Background()

	notesDir := filepath.Join(tempDir, "notes")
	_ = os.MkdirAll(notesDir, 0750)

	note1 := filepath.Join(notesDir, "redis.md")
	content1 := `---
type: tecnologia
description: Guia de arquitetura e cache com Redis.
tags:
  - database
---
# Redis In-Memory Cache

Redis é um banco de dados em memória chave-valor de altíssimo desempenho.
`
	_ = os.WriteFile(note1, []byte(content1), 0600)

	dbPath := filepath.Join(tempDir, "cache.db")
	engine, err := Open(dbPath, tempDir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer engine.Close() //nolint:errcheck,gosec // fully read or safe to ignore

	indexed, updated, err := engine.IndexVault(ctx, []string{"notes"}, true)
	if err != nil {
		t.Fatalf("IndexVault failed: %v", err)
	}
	if indexed != 1 || updated != 0 {
		t.Errorf("expected 1 indexed note, got indexed=%d, updated=%d", indexed, updated)
	}

	results, err := engine.Search(ctx, "Redis", "all", 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 search result, got %d", len(results))
	}
	if results[0].Title != "Redis In-Memory Cache" {
		t.Errorf("expected title 'Redis In-Memory Cache', got '%s'", results[0].Title)
	}
}

func TestSearchEngine_NilIndexedDirs_AndScopedPruning(t *testing.T) {
	tempDir := t.TempDir()
	ctx := context.Background()

	notesDir := filepath.Join(tempDir, "notes")
	sourcesDir := filepath.Join(tempDir, "sources")
	_ = os.MkdirAll(notesDir, 0750)
	_ = os.MkdirAll(sourcesDir, 0750)

	note1 := filepath.Join(notesDir, "note1.md")
	note2 := filepath.Join(sourcesDir, "note2.md")
	_ = os.WriteFile(note1, []byte("# Note 1\nAlpha content in notes.\n"), 0600)
	_ = os.WriteFile(note2, []byte("# Note 2\nBeta content in sources.\n"), 0600)

	dbPath := filepath.Join(tempDir, "cache.db")
	engine, err := Open(dbPath, tempDir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer engine.Close() //nolint:errcheck,gosec

	// 1. Index entire vault passing nil indexedDirs
	indexed, updated, err := engine.IndexVault(ctx, nil, true)
	if err != nil {
		t.Fatalf("IndexVault with nil failed: %v", err)
	}
	if indexed != 2 {
		t.Errorf("expected 2 indexed notes, got %d (updated: %d)", indexed, updated)
	}

	// 2. Scoped index of only "notes" directory should NOT prune note2 in "sources"
	_, _, err = engine.IndexVault(ctx, []string{"notes"}, false)
	if err != nil {
		t.Fatalf("IndexVault notes failed: %v", err)
	}

	resBeta, err := engine.Search(ctx, "Beta", "all", 10)
	if err != nil || len(resBeta) != 1 {
		t.Fatalf("expected note2 to survive scoped indexing, got %d results, err: %v", len(resBeta), err)
	}

	// 3. Delete note1 in "notes", run scoped index on "notes": note1 should be pruned, note2 preserved
	_ = os.Remove(note1)
	_, _, err = engine.IndexVault(ctx, []string{"notes"}, false)
	if err != nil {
		t.Fatalf("IndexVault notes after delete failed: %v", err)
	}

	resAlpha, err := engine.Search(ctx, "Alpha", "all", 10)
	if err != nil || len(resAlpha) != 0 {
		t.Errorf("expected note1 to be pruned, got %d results", len(resAlpha))
	}

	resBetaAfter, err := engine.Search(ctx, "Beta", "all", 10)
	if err != nil || len(resBetaAfter) != 1 {
		t.Errorf("expected note2 to remain intact, got %d results", len(resBetaAfter))
	}
}

func TestIndexVault_MissingVaultRoot_SafetyGuard(t *testing.T) {
	tempDir := t.TempDir()
	ctx := context.Background()

	// 1. Setup valid vault with 1 note
	vaultDir := filepath.Join(tempDir, "vault")
	notesDir := filepath.Join(vaultDir, "notes")
	_ = os.MkdirAll(notesDir, 0750)
	notePath := filepath.Join(notesDir, "system.md")
	_ = os.WriteFile(notePath, []byte("# Identity System\nIMP architecture note.\n"), 0600)

	dbPath := filepath.Join(tempDir, "index.db")
	engine, err := Open(dbPath, vaultDir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = engine.Close() }()

	indexed, _, err := engine.IndexVault(ctx, []string{"notes"}, false)
	if err != nil || indexed != 1 {
		t.Fatalf("IndexVault failed: %v, indexed: %d", err, indexed)
	}

	// 2. Point engine to a non-existent vault directory
	engine.vaultRoot = filepath.Join(tempDir, "non_existent_vault")

	// 3. Attempting to index non-existent vault must error out and NEVER wipe the existing index
	_, _, err = engine.IndexVault(ctx, []string{"notes"}, false)
	if err == nil {
		t.Fatal("expected error when vaultRoot does not exist, got nil")
	}

	// 4. Verify existing notes remain intact in the index
	res, err := engine.Search(ctx, "Identity", "all", 10)
	if err != nil || len(res) != 1 {
		t.Fatalf("expected existing note to be preserved, got %d results, err: %v", len(res), err)
	}
}

func BenchmarkSearch(b *testing.B) {
	tempDir := b.TempDir()
	ctx := context.Background()

	notesDir := filepath.Join(tempDir, "notes")
	_ = os.MkdirAll(notesDir, 0750)

	for i := 0; i < 100; i++ {
		notePath := filepath.Join(notesDir, fmt.Sprintf("note_%d.md", i))
		content := fmt.Sprintf("# Note %d\nThis is a benchmark note about microservices, databases, and golang architecture %d.\n", i, i)
		_ = os.WriteFile(notePath, []byte(content), 0600)
	}

	dbPath := filepath.Join(tempDir, "bench.db")
	engine, err := Open(dbPath, tempDir)
	if err != nil {
		b.Fatalf("failed to open engine: %v", err)
	}
	defer engine.Close() //nolint:errcheck,gosec // fully read or safe to ignore

	_, _, _ = engine.IndexVault(ctx, []string{"notes"}, true)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = engine.Search(ctx, "microservices AND golang", "all", 10)
	}
}
