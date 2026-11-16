# ADR 0043: Hybrid RAG Chunking and Pure-Go Embeddings

* Status: accepted
* Date: 2026-11-16

## Context and Problem Statement

While SQLite FTS5 (`notes_fts`) provides fast lexical matching with BM25 ranking, note-level full-text indexing has two structural limitations for Retrieval-Augmented Generation (RAG) and semantic discovery:

1. **Coarse Document Granularity**: Long notes covering multiple topics dilute BM25 term frequencies and return broad document snippets rather than the specific section that answers a query.
2. **Lexical Vocabulary Mismatch**: Pure keyword matching misses conceptually related notes when terminology differs (e.g., querying `"distributed consensus"` when a note discusses `"leader election"` and `"raft"`).

At the same time, Daneel strictly requires **Pure Go / Zero CGO** (`CGO_ENABLED=0`) portability across Linux, macOS, and Windows without requiring external ONNX Runtime shared libraries or Python sidecars.

## Decision Drivers

* **Pure Go / Zero CGO**: Maintain `CGO_ENABLED=0` static binaries using `modernc.org/sqlite` and `github.com/rostamlabs/rembed`.
* **Structural Markdown Coherence**: Split notes along Markdown heading boundaries (`#`..`####`) without breaking fenced code blocks (` ``` ` / `~~~`), enriching each chunk with hierarchical context (`title > heading_breadcrumb`).
* **Incremental & Resumable Embedding**: Preserve existing vector embeddings for unchanged sections (`content_hash`) when a note is edited, and commit embedding batches incrementally so interrupted runs resume cleanly.
* **Fast Default CLI & Graceful Fallback**: Keep `daneel search` sub-10ms by default, opt into semantic fusion via `--semantic` (or automatically in MCP `search_vault` when embeddings exist), and fall back seamlessly to `chunks_fts` / `notes_fts` when embeddings are absent or disabled.
* **Isolated `sources_index.db` Compatibility**: Replicate `chunks` and `chunks_fts` across both `vault_index.db` and `sources_index.db`, preserving `tech > meta > daily > other > sources` priority in global searches.

## Considered Options

* **Option 1: External sidecar / Python embedding service (e.g., Ollama / sentence-transformers HTTP server)**:
  * *Disadvantages*: Introduces external runtime dependencies, network/IPC overhead, and breaks Daneel's single-binary local-first guarantees.
* **Option 2: Structural Markdown Chunker + `rembed` Pure-Go Embeddings + RRF (`k = 60`) in SQLite (Chosen)**:
  * Embeds chunks locally in pure Go using `github.com/rostamlabs/rembed v0.3.0` (`sentence-transformers/paraphrase-multilingual-MiniLM-L12-v2` by default).
  * Stores chunk text, SHA-256 `content_hash`, FTS5 index (`chunks_fts`), and little-endian `float32` L2-normalized vectors (`chunks.embedding`) directly in SQLite.

## Decision Outcome

Chosen option: **Option 2: Structural Markdown Chunker + `rembed` Pure-Go Embeddings + RRF (`k = 60`) in SQLite**.

### 1. Structural Markdown Chunker (`internal/search/chunker.go`)

* `ChunkMarkdown(relPath, title, content, maxWords, overlapWords)` splits Markdown by headings (`#`, `##`, `###`, `####`) while tracking fenced code blocks (` ``` ` and `~~~`) so `#` comments inside code are never treated as section splits.
* Sections exceeding `maxWords` (default `350`) are split on paragraph boundaries with `overlapWords` (`40`) word overlap.
* Each chunk is prefixed with `title > heading_breadcrumb` and hashed with SHA-256 (`ContentHash`).

### 2. SQLite Schema & Hash-Preserving Sync (`internal/search/engine.go`)

* Added `chunks` and `chunks_fts` (`porter unicode61`) to both `vault_index.db` and `sources_index.db`, with `idx_chunks_rel_path` and partial index `idx_chunks_pending_embed ON chunks(model) WHERE embedding IS NULL`.
* `IndexVault`, `IndexNote`, and pruning keep `chunks` and `chunks_fts` synchronized and preserve `(embedding, model, dim)` for any chunk whose `content_hash` is unchanged.

### 3. `Embedder` Interface & `RembedEmbedder` (`internal/search/embedder.go`, `embedder_rembed.go`, `internal/config/config.go`)

* Added `EmbeddingsConfig` (`Enabled: true`, `Provider: "rembed"`, `Model: "sentence-transformers/paraphrase-multilingual-MiniLM-L12-v2"`, `Int8: false`, `QueryPrefix`, `DocPrefix`).
* `RembedEmbedder` wraps `*rembed.Embedder`, applies configured prefixes, and explicitly L2-normalizes every output vector (`normalizeL2`) so dot product equals cosine similarity.

### 4. Resumable Vector Indexing & Hybrid RRF Search (`internal/search/hybrid.go`)

* `EmbedPendingChunks` processes chunks where `embedding IS NULL OR model != embedder.Model()` in batches (default `64`), committing each batch independently for 100% resumability.
* `SearchHybrid` executes Top-50 lexical search (`chunks_fts`), Top-50 concurrent vector dot-product search (when `embedder != nil` and embeddings exist), and fuses ranks via Reciprocal Rank Fusion (`1/(60+rank_bm25) + 1/(60+rank_vec)`), deduplicating by `rel_path` and preserving `tech > meta > daily > other > sources` priority for `scope="all"`.

### 5. CLI & MCP Integration (`cmd/index.go`, `cmd/search.go`, `internal/mcp/server.go`)

* `daneel index` adds `--embed`, `--scope`, `--batch-size`, and `--embed-limit`.
* `daneel search` adds `--semantic` (default `false` for fast lexical hot path).
* MCP `search_vault` lazily initializes the `Embedder` once per server instance (only when embeddings exist in the database) and routes queries through `SearchHybrid`.

## Consequences

* **Positive**: Section-level hybrid retrieval resolves both exact technical tokens and conceptual/cross-lingual queries in pure Go with zero CGO or external services.
* **Positive**: Unchanged sections retain their embeddings across incremental note edits, avoiding redundant inference.
* **Negative**: Vector scanning performs an in-process linear scan over stored chunk blobs in the target scope, which is fast for thousands of personal/team chunks but may warrant approximate nearest-neighbor indexing if vaults grow to hundreds of thousands of chunks.
