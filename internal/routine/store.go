package routine

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"

	"github.com/rjfonseca/daneel/internal/config"
)

// DefaultMaxRunsRetention defines how many recent execution records are kept in SQLite.
const DefaultMaxRunsRetention = 500

// RoutineRun represents a single execution of a scheduled or manual routine.
type RoutineRun struct {
	ID            string `json:"id"`
	RoutineName   string `json:"routine_name"`
	StartedAt     int64  `json:"started_at"`     // Unix timestamp in seconds
	CompletedAt   int64  `json:"completed_at"`   // Unix timestamp in seconds
	DurationMs    int64  `json:"duration_ms"`    // Duration in milliseconds
	TriggerSource string `json:"trigger_source"` // systemd, cron, web_cockpit, cli, mcp
	ExitCode      int    `json:"exit_code"`
	Status        string `json:"status"` // success, failed, running, canceled
	Summary       string `json:"summary"`
	ErrorMessage  string `json:"error_message,omitempty"`
}

// Store defines the persistent ledger for routine executions.
type Store interface {
	RecordRun(ctx context.Context, run RoutineRun) error
	GetHistory(ctx context.Context, routineName string, limit int) ([]RoutineRun, error)
	GetLatestRun(ctx context.Context, routineName string) (*RoutineRun, error)
	Close() error
}

// SQLiteStore implements Store using SQLite (modernc.org/sqlite, CGO-free).
type SQLiteStore struct {
	mu             sync.RWMutex
	db             *sql.DB
	dbPath         string
	retentionLimit int
}

// OpenStore opens or creates the SQLite execution ledger database at dbPath.
// If the path cannot be written to, it gracefully falls back to the user's XDG cache directory.
func OpenStore(dbPath string) (*SQLiteStore, error) {
	if dbPath == "" {
		dbPath = filepath.Join(config.XDGCacheDir(), "routine_runs.db")
	}

	store, err := tryOpenSQLite(dbPath)
	if err != nil && dbPath != ":memory:" {
		xdgPath := filepath.Join(config.XDGCacheDir(), "routine_runs.db")
		if xdgPath != dbPath {
			var fallbackErr error
			store, fallbackErr = tryOpenSQLite(xdgPath)
			if fallbackErr != nil {
				return nil, fmt.Errorf("failed to open routine store at %s and fallback %s: %w", dbPath, xdgPath, err)
			}
		} else {
			return nil, err
		}
	}

	return store, nil
}

// OpenDefaultStore resolves the cache directory for vaultDir and opens routine_runs.db.
func OpenDefaultStore(vaultDir string) (*SQLiteStore, error) {
	if vaultDir != "" {
		localCache := filepath.Join(vaultDir, ".daneel", "cache")
		if err := os.MkdirAll(localCache, 0750); err == nil {
			return OpenStore(filepath.Join(localCache, "routine_runs.db"))
		}
	}
	cacheDir := config.ResolveVaultCacheDir(vaultDir)
	dbPath := filepath.Join(cacheDir, "routine_runs.db")
	return OpenStore(dbPath)
}

func tryOpenSQLite(dbPath string) (*SQLiteStore, error) {
	if dbPath != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(dbPath), 0750); err != nil {
			return nil, fmt.Errorf("failed to create routine store directory: %w", err)
		}
	}

	dsn := dbPath
	if dbPath != ":memory:" {
		dsn = fmt.Sprintf("%s?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(10000)", dbPath)
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	store := &SQLiteStore{
		db:             db,
		dbPath:         dbPath,
		retentionLimit: DefaultMaxRunsRetention,
	}

	if err := store.initSchema(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to initialize routine schema: %w", err)
	}

	return store, nil
}

