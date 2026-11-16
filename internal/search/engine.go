//nolint:revive // package comment not required
package search

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite" //nolint:revive

	"github.com/rjfonseca/daneel/internal/vault"
)

// SearchResult represents a matching note from a search query.
type SearchResult struct { //nolint:revive
	RelPath     string  `json:"rel_path"`
	Path        string  `json:"path"`
	Title       string  `json:"title"`
	Category    string  `json:"category"`
	Scope       string  `json:"scope"`
	NoteType    string  `json:"note_type"`
	Type        string  `json:"type"`
	Description string  `json:"description"`
	Desc        string  `json:"desc"`
	Snippet     string  `json:"snippet"`
	Score       float64 `json:"score"`
}

// NoteMeta represents stored metadata for a single note.
type NoteMeta struct {
	RelPath     string    `json:"rel_path"`
	Title       string    `json:"title"`
	Category    string    `json:"category"`
	Scope       string    `json:"scope"`
	NoteType    string    `json:"note_type"`
	Description string    `json:"description"`
	Tags        string    `json:"tags"`
	Headings    string    `json:"headings"`
	Aliases     string    `json:"aliases,omitempty"`
	MTime       time.Time `json:"mtime"`
}

// HealthReport summarizes vault statistics and broken references.
type HealthReport struct {
	TotalNotes   int            `json:"total_notes"`
	TotalLinks   int            `json:"total_links"`
	BrokenLinks  []string       `json:"broken_links"`
	NotesByScope map[string]int `json:"notes_by_scope"`
}

const (
	metaKeyLastSync        = "last_sync_at"
	metaKeySourcesMigrated = "sources_migrated"
	sourcesDBFileName      = "sources_index.db"
)

// Engine encapsulates the SQLite FTS5 search engine.
type Engine struct {
	db            *sql.DB
	sourcesDB     *sql.DB
	sourcesMu     sync.Mutex
	dbPath        string
	sourcesDBPath string
	vaultRoot     string
}

func isMemoryDBPath(dbPath string) bool {
	return dbPath == ":memory:" || strings.HasPrefix(dbPath, "file::memory:")
}

func deriveSourcesDBPath(dbPath string) string {
	if isMemoryDBPath(dbPath) {
		return ":memory:"
	}
	return filepath.Join(filepath.Dir(dbPath), sourcesDBFileName)
}

func sqliteDSN(dbPath string) string {
	if isMemoryDBPath(dbPath) {
		return dbPath
	}
	return fmt.Sprintf("%s?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(10000)", dbPath)
}

