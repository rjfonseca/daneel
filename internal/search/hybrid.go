//nolint:revive // package comment not required
package search

import (
	"container/heap"
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
)

const (
	rrfK               = 60.0
	hybridCandidateCap = 50
)

type dbTarget struct {
	db          *sql.DB
	filterScope string
}

func (e *Engine) resolveTargetDBs(scope string) ([]dbTarget, error) {
	trimmed := strings.TrimSpace(scope)
	switch trimmed {
	case "sources":
		if e.sourcesDB == nil && !e.sourcesDBExistsOnDisk() {
			return nil, nil
		}
		sdb, err := e.ensureSourcesDB()
		if err != nil {
			return nil, err
		}
		return []dbTarget{{db: sdb, filterScope: ""}}, nil
	case "", "all":
		targets := []dbTarget{{db: e.db, filterScope: ""}}
		if e.sourcesDB != nil || e.sourcesDBExistsOnDisk() {
			sdb, err := e.ensureSourcesDB()
			if err != nil {
				return nil, err
			}
			targets = append(targets, dbTarget{db: sdb, filterScope: ""})
		}
		return targets, nil
	default:
		return []dbTarget{{db: e.db, filterScope: trimmed}}, nil
	}
}

// EmbedPendingChunks computes and stores embeddings for chunks missing an embedding or
// embedded with a different model. Commits per batch so interrupted runs are resumable.
func (e *Engine) EmbedPendingChunks(
	ctx context.Context,
	embedder Embedder,
	scope string,
	batchSize int,
	limit int,
	progressFn func(done, total int),
) (int, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if embedder == nil {
		return 0, fmt.Errorf("embedder is required")
	}
	if batchSize <= 0 {
		batchSize = 64
	}

	targets, err := e.resolveTargetDBs(scope)
	if err != nil {
		return 0, err
	}
	if len(targets) == 0 {
		return 0, nil
	}

	modelName := embedder.Model()

	total := 0
	for _, tgt := range targets {
		cnt, err := countPendingChunks(ctx, tgt.db, modelName, tgt.filterScope)
		if err != nil {
			return 0, err
		}
		total += cnt
	}
	if limit > 0 && total > limit {
		total = limit
	}
	if total == 0 {
		return 0, nil
	}

	done := 0
	for _, tgt := range targets {
		var lastID int64
		for {
			if err := ctx.Err(); err != nil {
				return done, err
			}
			if limit > 0 && done >= limit {
				break
			}

			fetchLimit := batchSize
			if limit > 0 && limit-done < fetchLimit {
				fetchLimit = limit - done
			}

			batch, nextLastID, err := fetchPendingChunkBatch(ctx, tgt.db, modelName, tgt.filterScope, lastID, fetchLimit)
			if err != nil {
				return done, err
			}
			if len(batch) == 0 {
				break
			}
			lastID = nextLastID

			texts := make([]string, len(batch))
			for i, item := range batch {
				texts[i] = item.content
			}

			vecs, err := embedder.Embed(ctx, texts)
			if err != nil {
				return done, fmt.Errorf("failed to embed chunk batch: %w", err)
			}
			if len(vecs) != len(batch) {
				return done, fmt.Errorf("embedder returned %d vectors for %d chunks", len(vecs), len(batch))
			}

			tx, err := tgt.db.BeginTx(ctx, nil)
			if err != nil {
				return done, err
			}
			stmt, err := tx.PrepareContext(ctx, `UPDATE chunks SET embedding = ?, model = ?, dim = ? WHERE id = ?`)
			if err != nil {
				_ = tx.Rollback()
				return done, err
			}

			for i, item := range batch {
				blob := encodeVector(vecs[i])
				dim := len(vecs[i])
				if _, err := stmt.ExecContext(ctx, blob, modelName, dim, item.id); err != nil {
					_ = stmt.Close()
					_ = tx.Rollback()
					return done, err
				}
			}
			_ = stmt.Close()
			if err := tx.Commit(); err != nil {
				return done, err
			}

			done += len(batch)
			if progressFn != nil {
				progressFn(done, total)
			}
		}
	}

	return done, nil
}

type pendingChunk struct {
	id      int64
	content string
}

