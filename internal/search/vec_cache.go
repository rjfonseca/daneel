//nolint:revive // package comment not required
package search

import (
	"context"
	"database/sql"
	"encoding/binary"
	"math"
	"runtime"
	"sort"
	"strings"
	"sync"
)

// dbVectorCache holds an in-memory contiguous float32 matrix of chunk embeddings for a single SQLite DB.
type dbVectorCache struct {
	mu      sync.RWMutex
	dim     int
	rowCnt  int64
	maxID   int64
	ids     []int64
	scopes  []string
	vectors []float32 // flat row-major: len(ids) * dim
}

// VectorCache caches chunk embeddings in RAM per database (primary & sources) to eliminate
// SQLite BLOB I/O during semantic search in long-lived processes (daemon / MCP).
type VectorCache struct {
	primary dbVectorCache
	sources dbVectorCache
}

// EnableVectorCache enables the in-memory vector cache on this Engine.
func (e *Engine) EnableVectorCache() {
	e.vecCacheMu.Lock()
	defer e.vecCacheMu.Unlock()
	if e.vecCache == nil {
		e.vecCache = &VectorCache{}
	}
}

// WarmVectorCache preloads chunk embeddings for primary and (if present) sources DBs into RAM.
func (e *Engine) WarmVectorCache(ctx context.Context, dim int) error {
	if ctx == nil {
		ctx = context.Background()
	}
	e.EnableVectorCache()
	if err := e.vecCache.primary.ensureLoaded(ctx, e.db, dim); err != nil {
		return err
	}
	if e.sourcesDB != nil || e.sourcesDBExistsOnDisk() {
		sdb, err := e.ensureSourcesDB()
		if err != nil {
			return err
		}
		if err := e.vecCache.sources.ensureLoaded(ctx, sdb, dim); err != nil {
			return err
		}
	}
	return nil
}