// Open initializes and opens the SQLite database.
func Open(dbPath string, vaultRoot string) (*Engine, error) {
	if !isMemoryDBPath(dbPath) {
		if err := os.MkdirAll(filepath.Dir(dbPath), 0750); err != nil {
			return nil, fmt.Errorf("failed to create cache directory: %w", err)
		}
	}

	db, err := sql.Open("sqlite", sqliteDSN(dbPath))
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	engine := &Engine{
		db:            db,
		dbPath:        dbPath,
		sourcesDBPath: deriveSourcesDBPath(dbPath),
		vaultRoot:     vaultRoot,
	}

	if err := initSchemaForDB(db); err != nil {
		db.Close() //nolint:errcheck,gosec // fully read or safe to ignore
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return engine, nil
}

// DB returns the primary SQLite database connection.
func (e *Engine) DB() *sql.DB {
	return e.db
}

// DBPath returns the path to the active SQLite database file.
func (e *Engine) DBPath() string {
	return e.dbPath
}

// SourcesDBPath returns the path to the isolated sources SQLite database file.
func (e *Engine) SourcesDBPath() string {
	return e.sourcesDBPath
}

func (e *Engine) sourcesDBExistsOnDisk() bool {
	if isMemoryDBPath(e.sourcesDBPath) {
		return false
	}
	info, err := os.Stat(e.sourcesDBPath)
	return err == nil && !info.IsDir()
}

// ensureSourcesDB lazily opens and initializes the dedicated sources SQLite database.
func (e *Engine) ensureSourcesDB() (*sql.DB, error) {
	e.sourcesMu.Lock()
	defer e.sourcesMu.Unlock()

	if e.sourcesDB != nil {
		return e.sourcesDB, nil
	}

	if !isMemoryDBPath(e.sourcesDBPath) {
		if err := os.MkdirAll(filepath.Dir(e.sourcesDBPath), 0750); err != nil {
			return nil, fmt.Errorf("failed to create sources cache directory: %w", err)
		}
	}

	sdb, err := sql.Open("sqlite", sqliteDSN(e.sourcesDBPath))
	if err != nil {
		return nil, fmt.Errorf("failed to open sources sqlite database: %w", err)
	}

	if err := initSchemaForDB(sdb); err != nil {
		sdb.Close() //nolint:errcheck,gosec // fully read or safe to ignore
		return nil, fmt.Errorf("failed to initialize sources schema: %w", err)
	}

	e.sourcesDB = sdb
	return e.sourcesDB, nil
}

// Close closes the underlying database connections.
func (e *Engine) Close() error {
	e.sourcesMu.Lock()
	sdb := e.sourcesDB
	e.sourcesDB = nil
	e.sourcesMu.Unlock()

	var errs []error
	if sdb != nil {
		if err := sdb.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if e.db != nil {
		if err := e.db.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func initSchemaForDB(targetDB *sql.DB) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS index_metadata (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS notes_meta (
			rel_path TEXT PRIMARY KEY,
			title TEXT,
			category TEXT,
			scope TEXT,
			note_type TEXT,
			description TEXT,
			tags TEXT,
			headings TEXT,
			aliases TEXT,
			mtime REAL
		);`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS notes_fts USING fts5(
			rel_path UNINDEXED,
			title,
			category,
			scope,
			note_type,
			description,
			tags,
			headings,
			content,
			tokenize="porter unicode61"
		);`,
		`CREATE TABLE IF NOT EXISTS note_links (
			source_rel_path TEXT,
			target_rel_path TEXT,
			link_text TEXT
		);`,
		`CREATE INDEX IF NOT EXISTS idx_note_links_target ON note_links(target_rel_path);`,
		`CREATE INDEX IF NOT EXISTS idx_note_links_source ON note_links(source_rel_path);`,
		`CREATE TABLE IF NOT EXISTS people_meta (
			rel_path TEXT PRIMARY KEY,
			name TEXT,
			role TEXT,
			squad TEXT,
			email TEXT,
			aliases TEXT,
			mtime REAL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_people_name ON people_meta(name);`,
		`CREATE INDEX IF NOT EXISTS idx_people_squad ON people_meta(squad);`,
		`CREATE INDEX IF NOT EXISTS idx_people_role ON people_meta(role);`,
		`CREATE INDEX IF NOT EXISTS idx_people_email ON people_meta(email);`,
		`CREATE TABLE IF NOT EXISTS workspace_repos (
			rel_path TEXT PRIMARY KEY,
			name TEXT,
			host TEXT,
			abs_path TEXT,
			branch TEXT,
			is_clean INTEGER,
			stack TEXT,
			description TEXT,
			mtime REAL,
			json_data TEXT
		);`,
		`CREATE INDEX IF NOT EXISTS idx_workspace_repos_name ON workspace_repos(name);`,
		`CREATE INDEX IF NOT EXISTS idx_workspace_repos_clean ON workspace_repos(is_clean);`,
		`CREATE INDEX IF NOT EXISTS idx_workspace_repos_stack ON workspace_repos(stack);`,
	}

	for _, q := range queries {
		if _, err := targetDB.Exec(q); err != nil {
			return err
		}
	}

	// Safe migration for existing databases without aliases column
	_, _ = targetDB.Exec("ALTER TABLE notes_meta ADD COLUMN aliases TEXT;")

	return nil
}

func cleanPersonName(title string) string {
	clean := strings.TrimSpace(title)
	clean = strings.TrimPrefix(clean, "👤")
	clean = strings.TrimPrefix(clean, "#")
	return strings.TrimSpace(clean)
}

func getExtraString(m map[string]interface{}, keys ...string) string {
	if m == nil {
		return ""
	}
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok {
				return strings.TrimSpace(s)
			}
		}
	}
	return ""
}

type fileTask struct {
	absPath   string
	relPath   string
	mtimeUnix float64
	exists    bool
}

type parseResult struct {
	note      *vault.Note
	mtimeUnix float64
	exists    bool
}

func isSourcesPath(relPath string) bool {
	clean := filepath.ToSlash(strings.TrimPrefix(filepath.Clean(relPath), "./"))
	return clean == "sources" || strings.HasPrefix(clean, "sources/")
}

func (e *Engine) getMetadata(ctx context.Context, targetDB *sql.DB, key string) (string, bool, error) {
	var val string
	err := targetDB.QueryRowContext(ctx, "SELECT value FROM index_metadata WHERE key = ?", key).Scan(&val)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return val, true, nil
}

func (e *Engine) setMetadata(ctx context.Context, targetDB *sql.DB, key, value string) error {
	_, err := targetDB.ExecContext(ctx, `
		INSERT INTO index_metadata(key, value)
		VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value;
	`, key, value)
	return err
}

// ShouldSync reports whether an incremental index sync should run based on last_sync_at and minAge.
func (e *Engine) ShouldSync(ctx context.Context, minAge time.Duration) (bool, time.Duration, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	val, found, err := e.getMetadata(ctx, e.db, metaKeyLastSync)
	if err != nil {
		return false, 0, err
	}
	if !found || strings.TrimSpace(val) == "" {
		return true, 0, nil
	}

	lastSync, parseErr := time.Parse(time.RFC3339Nano, val)
	if parseErr != nil {
		slog.Debug("invalid last_sync_at timestamp, forcing sync", "value", val, "error", parseErr)
		return true, 0, nil //nolint:nilerr // corrupt timestamp triggers self-healing sync
	}

	age := time.Since(lastSync)
	if age < 0 {
		age = 0
	}
	return age >= minAge, age, nil
}

// IndexVaultIfStale runs incremental IndexVault only if the index is older than minAge or never synced.
func (e *Engine) IndexVaultIfStale(ctx context.Context, dirs []string, minAge time.Duration) (indexed int, updated int, skipped bool, err error) {
	if ctx == nil {
		ctx = context.Background()
	}

	should, _, err := e.ShouldSync(ctx, minAge)
	if err != nil {
		return 0, 0, false, err
	}
	if !should {
		return 0, 0, true, nil
	}

	indexed, updated, err = e.IndexVault(ctx, dirs, false)
	return indexed, updated, false, err
}

func (e *Engine) migrateSourcesFromMainDB(ctx context.Context) error {
	val, found, err := e.getMetadata(ctx, e.db, metaKeySourcesMigrated)
	if err != nil {
		return err
	}
	if found && val == "1" {
		return nil
	}

	var count int
	err = e.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM notes_meta WHERE scope = 'sources'").Scan(&count)
	if err != nil {
		return err
	}

	if count > 0 {
		tx, err := e.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback() //nolint:errcheck // safe to ignore rollback error

		if _, err := tx.ExecContext(ctx, "DELETE FROM notes_fts WHERE rel_path IN (SELECT rel_path FROM notes_meta WHERE scope = 'sources')"); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM note_links WHERE source_rel_path IN (SELECT rel_path FROM notes_meta WHERE scope = 'sources')"); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM notes_meta WHERE scope = 'sources'"); err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}

		if _, err := e.db.ExecContext(ctx, "VACUUM"); err != nil {
			return err
		}
		slog.Debug("migrated legacy sources rows out of primary index", "removed_rows", count)
	}

	return e.setMetadata(ctx, e.db, metaKeySourcesMigrated, "1")
}