func countPendingChunks(ctx context.Context, db *sql.DB, modelName, filterScope string) (int, error) {
	query := `SELECT COUNT(*) FROM chunks WHERE (embedding IS NULL OR model != ?)`
	args := []interface{}{modelName}
	if filterScope != "" {
		query += ` AND scope = ?`
		args = append(args, filterScope)
	}
	var cnt int
	if err := db.QueryRowContext(ctx, query, args...).Scan(&cnt); err != nil {
		return 0, err
	}
	return cnt, nil
}

func fetchPendingChunkBatch(ctx context.Context, db *sql.DB, modelName, filterScope string, lastID int64, limit int) ([]pendingChunk, int64, error) {
	query := `SELECT id, content FROM chunks WHERE id > ? AND (embedding IS NULL OR model != ?)`
	args := []interface{}{lastID, modelName}
	if filterScope != "" {
		query += ` AND scope = ?`
		args = append(args, filterScope)
	}
	query += ` ORDER BY id ASC LIMIT ?`
	args = append(args, limit)

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, lastID, err
	}
	defer rows.Close() //nolint:errcheck,gosec

	var out []pendingChunk
	nextID := lastID
	for rows.Next() {
		var item pendingChunk
		if err := rows.Scan(&item.id, &item.content); err != nil {
			return nil, lastID, err
		}
		out = append(out, item)
		if item.id > nextID {
			nextID = item.id
		}
	}
	return out, nextID, rows.Err()
}

type chunkKey struct {
	relPath  string
	chunkIdx int
}

type chunkCandidate struct {
	relPath     string
	chunkIdx    int
	title       string
	category    string
	scope       string
	noteType    string
	description string
	heading     string
	content     string
	snippet     string
	bm25Score   float64
	vecScore    float64
}

