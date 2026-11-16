//nolint:revive // package comment not required
package search

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
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

func TestIndexVaultIfStale_And_ShouldSync(t *testing.T) {
	tempDir := t.TempDir()
	ctx := context.Background()

	notesDir := filepath.Join(tempDir, "notes")
	_ = os.MkdirAll(notesDir, 0750)
	_ = os.WriteFile(filepath.Join(notesDir, "consensus.md"), []byte("# Raft Consensus\nLeader election and log replication.\n"), 0600)

	dbPath := filepath.Join(tempDir, "vault_index.db")
	engine, err := Open(dbPath, tempDir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = engine.Close() }()

	// Never synced -> ShouldSync must be true
	should, _, err := engine.ShouldSync(ctx, 2*time.Hour+30*time.Minute)
	if err != nil {
		t.Fatalf("ShouldSync failed: %v", err)
	}
	if !should {
		t.Fatal("expected ShouldSync=true on fresh database")
	}

	// First IndexVaultIfStale should run indexing
	idx, upd, skipped, err := engine.IndexVaultIfStale(ctx, []string{"notes"}, 2*time.Hour+30*time.Minute)
	if err != nil {
		t.Fatalf("IndexVaultIfStale failed: %v", err)
	}
	if skipped || idx != 1 || upd != 0 {
		t.Fatalf("expected indexed=1, updated=0, skipped=false; got indexed=%d updated=%d skipped=%v", idx, upd, skipped)
	}

	// Immediately calling ShouldSync with 2h30m should return false
	should, age, err := engine.ShouldSync(ctx, 2*time.Hour+30*time.Minute)
	if err != nil {
		t.Fatalf("ShouldSync failed: %v", err)
	}
	if should {
		t.Fatalf("expected ShouldSync=false immediately after sync, got true (age=%v)", age)
	}

	// Add a second note; IndexVaultIfStale with 2h30m should skip
	_ = os.WriteFile(filepath.Join(notesDir, "paxos.md"), []byte("# Paxos\nMulti-decree consensus protocol.\n"), 0600)
	idx, _, skipped, err = engine.IndexVaultIfStale(ctx, []string{"notes"}, 2*time.Hour+30*time.Minute)
	if err != nil {
		t.Fatalf("IndexVaultIfStale failed: %v", err)
	}
	if !skipped || idx != 0 {
		t.Fatalf("expected skipped=true and idx=0, got skipped=%v idx=%d", skipped, idx)
	}

	// With minAge=0, IndexVaultIfStale should run and pick up the second note
	idx, _, skipped, err = engine.IndexVaultIfStale(ctx, []string{"notes"}, 0)
	if err != nil {
		t.Fatalf("IndexVaultIfStale(0) failed: %v", err)
	}
	if skipped || idx != 1 {
		t.Fatalf("expected skipped=false and idx=1, got skipped=%v idx=%d", skipped, idx)
	}
}