func loadExistingMTimes(ctx context.Context, targetDB *sql.DB) map[string]float64 {
	existing := make(map[string]float64)
	if targetDB == nil {
		return existing
	}
	rows, err := targetDB.QueryContext(ctx, "SELECT rel_path, mtime FROM notes_meta")
	if err != nil {
		return existing
	}
	defer rows.Close() //nolint:errcheck,gosec // fully read or safe to ignore
	for rows.Next() {
		var p string
		var mt float64
		if err := rows.Scan(&p, &mt); err == nil {
			existing[p] = mt
		}
	}
	return existing
}

func (e *Engine) writeBatchToDB(
	ctx context.Context,
	targetDB *sql.DB,
	results []parseResult,
	existingMTimes map[string]float64,
	currentFiles map[string]bool,
	scannedDirs []string,
) (indexed int, updated int, err error) {
	tx, err := targetDB.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback() //nolint:errcheck // safe to ignore rollback error

	stmtInsertMeta, err := tx.PrepareContext(ctx, `
		INSERT INTO notes_meta(rel_path, title, category, scope, note_type, description, tags, headings, aliases, mtime)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(rel_path) DO UPDATE SET
			title=excluded.title,
			category=excluded.category,
			scope=excluded.scope,
			note_type=excluded.note_type,
			description=excluded.description,
			tags=excluded.tags,
			headings=excluded.headings,
			aliases=excluded.aliases,
			mtime=excluded.mtime;
	`)
	if err != nil {
		return 0, 0, err
	}
	defer stmtInsertMeta.Close() //nolint:errcheck,gosec // fully read or safe to ignore

	stmtDelFTS, err := tx.PrepareContext(ctx, "DELETE FROM notes_fts WHERE rel_path = ?")
	if err != nil {
		return 0, 0, err
	}
	defer stmtDelFTS.Close() //nolint:errcheck,gosec // fully read or safe to ignore

	stmtInsertFTS, err := tx.PrepareContext(ctx, `
		INSERT INTO notes_fts(rel_path, title, category, scope, note_type, description, tags, headings, content)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);
	`)
	if err != nil {
		return 0, 0, err
	}
	defer stmtInsertFTS.Close() //nolint:errcheck,gosec // fully read or safe to ignore

	stmtDelLinks, err := tx.PrepareContext(ctx, "DELETE FROM note_links WHERE source_rel_path = ?")
	if err != nil {
		return 0, 0, err
	}
	defer stmtDelLinks.Close() //nolint:errcheck,gosec // fully read or safe to ignore

	stmtInsertLink, err := tx.PrepareContext(ctx, "INSERT INTO note_links(source_rel_path, target_rel_path, link_text) VALUES (?, ?, ?)")
	if err != nil {
		return 0, 0, err
	}
	defer stmtInsertLink.Close() //nolint:errcheck,gosec // fully read or safe to ignore

	stmtInsertPeople, err := tx.PrepareContext(ctx, `
		INSERT INTO people_meta(rel_path, name, role, squad, email, aliases, mtime)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(rel_path) DO UPDATE SET
			name=excluded.name,
			role=excluded.role,
			squad=excluded.squad,
			email=excluded.email,
			aliases=excluded.aliases,
			mtime=excluded.mtime;
	`)
	if err != nil {
		return 0, 0, err
	}
	defer stmtInsertPeople.Close() //nolint:errcheck,gosec

	stmtDelPeople, err := tx.PrepareContext(ctx, "DELETE FROM people_meta WHERE rel_path = ?")
	if err != nil {
		return 0, 0, err
	}
	defer stmtDelPeople.Close() //nolint:errcheck,gosec

	for _, res := range results {
		if err := ctx.Err(); err != nil {
			return 0, 0, err
		}

		n := res.note
		tagsStr := strings.Join(n.Tags, " ")
		headingsStr := strings.Join(n.Headings, " | ")
		aliasesStr := strings.Join(n.Aliases, " | ")

		_, err = stmtInsertMeta.ExecContext(ctx, n.RelPath, n.Title, n.Category, n.Scope, n.NoteType, n.Description, tagsStr, headingsStr, aliasesStr, res.mtimeUnix)
		if err != nil {
			continue
		}

		if res.exists {
			_, _ = stmtDelFTS.ExecContext(ctx, n.RelPath)
			_, _ = stmtDelLinks.ExecContext(ctx, n.RelPath)
			_, _ = stmtDelPeople.ExecContext(ctx, n.RelPath)
		}

		_, _ = stmtInsertFTS.ExecContext(ctx, n.RelPath, n.Title, n.Category, n.Scope, n.NoteType, n.Description, tagsStr, headingsStr, n.Body)

		for _, l := range n.Links {
			_, _ = stmtInsertLink.ExecContext(ctx, l.SourceRelPath, l.TargetRelPath, l.LinkText)
		}

		if n.NoteType == "person" || strings.HasPrefix(n.RelPath, "notes/pessoas/") || strings.HasPrefix(n.RelPath, "meta/pessoas/") {
			cleanName := cleanPersonName(n.Title)
			role := getExtraString(n.FM.Extra, "role", "cargo")
			squad := getExtraString(n.FM.Extra, "squad", "equipe", "time")
			email := getExtraString(n.FM.Extra, "email")
			_, _ = stmtInsertPeople.ExecContext(ctx, n.RelPath, cleanName, role, squad, email, aliasesStr, res.mtimeUnix)
		}

		if res.exists {
			updated++
		} else {
			indexed++
		}
	}

	if len(scannedDirs) > 0 {
		for p := range existingMTimes {
			if pathInScannedDirs(p, scannedDirs) && !currentFiles[p] {
				_, _ = tx.ExecContext(ctx, "DELETE FROM notes_meta WHERE rel_path = ?", p)
				_, _ = stmtDelFTS.ExecContext(ctx, p)
				_, _ = stmtDelLinks.ExecContext(ctx, p)
				_, _ = stmtDelPeople.ExecContext(ctx, p)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, 0, err
	}

	return indexed, updated, nil
}

// IndexVault scans the vault and updates the SQLite FTS5 databases incrementally.
func (e *Engine) IndexVault(ctx context.Context, indexedDirs []string, force bool) (indexed int, updated int, err error) {
	if ctx == nil {
		ctx = context.Background()
	}

	// 0. Verify vault root exists
	if stat, err := os.Stat(e.vaultRoot); err != nil || !stat.IsDir() {
		return 0, 0, fmt.Errorf("vault root directory does not exist or is not a directory: %s", e.vaultRoot)
	}

	// 0.5. One-time migration hygiene for legacy sources rows in primary DB
	if err := e.migrateSourcesFromMainDB(ctx); err != nil {
		return 0, 0, fmt.Errorf("failed to migrate legacy sources rows: %w", err)
	}

	dirs := indexedDirs
	if len(dirs) == 0 {
		dirs = []string{"."}
	}

	// Determine if any scanned directory could touch sources/
	needsSourcesCheck := false
	for _, dir := range dirs {
		cleanDir := filepath.ToSlash(strings.TrimPrefix(filepath.Clean(dir), "./"))
		if cleanDir == "." || cleanDir == "" || isSourcesPath(cleanDir) {
			if _, statErr := os.Stat(filepath.Join(e.vaultRoot, dir)); statErr == nil {
				needsSourcesCheck = true
				break
			}
		}
	}

	// 1. Load existing mtime maps from DB(s)
	existingMain := loadExistingMTimes(ctx, e.db)
	var existingSources map[string]float64
	var sourcesDB *sql.DB
	if needsSourcesCheck && (e.sourcesDB != nil || e.sourcesDBExistsOnDisk()) {
		var openErr error
		sourcesDB, openErr = e.ensureSourcesDB()
		if openErr != nil {
			return 0, 0, openErr
		}
		existingSources = loadExistingMTimes(ctx, sourcesDB)
	} else {
		existingSources = make(map[string]float64)
	}

	mainCurrentFiles := make(map[string]bool)
	sourcesCurrentFiles := make(map[string]bool)
	var tasks []fileTask
	var mainScannedDirs []string
	var sourcesScannedDirs []string

	// 2. Discover all files needing parsing
	for _, dir := range dirs {
		if err := ctx.Err(); err != nil {
			return 0, 0, err
		}

		absDir := filepath.Join(e.vaultRoot, dir)
		if _, err := os.Stat(absDir); os.IsNotExist(err) {
			continue
		}

		cleanDir := filepath.ToSlash(strings.TrimPrefix(filepath.Clean(dir), "./"))
		if cleanDir == "." || cleanDir == "" {
			mainScannedDirs = append(mainScannedDirs, dir)
			sourcesScannedDirs = append(sourcesScannedDirs, dir)
		} else if isSourcesPath(cleanDir) {
			sourcesScannedDirs = append(sourcesScannedDirs, dir)
		} else {
			mainScannedDirs = append(mainScannedDirs, dir)
		}

		_ = filepath.Walk(absDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				base := info.Name()
				if base == ".git" || base == ".dev" || base == "node_modules" || base == ".obsidian" || base == ".agents" || base == ".daneel" || base == "vendor" || base == "dist" || base == "build" {
					return filepath.SkipDir
				}
				return nil
			}

			if !strings.HasSuffix(strings.ToLower(info.Name()), ".md") {
				return nil
			}

			relPath, err := filepath.Rel(e.vaultRoot, path)
			if err != nil {
				relPath = path
			}
			relPath = filepath.ToSlash(relPath)

			mtimeUnix := float64(info.ModTime().UnixNano()) / 1e9
			var dbMTime float64
			var exists bool
			if isSourcesPath(relPath) {
				sourcesCurrentFiles[relPath] = true
				dbMTime, exists = existingSources[relPath]
			} else {
				mainCurrentFiles[relPath] = true
				dbMTime, exists = existingMain[relPath]
			}

			// If file hasn't changed, skip parsing!
			if !force && exists && (dbMTime == mtimeUnix || int64(dbMTime) == int64(mtimeUnix)) {
				return nil
			}

			tasks = append(tasks, fileTask{
				absPath:   path,
				relPath:   relPath,
				mtimeUnix: mtimeUnix,
				exists:    exists,
			})
			return nil
		})
	}

	if err := ctx.Err(); err != nil {
		return 0, 0, err
	}

	// 3. Concurrently parse notes using worker pool
	numWorkers := runtime.NumCPU()
	if numWorkers > 16 {
		numWorkers = 16
	}
	if numWorkers < 1 {
		numWorkers = 1
	}

	taskCh := make(chan fileTask, len(tasks))
	resCh := make(chan parseResult, len(tasks))

	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case t, ok := <-taskCh:
					if !ok {
						return
					}
					n, err := vault.ParseNote(t.absPath, e.vaultRoot)
					if err == nil && n != nil {
						select {
						case <-ctx.Done():
							return
						case resCh <- parseResult{note: n, mtimeUnix: t.mtimeUnix, exists: t.exists}:
						}
					}
				}
			}
		}()
	}

	for _, t := range tasks {
		taskCh <- t
	}
	close(taskCh)

	go func() {
		wg.Wait()
		close(resCh)
	}()

	var mainResults []parseResult
	var sourcesResults []parseResult
	for res := range resCh {
		if err := ctx.Err(); err != nil {
			return 0, 0, err
		}
		if isSourcesPath(res.note.RelPath) {
			sourcesResults = append(sourcesResults, res)
		} else {
			mainResults = append(mainResults, res)
		}
	}

	// 4. Batch insert into primary DB (non-sources)
	if len(mainResults) > 0 || len(mainScannedDirs) > 0 {
		mIdx, mUpd, err := e.writeBatchToDB(ctx, e.db, mainResults, existingMain, mainCurrentFiles, mainScannedDirs)
		if err != nil {
			return 0, 0, err
		}
		indexed += mIdx
		updated += mUpd
	}

	// 5. Batch insert into isolated sources DB
	if len(sourcesResults) > 0 || (len(sourcesScannedDirs) > 0 && sourcesDB != nil) {
		if sourcesDB == nil {
			sourcesDB, err = e.ensureSourcesDB()
			if err != nil {
				return 0, 0, err
			}
		}
		sIdx, sUpd, err := e.writeBatchToDB(ctx, sourcesDB, sourcesResults, existingSources, sourcesCurrentFiles, sourcesScannedDirs)
		if err != nil {
			return 0, 0, err
		}
		indexed += sIdx
		updated += sUpd
	}

	nowStr := time.Now().UTC().Format(time.RFC3339Nano)
	if err := e.setMetadata(ctx, e.db, metaKeyLastSync, nowStr); err != nil {
		return 0, 0, fmt.Errorf("failed to persist last_sync_at: %w", err)
	}
	if sourcesDB != nil {
		_ = e.setMetadata(ctx, sourcesDB, metaKeyLastSync, nowStr)
	}

	slog.Debug("vault indexing completed", "indexed", indexed, "updated", updated)
	return indexed, updated, nil
}

