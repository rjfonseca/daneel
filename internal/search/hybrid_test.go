package search

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type mockEmbedder struct {
	model string
	dim   int
	calls int
	fn    func(text string) []float32
}

func (m *mockEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	m.calls++
	out := make([][]float32, len(texts))
	for i, t := range texts {
		out[i] = normalizeL2(m.fn(t))
	}
	return out, nil
}

func (m *mockEmbedder) Model() string   { return m.model }
func (m *mockEmbedder) Dimensions() int { return m.dim }
func (m *mockEmbedder) Close() error    { return nil }

func newSemanticMockEmbedder() *mockEmbedder {
	// 4D orthogonal semantic space:
	// dim 0: consensus / raft / replication
	// dim 1: storage / sqlite / fts5
	// dim 2: observability / tracing / metrics
	// dim 3: general / fallback
	return &mockEmbedder{
		model: "mock-minilm-v1",
		dim:   4,
		fn: func(text string) []float32 {
			lower := strings.ToLower(text)
			vec := []float32{0.01, 0.01, 0.01, 0.01}
			if strings.Contains(lower, "consensus") || strings.Contains(lower, "raft") || strings.Contains(lower, "leader") {
				vec[0] += 1.0
			}
			if strings.Contains(lower, "sqlite") || strings.Contains(lower, "storage") || strings.Contains(lower, "fts5") {
				vec[1] += 1.0
			}
			if strings.Contains(lower, "tracing") || strings.Contains(lower, "opentelemetry") || strings.Contains(lower, "metrics") {
				vec[2] += 1.0
			}
			return vec
		},
	}
}