func (s *SQLiteStore) initSchema() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS routine_runs (
			id TEXT PRIMARY KEY,
			routine_name TEXT NOT NULL,
			started_at INTEGER NOT NULL,
			completed_at INTEGER NOT NULL,
			duration_ms INTEGER NOT NULL,
			trigger_source TEXT NOT NULL,
			exit_code INTEGER NOT NULL,
			status TEXT NOT NULL,
			summary TEXT,
			error_message TEXT
		);`,
		`CREATE INDEX IF NOT EXISTS idx_routine_runs_started ON routine_runs(started_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_routine_runs_name_started ON routine_runs(routine_name, started_at DESC);`,
	}

	for _, q := range queries {
		if _, err := s.db.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

// SetRetentionLimit overrides the maximum number of historical records to keep.
func (s *SQLiteStore) SetRetentionLimit(limit int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if limit > 0 {
		s.retentionLimit = limit
	}
}

// RecordRun writes a new execution event to the ledger and prunes old records.
func (s *SQLiteStore) RecordRun(ctx context.Context, run RoutineRun) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if run.ID == "" {
		run.ID = fmt.Sprintf("run-%d-%d", run.StartedAt, time.Now().UnixNano()%1000000)
	}
	if run.StartedAt == 0 {
		run.StartedAt = time.Now().Unix()
	}
	if run.CompletedAt == 0 {
		run.CompletedAt = time.Now().Unix()
	}
	if run.DurationMs == 0 && run.CompletedAt >= run.StartedAt {
		run.DurationMs = (run.CompletedAt - run.StartedAt) * 1000
	}
	if run.Status == "" {
		if run.ExitCode == 0 {
			run.Status = "success"
		} else {
			run.Status = "failed"
		}
	}
	if run.TriggerSource == "" {
		run.TriggerSource = "cli"
	}

	query := `INSERT OR REPLACE INTO routine_runs (
		id, routine_name, started_at, completed_at, duration_ms, trigger_source, exit_code, status, summary, error_message
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`

	_, err := s.db.ExecContext(ctx, query,
		run.ID,
		run.RoutineName,
		run.StartedAt,
		run.CompletedAt,
		run.DurationMs,
		run.TriggerSource,
		run.ExitCode,
		run.Status,
		run.Summary,
		run.ErrorMessage,
	)
	if err != nil {
		return fmt.Errorf("failed to record routine run: %w", err)
	}

	// Automated retention prune
	pruneQuery := `DELETE FROM routine_runs WHERE id NOT IN (
		SELECT id FROM routine_runs ORDER BY started_at DESC LIMIT ?
	);`
	_, _ = s.db.ExecContext(ctx, pruneQuery, s.retentionLimit)

	return nil
}

// GetHistory retrieves execution history ordered by started_at DESC, optionally filtered by routineName.
func (s *SQLiteStore) GetHistory(ctx context.Context, routineName string, limit int) ([]RoutineRun, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 30
	}
	if limit > 100 {
		limit = 100
	}

	var rows *sql.Rows
	var err error

	routineName = strings.TrimSpace(routineName)
	if routineName != "" {
		query := `SELECT id, routine_name, started_at, completed_at, duration_ms, trigger_source, exit_code, status, summary, error_message
			FROM routine_runs
			WHERE routine_name = ?
			ORDER BY started_at DESC
			LIMIT ?;`
		rows, err = s.db.QueryContext(ctx, query, routineName, limit)
	} else {
		query := `SELECT id, routine_name, started_at, completed_at, duration_ms, trigger_source, exit_code, status, summary, error_message
			FROM routine_runs
			ORDER BY started_at DESC
			LIMIT ?;`
		rows, err = s.db.QueryContext(ctx, query, limit)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to query routine runs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var list []RoutineRun
	for rows.Next() {
		var r RoutineRun
		var summary, errorMsg sql.NullString
		if err := rows.Scan(
			&r.ID,
			&r.RoutineName,
			&r.StartedAt,
			&r.CompletedAt,
			&r.DurationMs,
			&r.TriggerSource,
			&r.ExitCode,
			&r.Status,
			&summary,
			&errorMsg,
		); err != nil {
			return nil, fmt.Errorf("failed to scan routine run: %w", err)
		}
		if summary.Valid {
			r.Summary = summary.String
		}
		if errorMsg.Valid {
			r.ErrorMessage = errorMsg.String
		}
		list = append(list, r)
	}

	return list, rows.Err()
}

// GetLatestRun returns the most recent execution record for a given routine.
func (s *SQLiteStore) GetLatestRun(ctx context.Context, routineName string) (*RoutineRun, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `SELECT id, routine_name, started_at, completed_at, duration_ms, trigger_source, exit_code, status, summary, error_message
		FROM routine_runs
		WHERE routine_name = ?
		ORDER BY started_at DESC
		LIMIT 1;`

	row := s.db.QueryRowContext(ctx, query, strings.TrimSpace(routineName))
	var r RoutineRun
	var summary, errorMsg sql.NullString
	err := row.Scan(
		&r.ID,
		&r.RoutineName,
		&r.StartedAt,
		&r.CompletedAt,
		&r.DurationMs,
		&r.TriggerSource,
		&r.ExitCode,
		&r.Status,
		&summary,
		&errorMsg,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get latest routine run: %w", err)
	}
	if summary.Valid {
		r.Summary = summary.String
	}
	if errorMsg.Valid {
		r.ErrorMessage = errorMsg.String
	}
	return &r, nil
}

// Close closes the underlying database connection.
func (s *SQLiteStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}