// IndexNote parses and upserts a single note into SQLite FTS5 in sub-millisecond time.
func (e *Engine) IndexNote(ctx context.Context, relPath string) error {
	if ctx == nil {
		ctx = context.Background()
	}

	absPath := filepath.Join(e.vaultRoot, relPath)
	info, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("file not found: %w", err)
	}

	note, err := vault.ParseNote(absPath, e.vaultRoot)
	if err != nil {
		return fmt.Errorf("failed to parse note: %w", err)
	}

	targetDB := e.db
	if isSourcesPath(note.RelPath) {
		targetDB, err = e.ensureSourcesDB()
		if err != nil {
			return err
		}
	}

	mtimeUnix := float64(info.ModTime().UnixNano()) / 1e9

	tx, err := targetDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // safe to ignore rollback error

	aliasesStr := strings.Join(note.Aliases, " | ")

	// 1. Upsert Meta
	_, err = tx.ExecContext(ctx, `
		INSERT INTO notes_meta(rel_path, title, category, scope, note_type, description, tags, headings, aliases, mtime)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(rel_path) DO UPDATE SET
			title=excluded.title,
			category=excluded.category,
			scope=excluded.scope,
			note_type=excluded.note_type,
			description=excluded.description,
			tags=excluded.tags,
			headings=excluded.headings,
			aliases=excluded.aliases,
			mtime=excluded.mtime;
	`, note.RelPath, note.Title, note.Category, note.Scope, note.NoteType, note.Description,
		strings.Join(note.Tags, ","), strings.Join(note.Headings, "|"), aliasesStr, mtimeUnix)
	if err != nil {
		return err
	}

	if note.NoteType == "person" || strings.HasPrefix(note.RelPath, "notes/pessoas/") || strings.HasPrefix(note.RelPath, "meta/pessoas/") {
		cleanName := cleanPersonName(note.Title)
		role := getExtraString(note.FM.Extra, "role", "cargo")
		squad := getExtraString(note.FM.Extra, "squad", "equipe", "time")
		email := getExtraString(note.FM.Extra, "email")
		_, _ = tx.ExecContext(ctx, `
			INSERT INTO people_meta(rel_path, name, role, squad, email, aliases, mtime)
			VALUES (?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(rel_path) DO UPDATE SET
				name=excluded.name,
				role=excluded.role,
				squad=excluded.squad,
				email=excluded.email,
				aliases=excluded.aliases,
				mtime=excluded.mtime;
		`, note.RelPath, cleanName, role, squad, email, aliasesStr, mtimeUnix)
	}

	// 2. Refresh FTS
	_, _ = tx.ExecContext(ctx, "DELETE FROM notes_fts WHERE rel_path = ?", note.RelPath)
	_, err = tx.ExecContext(ctx, `
		INSERT INTO notes_fts(rel_path, title, category, scope, note_type, description, tags, headings, content)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);
	`, note.RelPath, note.Title, note.Category, note.Scope, note.NoteType, note.Description,
		strings.Join(note.Tags, " "), strings.Join(note.Headings, " "), note.Content)
	if err != nil {
		return err
	}

	// 3. Refresh Links
	_, _ = tx.ExecContext(ctx, "DELETE FROM note_links WHERE source_rel_path = ?", note.RelPath)
	for _, l := range note.Links {
		_, _ = tx.ExecContext(ctx, `
			INSERT INTO note_links(source_rel_path, target_rel_path, link_text)
			VALUES (?, ?, ?)
		`, note.RelPath, l.TargetRelPath, l.LinkText)
	}

	return tx.Commit()
}