func TestSourcesDBIsolation(t *testing.T) {
	tempDir := t.TempDir()
	ctx := context.Background()

	notesDir := filepath.Join(tempDir, "notes")
	sourcesDir := filepath.Join(tempDir, "sources")
	_ = os.MkdirAll(notesDir, 0750)
	_ = os.MkdirAll(sourcesDir, 0750)

	_ = os.WriteFile(filepath.Join(notesDir, "gateway.md"), []byte("# API Gateway\nRouting and rate limiting for distributed services.\n"), 0600)
	_ = os.WriteFile(filepath.Join(sourcesDir, "external-rfc.md"), []byte("# External RFC\nDistributed services specification from upstream.\n"), 0600)

	dbPath := filepath.Join(tempDir, "vault_index.db")
	engine, err := Open(dbPath, tempDir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	if _, _, err := engine.IndexVault(ctx, []string{"notes", "sources"}, true); err != nil {
		t.Fatalf("IndexVault failed: %v", err)
	}

	// Verify sources_index.db was created in the same directory
	expectedSourcesPath := filepath.Join(tempDir, "sources_index.db")
	if engine.SourcesDBPath() != expectedSourcesPath {
		t.Fatalf("expected SourcesDBPath=%s, got %s", expectedSourcesPath, engine.SourcesDBPath())
	}
	if _, err := os.Stat(expectedSourcesPath); err != nil {
		t.Fatalf("expected sources_index.db to exist on disk: %v", err)
	}

	// Verify primary DB contains only curated note, not sources
	var mainCount, sourcesInMain int
	_ = engine.DB().QueryRow("SELECT COUNT(*) FROM notes_meta").Scan(&mainCount)
	_ = engine.DB().QueryRow("SELECT COUNT(*) FROM notes_meta WHERE scope = 'sources'").Scan(&sourcesInMain)
	if mainCount != 1 || sourcesInMain != 0 {
		t.Fatalf("expected mainCount=1 and sourcesInMain=0, got mainCount=%d sourcesInMain=%d", mainCount, sourcesInMain)
	}
	_ = engine.Close()

	// Reopen fresh engine to verify lazy opening of sources_index.db
	fresh, err := Open(dbPath, tempDir)
	if err != nil {
		t.Fatalf("reopen failed: %v", err)
	}
	defer func() { _ = fresh.Close() }()

	// 1. scope=tech must NOT open sources_index.db
	techRes, err := fresh.SearchFaceted(ctx, SearchOptions{Query: "distributed", Scope: "tech", Limit: 10})
	if err != nil {
		t.Fatalf("SearchFaceted(tech) failed: %v", err)
	}
	if len(techRes) != 1 || techRes[0].RelPath != "notes/gateway.md" {
		t.Fatalf("unexpected tech results: %+v", techRes)
	}
	if fresh.sourcesDB != nil {
		t.Fatal("expected sourcesDB to remain unopened for scope=tech")
	}

	// 2. scope=all with limit=1 satisfied by primary DB must NOT open sources_index.db
	limitedRes, err := fresh.SearchFaceted(ctx, SearchOptions{Query: "distributed", Scope: "all", Limit: 1})
	if err != nil {
		t.Fatalf("SearchFaceted(all, limit=1) failed: %v", err)
	}
	if len(limitedRes) != 1 || limitedRes[0].RelPath != "notes/gateway.md" {
		t.Fatalf("unexpected limited all results: %+v", limitedRes)
	}
	if fresh.sourcesDB != nil {
		t.Fatal("expected sourcesDB to remain unopened when primary DB satisfies limit")
	}

	// 3. scope=all with limit=5 falls back to sources_index.db and prioritizes curated notes first
	allRes, err := fresh.SearchFaceted(ctx, SearchOptions{Query: "distributed", Scope: "all", Limit: 5})
	if err != nil {
		t.Fatalf("SearchFaceted(all, limit=5) failed: %v", err)
	}
	if len(allRes) != 2 {
		t.Fatalf("expected 2 results for scope=all, got %d", len(allRes))
	}
	if allRes[0].RelPath != "notes/gateway.md" || allRes[1].RelPath != "sources/external-rfc.md" {
		t.Fatalf("unexpected ordering for scope=all: %+v", allRes)
	}

	// 4. scope=sources queries only sourcesDB
	srcRes, err := fresh.SearchFaceted(ctx, SearchOptions{Query: "distributed", Scope: "sources", Limit: 10})
	if err != nil {
		t.Fatalf("SearchFaceted(sources) failed: %v", err)
	}
	if len(srcRes) != 1 || srcRes[0].RelPath != "sources/external-rfc.md" {
		t.Fatalf("unexpected sources results: %+v", srcRes)
	}
}

func TestSourcesMigrationHygiene(t *testing.T) {
	tempDir := t.TempDir()
	ctx := context.Background()

	notesDir := filepath.Join(tempDir, "notes")
	_ = os.MkdirAll(notesDir, 0750)
	_ = os.WriteFile(filepath.Join(notesDir, "curated.md"), []byte("# Curated Note\nClean architecture.\n"), 0600)

	dbPath := filepath.Join(tempDir, "vault_index.db")
	engine, err := Open(dbPath, tempDir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = engine.Close() }()

	// Simulate legacy rows with scope='sources' in primary DB
	_, err = engine.DB().Exec(`
		INSERT INTO notes_meta(rel_path, title, category, scope, note_type, description, tags, headings, aliases, mtime)
		VALUES ('sources/legacy.md', 'Legacy Source', 'sources', 'sources', 'doc', 'Legacy', '', '', '', 1.0);
		INSERT INTO notes_fts(rel_path, title, category, scope, note_type, description, tags, headings, content)
		VALUES ('sources/legacy.md', 'Legacy Source', 'sources', 'sources', 'doc', 'Legacy', '', '', 'Legacy content');
		INSERT INTO note_links(source_rel_path, target_rel_path, link_text)
		VALUES ('sources/legacy.md', 'notes/curated.md', 'Curated');
	`)
	if err != nil {
		t.Fatalf("failed to seed legacy rows: %v", err)
	}

	if _, _, err := engine.IndexVault(ctx, []string{"notes"}, false); err != nil {
		t.Fatalf("IndexVault failed: %v", err)
	}

	var metaCnt, ftsCnt, linkCnt int
	_ = engine.DB().QueryRow("SELECT COUNT(*) FROM notes_meta WHERE scope = 'sources'").Scan(&metaCnt)
	_ = engine.DB().QueryRow("SELECT COUNT(*) FROM notes_fts WHERE rel_path = 'sources/legacy.md'").Scan(&ftsCnt)
	_ = engine.DB().QueryRow("SELECT COUNT(*) FROM note_links WHERE source_rel_path = 'sources/legacy.md'").Scan(&linkCnt)
	if metaCnt != 0 || ftsCnt != 0 || linkCnt != 0 {
		t.Fatalf("expected legacy sources rows to be purged, got meta=%d fts=%d links=%d", metaCnt, ftsCnt, linkCnt)
	}

	var migrated string
	if err := engine.DB().QueryRow("SELECT value FROM index_metadata WHERE key = 'sources_migrated'").Scan(&migrated); err != nil || migrated != "1" {
		t.Fatalf("expected sources_migrated='1', got '%s' (err=%v)", migrated, err)
	}
}

func TestOpen_InMemoryDB(t *testing.T) {
	tempDir := t.TempDir()
	engine, err := Open(":memory:", tempDir)
	if err != nil {
		t.Fatalf("Open(:memory:) failed: %v", err)
	}
	defer func() { _ = engine.Close() }()

	if engine.SourcesDBPath() != ":memory:" {
		t.Fatalf("expected SourcesDBPath=':memory:', got %q", engine.SourcesDBPath())
	}
}