func (c *dbVectorCache) ensureLoaded(ctx context.Context, db *sql.DB, dim int) error {
	if db == nil || dim <= 0 {
		return nil
	}

	var cnt int64
	var maxID sql.NullInt64
	err := db.QueryRowContext(ctx,
		`SELECT COUNT(*), MAX(id) FROM chunks WHERE embedding IS NOT NULL AND dim = ?`, dim,
	).Scan(&cnt, &maxID)
	if err != nil {
		return err
	}

	c.mu.RLock()
	upToDate := c.dim == dim && c.rowCnt == cnt && c.maxID == maxID.Int64 && len(c.ids) == int(cnt)
	c.mu.RUnlock()
	if upToDate {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	// Re-check under write lock
	if c.dim == dim && c.rowCnt == cnt && c.maxID == maxID.Int64 && len(c.ids) == int(cnt) {
		return nil
	}

	if cnt == 0 {
		c.dim = dim
		c.rowCnt = 0
		c.maxID = 0
		c.ids = nil
		c.scopes = nil
		c.vectors = nil
		return nil
	}

	rows, err := db.QueryContext(ctx,
		`SELECT id, scope, embedding FROM chunks WHERE embedding IS NOT NULL AND dim = ? ORDER BY id ASC`, dim,
	)
	if err != nil {
		return err
	}
	defer rows.Close() //nolint:errcheck,gosec

	ids := make([]int64, 0, cnt)
	scopes := make([]string, 0, cnt)
	vectors := make([]float32, 0, int(cnt)*dim)
	stride := dim * 4

	for rows.Next() {
		var id int64
		var scope string
		var raw sql.RawBytes
		if err := rows.Scan(&id, &scope, &raw); err != nil {
			return err
		}
		if len(raw) != stride {
			continue
		}
		ids = append(ids, id)
		scopes = append(scopes, scope)
		for i := 0; i < dim; i++ {
			vectors = append(vectors, math.Float32frombits(binary.LittleEndian.Uint32(raw[i*4:])))
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	c.dim = dim
	c.rowCnt = cnt
	c.maxID = maxID.Int64
	c.ids = ids
	c.scopes = scopes
	c.vectors = vectors
	return nil
}

func (c *dbVectorCache) queryTopK(
	ctx context.Context,
	db *sql.DB,
	filterScope string,
	queryVec []float32,
	capLimit int,
) ([]chunkCandidate, bool, error) {
	if err := c.ensureLoaded(ctx, db, len(queryVec)); err != nil {
		return nil, false, err
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	n := len(c.ids)
	if n == 0 || c.dim != len(queryVec) {
		return nil, true, nil
	}

	dim := c.dim
	checkScope := filterScope != "" && filterScope != "all"

	workers := runtime.NumCPU()
	if workers > 8 {
		workers = 8
	}
	if workers < 1 || n < 256 {
		workers = 1
	}

	workerHeaps := make([]scoredChunkMinHeap, workers)
	for w := range workerHeaps {
		workerHeaps[w] = make(scoredChunkMinHeap, 0, capLimit)
	}

	var wg sync.WaitGroup
	chunkSize := (n + workers - 1) / workers
	for w := 0; w < workers; w++ {
		start := w * chunkSize
		if start >= n {
			break
		}
		end := start + chunkSize
		if end > n {
			end = n
		}
		wg.Add(1)
		go func(workerIdx, lo, hi int) {
			defer wg.Done()
			h := &workerHeaps[workerIdx]
			for i := lo; i < hi; i++ {
				if checkScope && c.scopes[i] != filterScope {
					continue
				}
				off := i * dim
				score := dotProductUnrolled(queryVec, c.vectors[off:off+dim])
				pushTopK(h, capLimit, scoredChunkID{id: c.ids[i], score: score})
			}
		}(w, start, end)
	}
	wg.Wait()

	globalHeap := make(scoredChunkMinHeap, 0, capLimit)
	for w := range workerHeaps {
		for _, item := range workerHeaps[w] {
			pushTopK(&globalHeap, capLimit, item)
		}
	}
	if globalHeap.Len() == 0 {
		return nil, true, nil
	}

	res, err := hydrateChunkCandidates(ctx, db, globalHeap)
	return res, true, err
}

func dotProductUnrolled(a, b []float32) float64 {
	n := len(a)
	var sum0, sum1, sum2, sum3 float64
	i := 0
	for ; i+3 < n; i += 4 {
		sum0 += float64(a[i]) * float64(b[i])
		sum1 += float64(a[i+1]) * float64(b[i+1])
		sum2 += float64(a[i+2]) * float64(b[i+2])
		sum3 += float64(a[i+3]) * float64(b[i+3])
	}
	sum := sum0 + sum1 + sum2 + sum3
	for ; i < n; i++ {
		sum += float64(a[i]) * float64(b[i])
	}
	return sum
}

func hydrateChunkCandidates(ctx context.Context, db *sql.DB, globalHeap scoredChunkMinHeap) ([]chunkCandidate, error) {
	scoreByID := make(map[int64]float64, globalHeap.Len())
	placeholders := make([]string, 0, globalHeap.Len())
	hydrateArgs := make([]interface{}, 0, globalHeap.Len())
	for _, item := range globalHeap {
		scoreByID[item.id] = item.score
		placeholders = append(placeholders, "?")
		hydrateArgs = append(hydrateArgs, item.id)
	}

	var hqb strings.Builder
	hqb.WriteString(`
		SELECT c.id, c.rel_path, c.chunk_idx, m.title, m.category, m.scope, m.note_type, m.description,
		       c.heading, c.content
		FROM chunks c
		JOIN notes_meta m ON c.rel_path = m.rel_path
		WHERE c.id IN (`)
	hqb.WriteString(strings.Join(placeholders, ","))
	hqb.WriteString(`)`)

	hRows, err := db.QueryContext(ctx, hqb.String(), hydrateArgs...)
	if err != nil {
		return nil, err
	}
	defer hRows.Close() //nolint:errcheck,gosec

	scored := make([]chunkCandidate, 0, len(scoreByID))
	for hRows.Next() {
		var id int64
		var c chunkCandidate
		if err := hRows.Scan(
			&id, &c.relPath, &c.chunkIdx, &c.title, &c.category,
			&c.scope, &c.noteType, &c.description,
			&c.heading, &c.content,
		); err != nil {
			return nil, err
		}
		c.vecScore = scoreByID[id]
		scored = append(scored, c)
	}
	if err := hRows.Err(); err != nil {
		return nil, err
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].vecScore > scored[j].vecScore
	})
	return scored, nil
}