// SearchOptions holds parameters for faceted vault searches.
type SearchOptions struct {
	Query    string `json:"query"`
	Scope    string `json:"scope"`
	NoteType string `json:"note_type"`
	Tag      string `json:"tag"`
	Limit    int    `json:"limit"`
}

// Search executes a full-text query with BM25 ranking and scope filtering.
func (e *Engine) Search(ctx context.Context, queryStr string, scope string, limit int) ([]SearchResult, error) {
	return e.SearchFaceted(ctx, SearchOptions{
		Query: queryStr,
		Scope: scope,
		Limit: limit,
	})
}

// SearchFaceted executes a full-text search with faceted filters (type, tag, scope).
func (e *Engine) SearchFaceted(ctx context.Context, opts SearchOptions) ([]SearchResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = 10
	}

	scope := strings.TrimSpace(opts.Scope)

	if scope == "sources" {
		if e.sourcesDB == nil && !e.sourcesDBExistsOnDisk() {
			return nil, nil
		}
		sdb, err := e.ensureSourcesDB()
		if err != nil {
			return nil, err
		}
		return searchDB(ctx, sdb, opts, limit)
	}

	results, err := searchDB(ctx, e.db, opts, limit)
	if err != nil {
		return nil, err
	}

	if (scope == "all" || scope == "") && len(results) < limit {
		if e.sourcesDB != nil || e.sourcesDBExistsOnDisk() {
			sdb, err := e.ensureSourcesDB()
			if err != nil {
				return nil, err
			}
			remaining := limit - len(results)
			sourceOpts := opts
			sourceOpts.Scope = "sources"
			sourceResults, err := searchDB(ctx, sdb, sourceOpts, remaining)
			if err != nil {
				return nil, err
			}
			results = append(results, sourceResults...)
		}
	}

	return results, nil
}