func writeTestNote(t *testing.T, vaultRoot, relPath, content string) {
	t.Helper()
	abs := filepath.Join(vaultRoot, relPath)
	if err := os.MkdirAll(filepath.Dir(abs), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestNormalizeL2AndVectorRoundTrip(t *testing.T) {
	v := normalizeL2([]float32{3, 4})
	if math.Abs(float64(v[0])-0.6) > 1e-5 || math.Abs(float64(v[1])-0.8) > 1e-5 {
		t.Fatalf("unexpected normalized vector: %v", v)
	}
	if dp := dotProduct(v, v); math.Abs(dp-1.0) > 1e-5 {
		t.Fatalf("expected unit norm dot product 1.0, got %f", dp)
	}

	blob := encodeVector(v)
	decoded := decodeVector(blob)
	if len(decoded) != 2 || math.Abs(float64(decoded[0]-v[0])) > 1e-6 || math.Abs(float64(decoded[1]-v[1])) > 1e-6 {
		t.Fatalf("vector roundtrip mismatch: got %v, want %v", decoded, v)
	}
}

func TestEmbedPendingChunks_ResumableAndPreservesUnchangedHashes(t *testing.T) {
	vaultRoot := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "vault_index.db")

	writeTestNote(t, vaultRoot, "notes/arquitetura/raft.md", `---
type: arquitetura
description: Consensus notes
---
# Raft Notes

## Leader Election
Raft uses leader election and heartbeats.

## Log Replication
Followers append log entries from the leader.
`)

	engine, err := Open(dbPath, vaultRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close() //nolint:errcheck

	ctx := context.Background()
	if _, _, err := engine.IndexVault(ctx, []string{"notes"}, false); err != nil {
		t.Fatal(err)
	}

	emb := newSemanticMockEmbedder()

	// Resumable batch 1: limit=1 embeds only 1 chunk
	done1, err := engine.EmbedPendingChunks(ctx, emb, "all", 1, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if done1 != 1 {
		t.Fatalf("expected 1 embedded chunk in first pass, got %d", done1)
	}

	// Resumable batch 2: embeds remaining 1 chunk
	done2, err := engine.EmbedPendingChunks(ctx, emb, "all", 64, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if done2 != 1 {
		t.Fatalf("expected 1 remaining chunk in second pass, got %d", done2)
	}

	// Running again should be a no-op
	done3, err := engine.EmbedPendingChunks(ctx, emb, "all", 64, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if done3 != 0 {
		t.Fatalf("expected 0 pending chunks, got %d", done3)
	}

	// Update ONLY the second section and re-index; first section must keep its embedding!
	time.Sleep(15 * time.Millisecond)
	writeTestNote(t, vaultRoot, "notes/arquitetura/raft.md", `---
type: arquitetura
description: Consensus notes
---
# Raft Notes

## Leader Election
Raft uses leader election and heartbeats.

## Log Replication
Followers append log entries and commit snapshots.
`)
	if err := engine.IndexNote(ctx, "notes/arquitetura/raft.md"); err != nil {
		t.Fatal(err)
	}

	var withEmb, withoutEmb int
	if err := engine.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM chunks WHERE rel_path = 'notes/arquitetura/raft.md' AND embedding IS NOT NULL`).Scan(&withEmb); err != nil {
		t.Fatal(err)
	}
	if err := engine.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM chunks WHERE rel_path = 'notes/arquitetura/raft.md' AND embedding IS NULL`).Scan(&withoutEmb); err != nil {
		t.Fatal(err)
	}
	if withEmb != 1 || withoutEmb != 1 {
		t.Fatalf("expected 1 preserved embedding and 1 invalidated embedding, got with=%d without=%d", withEmb, withoutEmb)
	}
}

func TestSearchHybrid_RRFFusionAndGracefulFallback(t *testing.T) {
	vaultRoot := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "vault_index.db")

	// Note 1 (tech): talks about "raft" and "leader" (no word "consensus")
	writeTestNote(t, vaultRoot, "notes/arquitetura/raft.md", `---
type: arquitetura
description: State machine replication
---
# Raft Protocol

## Leader Election
Nodes elect a leader using randomized timeouts.
`)

	// Note 2 (tech): talks about SQLite FTS5
	writeTestNote(t, vaultRoot, "notes/dados/sqlite.md", `---
type: dados
description: Embedded SQL engine
---
# SQLite Storage

## FTS5 Engine
Uses porter unicode61 tokenizer for fast lexical search.
`)

	// Note 3 (sources): also mentions consensus
	writeTestNote(t, vaultRoot, "sources/paper-consensus.md", `---
type: source
description: External paper
---
# External Consensus Paper

## Summary
A paper about consensus protocols.
`)

	engine, err := Open(dbPath, vaultRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close() //nolint:errcheck

	ctx := context.Background()
	if _, _, err := engine.IndexVault(ctx, []string{"notes", "sources"}, false); err != nil {
		t.Fatal(err)
	}

	emb := newSemanticMockEmbedder()
	if _, err := engine.EmbedPendingChunks(ctx, emb, "all", 64, 0, nil); err != nil {
		t.Fatal(err)
	}

	// Query "consensus":
	// - sources/paper-consensus.md matches lexically AND vectorially, but has scope=sources (priority 4)
	// - notes/arquitetura/raft.md matches vectorially (raft/leader -> consensus dim) and has scope=tech (priority 0)
	results, err := engine.SearchHybrid(ctx, SearchOptions{
		Query: "consensus",
		Scope: "all",
		Limit: 10,
	}, emb)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) < 2 {
		t.Fatalf("expected at least 2 hybrid results, got %d", len(results))
	}
	if results[0].Scope != "tech" || results[0].RelPath != "notes/arquitetura/raft.md" {
		t.Fatalf("expected tech note first due to scope priority + vector match, got %+v", results[0])
	}
	if results[0].ChunkHeading == "" {
		t.Errorf("expected ChunkHeading to be populated, got empty")
	}

	// Graceful fallback with nil embedder (lexical chunks_fts only)
	lexResults, err := engine.SearchHybrid(ctx, SearchOptions{
		Query: "SQLite",
		Scope: "tech",
		Limit: 5,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(lexResults) != 1 || lexResults[0].RelPath != "notes/dados/sqlite.md" {
		t.Fatalf("expected sqlite.md via chunks_fts fallback, got %+v", lexResults)
	}
}

func TestDotProductBlobLE_Parity(t *testing.T) {
	q := make([]float32, 384)
	v := make([]float32, 384)
	for i := range q {
		q[i] = float32(i%17-8) * 0.031
		v[i] = float32(i%13-6) * 0.047
	}
	blob := encodeVector(v)
	want := dotProduct(q, v)
	got := dotProductBlobLE(q, blob)
	diff := want - got
	if diff < 0 {
		diff = -diff
	}
	if diff > 1e-9 {
		t.Fatalf("dotProductBlobLE mismatch: want %.12f, got %.12f", want, got)
	}
}

func TestVectorCache_Parity(t *testing.T) {
	vaultRoot := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "vault_index.db")

	writeTestNote(t, vaultRoot, "notes/arquitetura/raft.md", `---
type: arquitetura
description: State machine replication
---
# Raft Protocol

## Leader Election
Nodes elect a leader using randomized timeouts.
`)
	writeTestNote(t, vaultRoot, "notes/dados/sqlite.md", `---
type: dados
description: Embedded SQL engine
---
# SQLite Storage

## FTS5 Engine
Uses porter unicode61 tokenizer for fast lexical search.
`)

	engine, err := Open(dbPath, vaultRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close() //nolint:errcheck

	ctx := context.Background()
	if _, _, err := engine.IndexVault(ctx, []string{"notes"}, false); err != nil {
		t.Fatal(err)
	}
	emb := newSemanticMockEmbedder()
	if _, err := engine.EmbedPendingChunks(ctx, emb, "all", 64, 0, nil); err != nil {
		t.Fatal(err)
	}

	uncached, err := engine.SearchHybrid(ctx, SearchOptions{Query: "consensus", Scope: "tech", Limit: 5}, emb)
	if err != nil {
		t.Fatal(err)
	}

	if err := engine.WarmVectorCache(ctx, emb.Dimensions()); err != nil {
		t.Fatal(err)
	}
	cached, err := engine.SearchHybrid(ctx, SearchOptions{Query: "consensus", Scope: "tech", Limit: 5}, emb)
	if err != nil {
		t.Fatal(err)
	}

	if len(uncached) != len(cached) || len(cached) == 0 {
		t.Fatalf("expected same non-empty result count: uncached=%d cached=%d", len(uncached), len(cached))
	}
	for i := range uncached {
		if uncached[i].RelPath != cached[i].RelPath || math.Abs(uncached[i].Score-cached[i].Score) > 1e-9 {
			t.Fatalf("mismatch at %d: uncached=%+v cached=%+v", i, uncached[i], cached[i])
		}
	}
}

func BenchmarkQueryChunksVector(b *testing.B) {
	vaultRoot := b.TempDir()
	dbPath := filepath.Join(b.TempDir(), "vault_index.db")

	engine, err := Open(dbPath, vaultRoot)
	if err != nil {
		b.Fatal(err)
	}
	defer engine.Close() //nolint:errcheck

	ctx := context.Background()
	const numDocs = 500
	tx, err := engine.DB().BeginTx(ctx, nil)
	if err != nil {
		b.Fatal(err)
	}
	vec := make([]float32, 384)
	for i := range vec {
		vec[i] = float32(i%11+1) * 0.01
	}
	vec = normalizeL2(vec)
	blob := encodeVector(vec)

	for i := 0; i < numDocs; i++ {
		rel := filepath.Join("notes/go", fmt.Sprintf("note-%04d.md", i))
		_, err := tx.ExecContext(ctx, `
			INSERT INTO notes_meta (rel_path, title, category, scope, note_type, description, tags, mtime)
			VALUES (?, ?, 'notes/go', 'tech', 'tecnologia', 'Benchmark note', 'go', 1.0)
		`, rel, fmt.Sprintf("Note %d", i))
		if err != nil {
			b.Fatal(err)
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO chunks (rel_path, chunk_idx, scope, heading, content, content_hash, embedding, model, dim, mtime)
			VALUES (?, 0, 'tech', 'Heading', 'Some benchmark content for vector search', 'hash', ?, 'test-model', 384, 1.0)
		`, rel, blob)
		if err != nil {
			b.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		res, err := queryChunksVector(ctx, engine.DB(), SearchOptions{Query: "test", Scope: "tech", Limit: 10}, "tech", vec, hybridCandidateCap)
		if err != nil {
			b.Fatal(err)
		}
		if len(res) != hybridCandidateCap {
			b.Fatalf("expected %d results, got %d", hybridCandidateCap, len(res))
		}
	}
}
