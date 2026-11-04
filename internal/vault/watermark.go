//nolint:revive // package comment not required
package vault

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	_ "modernc.org/sqlite" // SQLite driver
)

// FileWatermark records fast change-detection metadata and extracted links for a note.
type FileWatermark struct {
	Path        string `json:"path"`
	MTime       int64  `json:"mtime"`
	ContentHash string `json:"content_hash"`
	LinkCount   int    `json:"link_count"`
	Title       string `json:"title"`
	NoteType    string `json:"note_type"`
	Description string `json:"description"`
	Links       []Link `json:"links"`
}

// ScanVaultIncremental traverses indexedDirs under vaultRoot using filepath.WalkDir and caches
// file watermarks and lightweight note metadata in <vaultRoot>/.daneel/cache/vault_index.db.
func ScanVaultIncremental(vaultRoot string, indexedDirs []string, force bool) ([]*Note, error) {
	cacheDir := filepath.Join(vaultRoot, ".daneel", "cache")
	if err := os.MkdirAll(cacheDir, 0750); err != nil {
		return ScanVault(vaultRoot, indexedDirs)
	}

	dbPath := filepath.Join(cacheDir, "vault_index.db")
	dsn := fmt.Sprintf("%s?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(10000)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return ScanVault(vaultRoot, indexedDirs)
	}
	defer db.Close() //nolint:errcheck

	if err := db.Ping(); err != nil {
		return ScanVault(vaultRoot, indexedDirs)
	}

	if err := initWatermarkSchema(db); err != nil {
		return ScanVault(vaultRoot, indexedDirs)
	}

	watermarks, err := loadWatermarks(db)
	if err != nil {
		return ScanVault(vaultRoot, indexedDirs)
	}

	type parseTask struct {
		absPath   string
		relPath   string
		mtimeNano int64
	}

	foundOnDisk := make(map[string]bool)
	var cachedNotes []*Note
	var toParse []parseTask

	for _, dir := range indexedDirs {
		absDir := filepath.Join(vaultRoot, dir)
		if _, err := os.Stat(absDir); os.IsNotExist(err) {
			continue
		}

		walkErr := filepath.WalkDir(absDir, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}

			if d.IsDir() {
				if isIgnoredDir(d.Name()) {
					return filepath.SkipDir
				}
				return nil
			}

			if !strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
				return nil
			}

			info, err := d.Info()
			if err != nil {
				return err
			}

			relPath, err := filepath.Rel(vaultRoot, path)
			if err != nil {
				return err
			}
			relPath = filepath.ToSlash(relPath)
			foundOnDisk[relPath] = true

			mtime := info.ModTime().UnixNano()
			if mtime == 0 {
				mtime = info.ModTime().Unix()
			}

			if !force {
				if existing, ok := watermarks[relPath]; ok && existing.MTime == mtime {
					tax := ResolveTaxonomy(relPath, vaultRoot)
					note := &Note{
						RelPath:     relPath,
						AbsPath:     path,
						Title:       existing.Title,
						Category:    tax.Category,
						Domain:      tax.Domain,
						Group:       tax.Group,
						Repository:  tax.Repository,
						Scope:       DetermineScope(relPath),
						NoteType:    existing.NoteType,
						Description: existing.Description,
						MTime:       info.ModTime(),
						Links:       existing.Links,
						FM:          Frontmatter{Type: existing.NoteType, Description: existing.Description},
					}
					cachedNotes = append(cachedNotes, note)
					return nil
				}
			}

			toParse = append(toParse, parseTask{
				absPath:   path,
				relPath:   relPath,
				mtimeNano: mtime,
			})
			return nil
		})

		if walkErr != nil {
			return nil, walkErr
		}
	}

	var parsedNotes []*Note
	var toUpsert []FileWatermark

	if len(toParse) > 0 {
		numWorkers := runtime.NumCPU()
		if numWorkers > 16 {
			numWorkers = 16
		}
		if numWorkers < 1 {
			numWorkers = 1
		}
		if numWorkers > len(toParse) {
			numWorkers = len(toParse)
		}

		type parseResult struct {
			note      *Note
			watermark FileWatermark
			err       error
		}

		tasks := make(chan parseTask, len(toParse))
		results := make(chan parseResult, len(toParse))

		var wg sync.WaitGroup
		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for task := range tasks {
					n, err := ParseNote(task.absPath, vaultRoot)
					if err != nil || n == nil {
						results <- parseResult{err: err}
						continue
					}
					hash := sha256.Sum256([]byte(n.Content))
					contentHash := hex.EncodeToString(hash[:])
					wm := FileWatermark{
						Path:        n.RelPath,
						MTime:       task.mtimeNano,
						ContentHash: contentHash,
						LinkCount:   len(n.Links),
						Title:       n.Title,
						NoteType:    n.NoteType,
						Description: n.Description,
						Links:       n.Links,
					}
					results <- parseResult{note: n, watermark: wm}
				}
			}()
		}

		for _, t := range toParse {
			tasks <- t
		}
		close(tasks)

		wg.Wait()
		close(results)

		for res := range results {
			if res.err == nil && res.note != nil {
				parsedNotes = append(parsedNotes, res.note)
				toUpsert = append(toUpsert, res.watermark)
			}
		}
	}

	if len(toUpsert) > 0 {
		if tx, err := db.Begin(); err == nil {
			stmt, err := tx.Prepare(`
				INSERT INTO file_watermarks (path, mtime, content_hash, link_count, title, note_type, description, links_json, updated_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
				ON CONFLICT(path) DO UPDATE SET
					mtime = excluded.mtime,
					content_hash = excluded.content_hash,
					link_count = excluded.link_count,
					title = excluded.title,
					note_type = excluded.note_type,
					description = excluded.description,
					links_json = excluded.links_json,
					updated_at = CURRENT_TIMESTAMP
			`)
			if err == nil {
				for _, wm := range toUpsert {
					linksJSON, err := json.Marshal(wm.Links)
					if err != nil || len(linksJSON) == 0 {
						linksJSON = []byte("[]")
					}
					_, _ = stmt.Exec(wm.Path, wm.MTime, wm.ContentHash, wm.LinkCount, wm.Title, wm.NoteType, wm.Description, string(linksJSON))
				}
				_ = stmt.Close()
			}
			_ = tx.Commit()
		}
	}

	var stalePaths []string
	for p := range watermarks {
		if belongsToIndexedDirs(p, indexedDirs) && !foundOnDisk[p] {
			stalePaths = append(stalePaths, p)
		}
	}

	if len(stalePaths) > 0 {
		if tx, err := db.Begin(); err == nil {
			stmt, err := tx.Prepare("DELETE FROM file_watermarks WHERE path = ?")
			if err == nil {
				for _, p := range stalePaths {
					_, _ = stmt.Exec(p)
				}
				_ = stmt.Close()
			}
			_ = tx.Commit()
		}
	}

	allNotes := make([]*Note, 0, len(cachedNotes)+len(parsedNotes))
	allNotes = append(allNotes, cachedNotes...)
	allNotes = append(allNotes, parsedNotes...)

	sort.Slice(allNotes, func(i, j int) bool {
		return allNotes[i].RelPath < allNotes[j].RelPath
	})

	return allNotes, nil
}