func searchDB(ctx context.Context, targetDB *sql.DB, opts SearchOptions, limit int) ([]SearchResult, error) {
	trimmedQuery := strings.TrimSpace(opts.Query)
	scope := strings.TrimSpace(opts.Scope)
	noteType := strings.TrimSpace(opts.NoteType)
	tag := strings.TrimSpace(opts.Tag)

	wordReg := regexp.MustCompile(`[a-zA-Z0-9_\-]+`)
	words := wordReg.FindAllString(trimmedQuery, -1)

	var queryBuilder strings.Builder
	var args []interface{}

	if len(words) > 0 {
		termList := make([]string, 0, len(words))
		for _, w := range words {
			clean := strings.ReplaceAll(w, `"`, "")
			termList = append(termList, fmt.Sprintf(`"%s"*`, clean))
		}
		ftsQuery := strings.Join(termList, " OR ")

		queryBuilder.WriteString(`
			SELECT m.rel_path, m.title, m.category, m.scope, m.note_type, m.description,
			       snippet(notes_fts, 8, '==', '==', '...', 20) as snip,
			       bm25(notes_fts, 5.0, 10.0, 2.0, 1.0, 1.0, 5.0, 3.0, 2.0, 1.0) as rank
			FROM notes_fts f
			JOIN notes_meta m ON f.rel_path = m.rel_path
			WHERE notes_fts MATCH ?
		`)
		args = append(args, ftsQuery)

		if scope != "" && scope != "all" {
			queryBuilder.WriteString(" AND m.scope = ?")
			args = append(args, scope)
		}
		if noteType != "" {
			queryBuilder.WriteString(" AND LOWER(m.note_type) = LOWER(?)")
			args = append(args, noteType)
		}
		if tag != "" {
			cleanTag := strings.TrimPrefix(tag, "#")
			queryBuilder.WriteString(" AND (m.tags LIKE ? OR m.tags LIKE ? OR m.tags LIKE ? OR m.tags = ?)")
			args = append(args, "% "+cleanTag+" %", cleanTag+" %", "% "+cleanTag, cleanTag)
		}

		// Prioritize tech scope when searching globally
		if scope == "all" || scope == "" {
			queryBuilder.WriteString(`
				ORDER BY 
					CASE WHEN m.scope = 'tech' THEN 0 
					     WHEN m.scope = 'meta' THEN 1 
					     WHEN m.scope = 'daily' THEN 2 
					     ELSE 3 END ASC,
					rank ASC
			`)
		} else {
			queryBuilder.WriteString(" ORDER BY rank ASC")
		}
	} else {
		// Faceted metadata-only query (e.g. daneel search --type rfc, daneel search --tag kafka)
		if scope == "" && noteType == "" && tag == "" {
			return nil, nil
		}

		queryBuilder.WriteString(`
			SELECT m.rel_path, m.title, m.category, m.scope, m.note_type, m.description,
			       '' as snip,
			       0.0 as rank
			FROM notes_meta m
			WHERE 1=1
		`)

		if scope != "" && scope != "all" {
			queryBuilder.WriteString(" AND m.scope = ?")
			args = append(args, scope)
		}
		if noteType != "" {
			queryBuilder.WriteString(" AND LOWER(m.note_type) = LOWER(?)")
			args = append(args, noteType)
		}
		if tag != "" {
			cleanTag := strings.TrimPrefix(tag, "#")
			queryBuilder.WriteString(" AND (m.tags LIKE ? OR m.tags LIKE ? OR m.tags LIKE ? OR m.tags = ?)")
			args = append(args, "% "+cleanTag+" %", cleanTag+" %", "% "+cleanTag, cleanTag)
		}

		queryBuilder.WriteString(" ORDER BY m.mtime DESC")
	}

	queryBuilder.WriteString(" LIMIT ?")
	args = append(args, limit)

	rows, err := targetDB.QueryContext(ctx, queryBuilder.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("search query failed: %w", err)
	}
	defer rows.Close() //nolint:errcheck,gosec // fully read or safe to ignore

	var results []SearchResult
	for rows.Next() {
		var res SearchResult
		var snip sql.NullString
		if err := rows.Scan(&res.RelPath, &res.Title, &res.Category, &res.Scope, &res.NoteType, &res.Description, &snip, &res.Score); err != nil {
			continue
		}
		if snip.Valid {
			res.Snippet = snip.String
		}
		res.Path = res.RelPath
		res.Type = res.NoteType
		res.Desc = res.Description
		results = append(results, res)
	}

	return results, nil
}