// SearchHybrid performs Hybrid RAG search combining chunk-level BM25 (chunks_fts)
// and dense vector similarity using Reciprocal Rank Fusion (RRF, k=60).
// Falls back gracefully to chunks_fts or notes_fts when embeddings are unavailable.
func (e *Engine) SearchHybrid(ctx context.Context, opts SearchOptions, embedder Embedder) ([]SearchResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	trimmedQuery := strings.TrimSpace(opts.Query)
	if trimmedQuery == "" {
		return e.SearchFaceted(ctx, opts)
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = 10
	}

	scope := strings.TrimSpace(opts.Scope)
	targets, err := e.resolveTargetDBs(scope)
	if err != nil {
		return nil, err
	}
	if len(targets) == 0 {
		return nil, nil
	}

	// Check if chunks_fts has any rows; if not, fall back to notes_fts via SearchFaceted.
	hasChunks := false
	for _, tgt := range targets {
		var cnt int
		if err := tgt.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM chunks LIMIT 1`).Scan(&cnt); err == nil && cnt > 0 {
			hasChunks = true
			break
		}
	}
	if !hasChunks {
		return e.SearchFaceted(ctx, opts)
	}

	// 1 & 2. Run Lexical Top 50 (chunks_fts) and Vectorial Top 50 concurrently.
	var (
		lexicalAll []chunkCandidate
		lexErr     error
		vectorAll  []chunkCandidate
		vecErr     error
		wg         sync.WaitGroup
	)

	wg.Add(1)
	go func() {
		defer wg.Done()
		var targetMu sync.Mutex
		var targetWG sync.WaitGroup
		for _, tgt := range targets {
			tgt := tgt
			targetWG.Add(1)
			go func() {
				defer targetWG.Done()
				lex, err := queryChunksFTS(ctx, tgt.db, opts, tgt.filterScope, hybridCandidateCap)
				targetMu.Lock()
				defer targetMu.Unlock()
				if err != nil && lexErr == nil {
					lexErr = err
					return
				}
				lexicalAll = append(lexicalAll, lex...)
			}()
		}
		targetWG.Wait()
	}()

	if embedder != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Check if any target DB has embeddings before computing query vector.
			embTargets := make([]dbTarget, 0, len(targets))
			for _, tgt := range targets {
				hasEmb, err := dbHasEmbeddings(ctx, tgt.db, tgt.filterScope)
				if err == nil && hasEmb {
					embTargets = append(embTargets, tgt)
				}
			}
			if len(embTargets) == 0 {
				return
			}
			queryVec, err := embedQuery(ctx, embedder, trimmedQuery)
			if err != nil || len(queryVec) == 0 {
				return
			}

			var targetMu sync.Mutex
			var targetWG sync.WaitGroup
			for _, tgt := range embTargets {
				tgt := tgt
				targetWG.Add(1)
				go func() {
					defer targetWG.Done()
					vecs, err := e.queryTargetVector(ctx, tgt.db, opts, tgt.filterScope, queryVec, hybridCandidateCap)
					targetMu.Lock()
					defer targetMu.Unlock()
					if err != nil && vecErr == nil {
						vecErr = err
						return
					}
					vectorAll = append(vectorAll, vecs...)
				}()
			}
			targetWG.Wait()
		}()
	}

	wg.Wait()
	if lexErr != nil {
		return nil, lexErr
	}
	if vecErr != nil {
		return nil, vecErr
	}

	sort.Slice(lexicalAll, func(i, j int) bool {
		return lexicalAll[i].bm25Score < lexicalAll[j].bm25Score
	})
	if len(lexicalAll) > hybridCandidateCap {
		lexicalAll = lexicalAll[:hybridCandidateCap]
	}

	sort.Slice(vectorAll, func(i, j int) bool {
		return vectorAll[i].vecScore > vectorAll[j].vecScore
	})
	if len(vectorAll) > hybridCandidateCap {
		vectorAll = vectorAll[:hybridCandidateCap]
	}

	// 3. RRF (k = 60)
	type fusedChunk struct {
		cand  chunkCandidate
		score float64
	}
	fusedMap := make(map[chunkKey]*fusedChunk)

	for rank, c := range lexicalAll {
		k := chunkKey{relPath: c.relPath, chunkIdx: c.chunkIdx}
		entry, ok := fusedMap[k]
		if !ok {
			copyC := c
			entry = &fusedChunk{cand: copyC}
			fusedMap[k] = entry
		}
		entry.score += 1.0 / (rrfK + float64(rank+1))
	}

	for rank, c := range vectorAll {
		k := chunkKey{relPath: c.relPath, chunkIdx: c.chunkIdx}
		entry, ok := fusedMap[k]
		if !ok {
			copyC := c
			entry = &fusedChunk{cand: copyC}
			fusedMap[k] = entry
		}
		entry.score += 1.0 / (rrfK + float64(rank+1))
	}

	// 4. Deduplicate by rel_path keeping the highest-scoring chunk
	bestByNote := make(map[string]*fusedChunk)
	for _, fc := range fusedMap {
		existing, ok := bestByNote[fc.cand.relPath]
		if !ok || fc.score > existing.score {
			bestByNote[fc.cand.relPath] = fc
		}
	}

	results := make([]SearchResult, 0, len(bestByNote))
	for _, fc := range bestByNote {
		c := fc.cand
		snip := c.snippet
		if snip == "" {
			snip = fallbackChunkSnippet(c.content, c.title, c.heading)
		}
		results = append(results, SearchResult{
			RelPath:      c.relPath,
			Path:         filepath.Join(e.vaultRoot, c.relPath),
			Title:        c.title,
			Category:     c.category,
			Scope:        c.scope,
			NoteType:     c.noteType,
			Type:         c.noteType,
			Description:  c.description,
			Desc:         c.description,
			Snippet:      snip,
			ChunkHeading: c.heading,
			Score:        fc.score,
		})
	}

	globalScope := scope == "" || scope == "all"
	sort.Slice(results, func(i, j int) bool {
		if globalScope {
			pi := scopePriority(results[i].Scope)
			pj := scopePriority(results[j].Scope)
			if pi != pj {
				return pi < pj
			}
		}
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		return results[i].RelPath < results[j].RelPath
	})

	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

func scopePriority(scope string) int {
	switch scope {
	case "tech":
		return 0
	case "meta":
		return 1
	case "daily":
		return 2
	case "sources":
		return 4
	default:
		return 3
	}
}

func fallbackChunkSnippet(content, title, heading string) string {
	s := strings.TrimSpace(content)
	if idx := strings.Index(s, "\n\n"); idx >= 0 {
		prefix := strings.TrimSpace(s[:idx])
		if prefix == strings.TrimSpace(title) || (heading != "" && strings.Contains(prefix, heading)) {
			s = strings.TrimSpace(s[idx+2:])
		}
	}
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > 180 {
		return s[:177] + "..."
	}
	return s
}

func queryChunksFTS(ctx context.Context, db *sql.DB, opts SearchOptions, filterScope string, capLimit int) ([]chunkCandidate, error) {
	wordReg := regexp.MustCompile(`[a-zA-Z0-9_\-]+`)
	words := wordReg.FindAllString(strings.TrimSpace(opts.Query), -1)
	if len(words) == 0 {
		return nil, nil
	}

	termList := make([]string, 0, len(words))
	for _, w := range words {
		clean := strings.ReplaceAll(w, `"`, "")
		termList = append(termList, fmt.Sprintf(`"%s"*`, clean))
	}
	ftsQuery := strings.Join(termList, " OR ")

	var qb strings.Builder
	var args []interface{}

	qb.WriteString(`
		SELECT f.rel_path, f.chunk_idx, m.title, m.category, m.scope, m.note_type, m.description,
		       f.heading, f.content,
		       snippet(chunks_fts, 4, '==', '==', '...', 20) AS snip,
		       bm25(chunks_fts, 1.0, 1.0, 1.0, 3.0, 1.0) AS rank
		FROM chunks_fts f
		JOIN notes_meta m ON f.rel_path = m.rel_path
		WHERE chunks_fts MATCH ?
	`)
	args = append(args, ftsQuery)

	if filterScope != "" && filterScope != "all" {
		qb.WriteString(" AND m.scope = ?")
		args = append(args, filterScope)
	}
	if nt := strings.TrimSpace(opts.NoteType); nt != "" {
		qb.WriteString(" AND LOWER(m.note_type) = LOWER(?)")
		args = append(args, nt)
	}
	if tag := strings.TrimSpace(opts.Tag); tag != "" {
		cleanTag := strings.TrimPrefix(tag, "#")
		qb.WriteString(" AND (m.tags LIKE ? OR m.tags LIKE ? OR m.tags LIKE ? OR m.tags = ?)")
		args = append(args, "% "+cleanTag+" %", cleanTag+" %", "% "+cleanTag, cleanTag)
	}

	qb.WriteString(" ORDER BY rank ASC LIMIT ?")
	args = append(args, capLimit)

	rows, err := db.QueryContext(ctx, qb.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck,gosec

	var out []chunkCandidate
	for rows.Next() {
		var c chunkCandidate
		if err := rows.Scan(
			&c.relPath, &c.chunkIdx, &c.title, &c.category, &c.scope, &c.noteType, &c.description,
			&c.heading, &c.content, &c.snippet, &c.bm25Score,
		); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func dbHasEmbeddings(ctx context.Context, db *sql.DB, filterScope string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM chunks WHERE embedding IS NOT NULL`
	var args []interface{}
	if filterScope != "" && filterScope != "all" {
		query += ` AND scope = ?`
		args = append(args, filterScope)
	}
	query += `)`
	var exists bool
	if err := db.QueryRowContext(ctx, query, args...).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

// HasAnyEmbeddings reports whether any chunk in the primary or sources database has a stored embedding.
func (e *Engine) HasAnyEmbeddings(ctx context.Context) bool {
	if ctx == nil {
		ctx = context.Background()
	}
	if ok, err := dbHasEmbeddings(ctx, e.db, ""); err == nil && ok {
		return true
	}
	if e.sourcesDB != nil || e.sourcesDBExistsOnDisk() {
		if sdb, err := e.ensureSourcesDB(); err == nil {
			if ok, err := dbHasEmbeddings(ctx, sdb, ""); err == nil && ok {
				return true
			}
		}
	}
	return false
}

type scoredChunkID struct {
	id    int64
	score float64
}

type scoredChunkMinHeap []scoredChunkID

func (h scoredChunkMinHeap) Len() int           { return len(h) }
func (h scoredChunkMinHeap) Less(i, j int) bool { return h[i].score < h[j].score }
func (h scoredChunkMinHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *scoredChunkMinHeap) Push(x any)        { *h = append(*h, x.(scoredChunkID)) }
func (h *scoredChunkMinHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[:n-1]
	return item
}

func pushTopK(h *scoredChunkMinHeap, capLimit int, item scoredChunkID) {
	if h.Len() < capLimit {
		heap.Push(h, item)
		return
	}
	if capLimit > 0 && item.score > (*h)[0].score {
		(*h)[0] = item
		heap.Fix(h, 0)
	}
}

func (e *Engine) queryTargetVector(ctx context.Context, db *sql.DB, opts SearchOptions, filterScope string, queryVec []float32, capLimit int) ([]chunkCandidate, error) {
	if strings.TrimSpace(opts.NoteType) == "" && strings.TrimSpace(opts.Tag) == "" {
		e.vecCacheMu.RLock()
		vc := e.vecCache
		e.vecCacheMu.RUnlock()
		if vc != nil {
			var targetCache *dbVectorCache
			switch db {
			case e.db:
				targetCache = &vc.primary
			case e.sourcesDB:
				targetCache = &vc.sources
			}
			if targetCache != nil {
				if res, ok, err := targetCache.queryTopK(ctx, db, filterScope, queryVec, capLimit); err == nil && ok {
					return res, nil
				}
			}
		}
	}
	return queryChunksVector(ctx, db, opts, filterScope, queryVec, capLimit)
}

const vectorScanBatchSize = 4096

func queryChunksVector(ctx context.Context, db *sql.DB, opts SearchOptions, filterScope string, queryVec []float32, capLimit int) ([]chunkCandidate, error) {
	if capLimit <= 0 {
		return nil, nil
	}

	var qb strings.Builder
	var args []interface{}

	nt := strings.TrimSpace(opts.NoteType)
	tag := strings.TrimSpace(opts.Tag)
	needsMetaJoin := nt != "" || tag != ""

	qb.WriteString(`SELECT c.id, c.embedding FROM chunks c `)
	if needsMetaJoin {
		qb.WriteString(`JOIN notes_meta m ON c.rel_path = m.rel_path `)
	}
	qb.WriteString(`WHERE c.embedding IS NOT NULL AND c.dim = ?`)
	args = append(args, len(queryVec))

	if filterScope != "" && filterScope != "all" {
		if needsMetaJoin {
			qb.WriteString(" AND m.scope = ?")
		} else {
			qb.WriteString(" AND c.scope = ?")
		}
		args = append(args, filterScope)
	}
	if nt != "" {
		qb.WriteString(" AND LOWER(m.note_type) = LOWER(?)")
		args = append(args, nt)
	}
	if tag != "" {
		cleanTag := strings.TrimPrefix(tag, "#")
		qb.WriteString(" AND (m.tags LIKE ? OR m.tags LIKE ? OR m.tags LIKE ? OR m.tags = ?)")
		args = append(args, "% "+cleanTag+" %", cleanTag+" %", "% "+cleanTag, cleanTag)
	}

	rows, err := db.QueryContext(ctx, qb.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck,gosec

	stride := len(queryVec) * 4
	ids := make([]int64, vectorScanBatchSize)
	arena := make([]byte, vectorScanBatchSize*stride)

	workers := runtime.NumCPU()
	if workers > 8 {
		workers = 8
	}
	if workers < 1 {
		workers = 1
	}

	workerHeaps := make([]scoredChunkMinHeap, workers)
	for w := range workerHeaps {
		workerHeaps[w] = make(scoredChunkMinHeap, 0, capLimit)
	}

	scoreBatch := func(count int) {
		if count == 0 {
			return
		}
		activeWorkers := workers
		if count < 64 {
			activeWorkers = 1
		}
		var wg sync.WaitGroup
		chunkSize := (count + activeWorkers - 1) / activeWorkers
		for w := 0; w < activeWorkers; w++ {
			start := w * chunkSize
			if start >= count {
				break
			}
			end := start + chunkSize
			if end > count {
				end = count
			}
			wg.Add(1)
			go func(workerIdx, lo, hi int) {
				defer wg.Done()
				h := &workerHeaps[workerIdx]
				for i := lo; i < hi; i++ {
					off := i * stride
					score := dotProductBlobLE(queryVec, arena[off:off+stride])
					pushTopK(h, capLimit, scoredChunkID{id: ids[i], score: score})
				}
			}(w, start, end)
		}
		wg.Wait()
	}

	count := 0
	for rows.Next() {
		var id int64
		var raw sql.RawBytes
		if err := rows.Scan(&id, &raw); err != nil {
			return nil, err
		}
		if len(raw) != stride {
			continue
		}
		ids[count] = id
		copy(arena[count*stride:(count+1)*stride], raw)
		count++
		if count == vectorScanBatchSize {
			scoreBatch(count)
			count = 0
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	scoreBatch(count)

	// Merge worker Top-K heaps into global Top-K
	globalHeap := make(scoredChunkMinHeap, 0, capLimit)
	for w := range workerHeaps {
		for _, item := range workerHeaps[w] {
			pushTopK(&globalHeap, capLimit, item)
		}
	}
	if globalHeap.Len() == 0 {
		return nil, nil
	}

	return hydrateChunkCandidates(ctx, db, globalHeap)
}