func initWatermarkSchema(db *sql.DB) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS file_watermarks (
			path TEXT PRIMARY KEY,
			mtime INTEGER NOT NULL,
			content_hash TEXT NOT NULL,
			link_count INTEGER NOT NULL,
			title TEXT NOT NULL DEFAULT '',
			note_type TEXT NOT NULL DEFAULT '',
			description TEXT NOT NULL DEFAULT '',
			links_json TEXT NOT NULL DEFAULT '[]',
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE INDEX IF NOT EXISTS idx_file_watermarks_mtime ON file_watermarks(mtime);`,
	}
	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

func loadWatermarks(db *sql.DB) (map[string]FileWatermark, error) {
	rows, err := db.Query("SELECT path, mtime, content_hash, link_count, title, note_type, description, links_json FROM file_watermarks")
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck

	watermarks := make(map[string]FileWatermark)
	for rows.Next() {
		var wm FileWatermark
		var linksJSON string
		if err := rows.Scan(&wm.Path, &wm.MTime, &wm.ContentHash, &wm.LinkCount, &wm.Title, &wm.NoteType, &wm.Description, &linksJSON); err == nil {
			if linksJSON != "" {
				_ = json.Unmarshal([]byte(linksJSON), &wm.Links)
			}
			if wm.Links == nil {
				wm.Links = []Link{}
			}
			watermarks[wm.Path] = wm
		}
	}
	return watermarks, rows.Err()
}

func belongsToIndexedDirs(relPath string, indexedDirs []string) bool {
	clean := filepath.ToSlash(filepath.Clean(relPath))
	for _, dir := range indexedDirs {
		cleanDir := filepath.ToSlash(filepath.Clean(dir))
		if cleanDir == "." || cleanDir == "" {
			return true
		}
		if clean == cleanDir || strings.HasPrefix(clean, cleanDir+"/") {
			return true
		}
	}
	return false
}