// GetNote retrieves a single note's metadata by its relative path.
func (e *Engine) GetNote(relPath string) (*NoteMeta, error) {
	targetDB := e.db
	if isSourcesPath(relPath) {
		if e.sourcesDB == nil && !e.sourcesDBExistsOnDisk() {
			return nil, sql.ErrNoRows
		}
		var err error
		targetDB, err = e.ensureSourcesDB()
		if err != nil {
			return nil, err
		}
	}

	row := targetDB.QueryRow(`
		SELECT rel_path, title, category, scope, note_type, description, tags, headings, COALESCE(aliases, ''), mtime
		FROM notes_meta WHERE rel_path = ?
	`, relPath)

	var meta NoteMeta
	var mtimeUnix float64
	err := row.Scan(&meta.RelPath, &meta.Title, &meta.Category, &meta.Scope, &meta.NoteType, &meta.Description, &meta.Tags, &meta.Headings, &meta.Aliases, &mtimeUnix)
	if err != nil {
		return nil, err
	}
	meta.MTime = time.Unix(int64(mtimeUnix), int64((mtimeUnix-float64(int64(mtimeUnix)))*1e9))
	return &meta, nil
}

func queryBacklinksFromDB(targetDB *sql.DB, targetRelPath string) ([]vault.Link, error) {
	rows, err := targetDB.Query(`
		SELECT source_rel_path, target_rel_path, link_text
		FROM note_links
		WHERE target_rel_path = ? OR target_rel_path LIKE ?
	`, targetRelPath, "%"+targetRelPath)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck,gosec // fully read or safe to ignore

	var links []vault.Link
	for rows.Next() {
		var l vault.Link
		if err := rows.Scan(&l.SourceRelPath, &l.TargetRelPath, &l.LinkText); err == nil {
			links = append(links, l)
		}
	}
	return links, nil
}

// GetBacklinks returns all incoming links pointing to targetRelPath.
func (e *Engine) GetBacklinks(targetRelPath string) ([]vault.Link, error) {
	links, err := queryBacklinksFromDB(e.db, targetRelPath)
	if err != nil {
		return nil, err
	}

	if e.sourcesDB != nil || e.sourcesDBExistsOnDisk() {
		sdb, err := e.ensureSourcesDB()
		if err != nil {
			return nil, err
		}
		sLinks, err := queryBacklinksFromDB(sdb, targetRelPath)
		if err != nil {
			return nil, err
		}
		links = append(links, sLinks...)
	}

	return links, nil
}

func aggregateHealthFromDB(targetDB *sql.DB, report *HealthReport) {
	var notesCnt, linksCnt int
	row := targetDB.QueryRow("SELECT count(*) FROM notes_meta")
	_ = row.Scan(&notesCnt)
	report.TotalNotes += notesCnt

	row = targetDB.QueryRow("SELECT count(*) FROM note_links")
	_ = row.Scan(&linksCnt)
	report.TotalLinks += linksCnt

	rows, err := targetDB.Query("SELECT scope, count(*) FROM notes_meta GROUP BY scope")
	if err == nil {
		for rows.Next() {
			var sc string
			var cnt int
			if err := rows.Scan(&sc, &cnt); err == nil {
				report.NotesByScope[sc] += cnt
			}
		}
		rows.Close() //nolint:errcheck,gosec // fully read or safe to ignore
	}
}

// HealthCheck inspects total counts and checks for broken references.
func (e *Engine) HealthCheck() (*HealthReport, error) {
	report := &HealthReport{
		NotesByScope: make(map[string]int),
	}

	aggregateHealthFromDB(e.db, report)
	if e.sourcesDB != nil || e.sourcesDBExistsOnDisk() {
		sdb, err := e.ensureSourcesDB()
		if err == nil {
			aggregateHealthFromDB(sdb, report)
		}
	}

	return report, nil
}

func pathInScannedDirs(relPath string, dirs []string) bool {
	for _, d := range dirs {
		if d == "." || d == "" || relPath == d || strings.HasPrefix(relPath, strings.TrimSuffix(d, "/")+"/") {
			return true
		}
	}
	return false
}
