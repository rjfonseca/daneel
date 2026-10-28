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
const (
	DefaultMaxRunsRetention      = 500
	DefaultMaxCmdEventsRetention = 5000
)

// RoutineRun represents a single execution of a scheduled or manual routine.
type RoutineRun struct {
	ID               string  `json:"id"`
	RoutineName      string  `json:"routine_name"`
	StartedAt        int64   `json:"started_at"`     // Unix timestamp in seconds
	CompletedAt      int64   `json:"completed_at"`   // Unix timestamp in seconds
	DurationMs       int64   `json:"duration_ms"`    // Duration in milliseconds
	TriggerSource    string  `json:"trigger_source"` // systemd, cron, web_cockpit, cli, mcp
	ExitCode         int     `json:"exit_code"`
	Status           string  `json:"status"` // success, failed, running, canceled
	Summary          string  `json:"summary"`
	ErrorMessage     string  `json:"error_message,omitempty"`
	ItemsProcessed   int     `json:"items_processed"`
	InputTokens      int64   `json:"input_tokens"`
	CacheReadTokens  int64   `json:"cache_read_tokens"`
	OutputTokens     int64   `json:"output_tokens"`
	EstimatedCostUSD float64 `json:"estimated_cost_usd"`
	Model            string  `json:"model,omitempty"`
	SessionID        string  `json:"session_id,omitempty"`
}

// CommandTelemetryEvent represents a single CLI or MCP tool execution event.
type CommandTelemetryEvent struct {
	ID         int64  `json:"id"`
	Surface    string `json:"surface"`    // "cli" or "mcp"
	Command    string `json:"command"`    // e.g. "resolve", "search", "dream"
	Scope      string `json:"scope"`      // optional scope or args summary
	StartedAt  int64  `json:"started_at"` // Unix timestamp in seconds
	DurationMs int64  `json:"duration_ms"`
	IsError    bool   `json:"is_error"`
}

// CommandTelemetryAgg represents aggregated usage metrics for a command surface.
type CommandTelemetryAgg struct {
	Surface       string  `json:"surface"`
	Command       string  `json:"command"`
	TotalCalls    int     `json:"total_calls"`
	ErrorCalls    int     `json:"error_calls"`
	AvgDurationMs float64 `json:"avg_duration_ms"`
	MaxDurationMs int64   `json:"max_duration_ms"`
	LastCalledAt  int64   `json:"last_called_at"`
}

// SessionTelemetryRecord captures high-level telemetry for an AI agent session.
type SessionTelemetryRecord struct {
	SessionID        string  `json:"session_id"`
	ParentSessionID  string  `json:"parent_session_id,omitempty"`
	Agent            string  `json:"agent"` // "antigravity", "claude-code", "codex", "cursor"
	Model            string  `json:"model"`
	Role             string  `json:"role,omitempty"`
	StartedAt        int64   `json:"started_at"`
	UpdatedAt        int64   `json:"updated_at"`
	TurnsCount       int     `json:"turns_count"`
	SubagentsCount   int     `json:"subagents_count"`
	InputTokens      int64   `json:"input_tokens"`
	CacheReadTokens  int64   `json:"cache_read_tokens"`
	OutputTokens     int64   `json:"output_tokens"`
	EstimatedCostUSD float64 `json:"estimated_cost_usd"`
	SkillsUsed       string  `json:"skills_used,omitempty"` // comma-separated skills/tools
	IsRoutine        bool    `json:"is_routine"`
	RoutineName      string  `json:"routine_name,omitempty"`
}

// RoutineWeightAgg represents aggregated resource usage and cost for background routines.
type RoutineWeightAgg struct {
	RoutineName       string  `json:"routine_name"`
	TotalRuns         int     `json:"total_runs"`
	SuccessRuns       int     `json:"success_runs"`
	FailedRuns        int     `json:"failed_runs"`
	AvgDurationMs     float64 `json:"avg_duration_ms"`
	MaxDurationMs     int64   `json:"max_duration_ms"`
	TotalItems        int64   `json:"total_items"`
	TotalInputTokens  int64   `json:"total_input_tokens"`
	TotalCacheTokens  int64   `json:"total_cache_tokens"`
	TotalOutputTokens int64   `json:"total_output_tokens"`
	TotalCostUSD      float64 `json:"total_cost_usd"`
	LastRunAt         int64   `json:"last_run_at"`
}

// ModelUsageAgg aggregates token consumption and estimated costs by model and agent.
type ModelUsageAgg struct {
	Model            string  `json:"model"`
	Agent            string  `json:"agent"`
	SessionsCount    int     `json:"sessions_count"`
	SubagentsCount   int     `json:"subagents_count"`
	InputTokens      int64   `json:"input_tokens"`
	CacheReadTokens  int64   `json:"cache_read_tokens"`
	OutputTokens     int64   `json:"output_tokens"`
	EstimatedCostUSD float64 `json:"estimated_cost_usd"`
}

// Store defines the persistent ledger for routine executions and operational telemetry.
type Store interface {
	RecordRun(ctx context.Context, run RoutineRun) error
	GetHistory(ctx context.Context, routineName string, limit int) ([]RoutineRun, error)
	GetLatestRun(ctx context.Context, routineName string) (*RoutineRun, error)
	RecordCommandEvent(ctx context.Context, ev CommandTelemetryEvent) error
	GetCommandAggregates(ctx context.Context, sinceUnix int64, limit int) ([]CommandTelemetryAgg, error)
	UpsertSessionTelemetry(ctx context.Context, rec SessionTelemetryRecord) error
	GetRecentSessionsTelemetry(ctx context.Context, sinceUnix int64, limit int) ([]SessionTelemetryRecord, error)
	GetRoutineWeightAggregates(ctx context.Context, sinceUnix int64) ([]RoutineWeightAgg, error)
	GetModelUsageAggregates(ctx context.Context, sinceUnix int64) ([]ModelUsageAgg, error)
	Close() error
}

// SQLiteStore implements Store using SQLite (modernc.org/sqlite, CGO-free).
type SQLiteStore struct {
	mu                sync.RWMutex
	db                *sql.DB
	dbPath            string
	retentionLimit    int
	cmdRetentionLimit int
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
		db:                db,
		dbPath:            dbPath,
		retentionLimit:    DefaultMaxRunsRetention,
		cmdRetentionLimit: DefaultMaxCmdEventsRetention,
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
			error_message TEXT,
			items_processed INTEGER NOT NULL DEFAULT 0,
			input_tokens INTEGER NOT NULL DEFAULT 0,
			cache_read_tokens INTEGER NOT NULL DEFAULT 0,
			output_tokens INTEGER NOT NULL DEFAULT 0,
			estimated_cost_usd REAL NOT NULL DEFAULT 0,
			model TEXT NOT NULL DEFAULT '',
			session_id TEXT NOT NULL DEFAULT ''
		);`,
		`CREATE INDEX IF NOT EXISTS idx_routine_runs_started ON routine_runs(started_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_routine_runs_name_started ON routine_runs(routine_name, started_at DESC);`,

		`CREATE TABLE IF NOT EXISTS command_telemetry (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			surface TEXT NOT NULL,
			command TEXT NOT NULL,
			scope TEXT NOT NULL DEFAULT '',
			started_at INTEGER NOT NULL,
			duration_ms INTEGER NOT NULL,
			is_error INTEGER NOT NULL DEFAULT 0
		);`,
		`CREATE INDEX IF NOT EXISTS idx_cmd_telemetry_started ON command_telemetry(started_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_cmd_telemetry_surface_command ON command_telemetry(surface, command);`,

		`CREATE TABLE IF NOT EXISTS session_telemetry (
			session_id TEXT PRIMARY KEY,
			parent_session_id TEXT NOT NULL DEFAULT '',
			agent TEXT NOT NULL,
			model TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT '',
			started_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL,
			turns_count INTEGER NOT NULL DEFAULT 0,
			subagents_count INTEGER NOT NULL DEFAULT 0,
			input_tokens INTEGER NOT NULL DEFAULT 0,
			cache_read_tokens INTEGER NOT NULL DEFAULT 0,
			output_tokens INTEGER NOT NULL DEFAULT 0,
			estimated_cost_usd REAL NOT NULL DEFAULT 0,
			skills_used TEXT NOT NULL DEFAULT '',
			is_routine INTEGER NOT NULL DEFAULT 0,
			routine_name TEXT NOT NULL DEFAULT ''
		);`,
		`CREATE INDEX IF NOT EXISTS idx_session_telemetry_started ON session_telemetry(started_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_session_telemetry_parent ON session_telemetry(parent_session_id);`,
	}

	for _, q := range queries {
		if _, err := s.db.Exec(q); err != nil {
			return err
		}
	}

	// Idempotent migrations for existing routine_runs tables
	alterCols := []string{
		"ALTER TABLE routine_runs ADD COLUMN items_processed INTEGER NOT NULL DEFAULT 0;",
		"ALTER TABLE routine_runs ADD COLUMN input_tokens INTEGER NOT NULL DEFAULT 0;",
		"ALTER TABLE routine_runs ADD COLUMN cache_read_tokens INTEGER NOT NULL DEFAULT 0;",
		"ALTER TABLE routine_runs ADD COLUMN output_tokens INTEGER NOT NULL DEFAULT 0;",
		"ALTER TABLE routine_runs ADD COLUMN estimated_cost_usd REAL NOT NULL DEFAULT 0;",
		"ALTER TABLE routine_runs ADD COLUMN model TEXT NOT NULL DEFAULT '';",
		"ALTER TABLE routine_runs ADD COLUMN session_id TEXT NOT NULL DEFAULT '';",
	}
	for _, stmt := range alterCols {
		if _, err := s.db.Exec(stmt); err != nil {
			if !strings.Contains(strings.ToLower(err.Error()), "duplicate column name") {
				return err
			}
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

// SetCommandRetentionLimit overrides the maximum number of command telemetry events to keep.
func (s *SQLiteStore) SetCommandRetentionLimit(limit int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if limit > 0 {
		s.cmdRetentionLimit = limit
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
		id, routine_name, started_at, completed_at, duration_ms, trigger_source, exit_code, status, summary, error_message,
		items_processed, input_tokens, cache_read_tokens, output_tokens, estimated_cost_usd, model, session_id
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`

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
		run.ItemsProcessed,
		run.InputTokens,
		run.CacheReadTokens,
		run.OutputTokens,
		run.EstimatedCostUSD,
		run.Model,
		run.SessionID,
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
		query := `SELECT id, routine_name, started_at, completed_at, duration_ms, trigger_source, exit_code, status, summary, error_message,
			items_processed, input_tokens, cache_read_tokens, output_tokens, estimated_cost_usd, model, session_id
			FROM routine_runs
			WHERE routine_name = ?
			ORDER BY started_at DESC
			LIMIT ?;`
		rows, err = s.db.QueryContext(ctx, query, routineName, limit)
	} else {
		query := `SELECT id, routine_name, started_at, completed_at, duration_ms, trigger_source, exit_code, status, summary, error_message,
			items_processed, input_tokens, cache_read_tokens, output_tokens, estimated_cost_usd, model, session_id
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
		var summary, errorMsg, model, sessionID sql.NullString
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
			&r.ItemsProcessed,
			&r.InputTokens,
			&r.CacheReadTokens,
			&r.OutputTokens,
			&r.EstimatedCostUSD,
			&model,
			&sessionID,
		); err != nil {
			return nil, fmt.Errorf("failed to scan routine run: %w", err)
		}
		if summary.Valid {
			r.Summary = summary.String
		}
		if errorMsg.Valid {
			r.ErrorMessage = errorMsg.String
		}
		if model.Valid {
			r.Model = model.String
		}
		if sessionID.Valid {
			r.SessionID = sessionID.String
		}
		list = append(list, r)
	}

	return list, rows.Err()
}

// GetLatestRun returns the most recent execution record for a given routine.
func (s *SQLiteStore) GetLatestRun(ctx context.Context, routineName string) (*RoutineRun, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `SELECT id, routine_name, started_at, completed_at, duration_ms, trigger_source, exit_code, status, summary, error_message,
		items_processed, input_tokens, cache_read_tokens, output_tokens, estimated_cost_usd, model, session_id
		FROM routine_runs
		WHERE routine_name = ?
		ORDER BY started_at DESC
		LIMIT 1;`

	row := s.db.QueryRowContext(ctx, query, strings.TrimSpace(routineName))
	var r RoutineRun
	var summary, errorMsg, model, sessionID sql.NullString
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
		&r.ItemsProcessed,
		&r.InputTokens,
		&r.CacheReadTokens,
		&r.OutputTokens,
		&r.EstimatedCostUSD,
		&model,
		&sessionID,
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
	if model.Valid {
		r.Model = model.String
	}
	if sessionID.Valid {
		r.SessionID = sessionID.String
	}
	return &r, nil
}

// RecordCommandEvent records a CLI or MCP command execution event and retains the latest 5000 events.
func (s *SQLiteStore) RecordCommandEvent(ctx context.Context, ev CommandTelemetryEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if ev.StartedAt == 0 {
		ev.StartedAt = time.Now().Unix()
	}

	isErr := 0
	if ev.IsError {
		isErr = 1
	}

	query := `INSERT INTO command_telemetry (surface, command, scope, started_at, duration_ms, is_error)
		VALUES (?, ?, ?, ?, ?, ?);`
	_, err := s.db.ExecContext(ctx, query, ev.Surface, ev.Command, ev.Scope, ev.StartedAt, ev.DurationMs, isErr)
	if err != nil {
		return fmt.Errorf("failed to record command telemetry event: %w", err)
	}

	limit := s.cmdRetentionLimit
	if limit <= 0 {
		limit = DefaultMaxCmdEventsRetention
	}
	pruneQuery := `DELETE FROM command_telemetry WHERE id NOT IN (
		SELECT id FROM command_telemetry ORDER BY started_at DESC, id DESC LIMIT ?
	);`
	_, _ = s.db.ExecContext(ctx, pruneQuery, limit)

	return nil
}

// GetCommandAggregates returns aggregated telemetry for command execution.
func (s *SQLiteStore) GetCommandAggregates(ctx context.Context, sinceUnix int64, limit int) ([]CommandTelemetryAgg, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}

	var query string
	var args []any
	if sinceUnix > 0 {
		query = `SELECT surface, command, COUNT(*) as total_calls,
			SUM(CASE WHEN is_error != 0 THEN 1 ELSE 0 END) as error_calls,
			COALESCE(AVG(duration_ms), 0.0) as avg_duration_ms,
			COALESCE(MAX(duration_ms), 0) as max_duration_ms,
			COALESCE(MAX(started_at), 0) as last_called_at
			FROM command_telemetry
			WHERE started_at >= ?
			GROUP BY surface, command
			ORDER BY total_calls DESC, last_called_at DESC
			LIMIT ?;`
		args = []any{sinceUnix, limit}
	} else {
		query = `SELECT surface, command, COUNT(*) as total_calls,
			SUM(CASE WHEN is_error != 0 THEN 1 ELSE 0 END) as error_calls,
			COALESCE(AVG(duration_ms), 0.0) as avg_duration_ms,
			COALESCE(MAX(duration_ms), 0) as max_duration_ms,
			COALESCE(MAX(started_at), 0) as last_called_at
			FROM command_telemetry
			GROUP BY surface, command
			ORDER BY total_calls DESC, last_called_at DESC
			LIMIT ?;`
		args = []any{limit}
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query command aggregates: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var results []CommandTelemetryAgg
	for rows.Next() {
		var a CommandTelemetryAgg
		if err := rows.Scan(
			&a.Surface,
			&a.Command,
			&a.TotalCalls,
			&a.ErrorCalls,
			&a.AvgDurationMs,
			&a.MaxDurationMs,
			&a.LastCalledAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan command aggregate: %w", err)
		}
		results = append(results, a)
	}
	return results, rows.Err()
}

// UpsertSessionTelemetry inserts or updates a session telemetry record.
func (s *SQLiteStore) UpsertSessionTelemetry(ctx context.Context, rec SessionTelemetryRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().Unix()
	if rec.StartedAt == 0 {
		rec.StartedAt = now
	}
	if rec.UpdatedAt == 0 {
		rec.UpdatedAt = now
	}

	isRoutine := 0
	if rec.IsRoutine {
		isRoutine = 1
	}

	query := `INSERT INTO session_telemetry (
		session_id, parent_session_id, agent, model, role, started_at, updated_at,
		turns_count, subagents_count, input_tokens, cache_read_tokens, output_tokens,
		estimated_cost_usd, skills_used, is_routine, routine_name
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(session_id) DO UPDATE SET
		parent_session_id = excluded.parent_session_id,
		agent = excluded.agent,
		model = excluded.model,
		role = excluded.role,
		started_at = excluded.started_at,
		updated_at = excluded.updated_at,
		turns_count = excluded.turns_count,
		subagents_count = excluded.subagents_count,
		input_tokens = excluded.input_tokens,
		cache_read_tokens = excluded.cache_read_tokens,
		output_tokens = excluded.output_tokens,
		estimated_cost_usd = excluded.estimated_cost_usd,
		skills_used = excluded.skills_used,
		is_routine = excluded.is_routine,
		routine_name = excluded.routine_name;`

	_, err := s.db.ExecContext(ctx, query,
		rec.SessionID,
		rec.ParentSessionID,
		rec.Agent,
		rec.Model,
		rec.Role,
		rec.StartedAt,
		rec.UpdatedAt,
		rec.TurnsCount,
		rec.SubagentsCount,
		rec.InputTokens,
		rec.CacheReadTokens,
		rec.OutputTokens,
		rec.EstimatedCostUSD,
		rec.SkillsUsed,
		isRoutine,
		rec.RoutineName,
	)
	if err != nil {
		return fmt.Errorf("failed to upsert session telemetry: %w", err)
	}
	return nil
}

// GetRecentSessionsTelemetry retrieves sessions ordered by started_at DESC.
func (s *SQLiteStore) GetRecentSessionsTelemetry(ctx context.Context, sinceUnix int64, limit int) ([]SessionTelemetryRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}

	var query string
	var args []any
	if sinceUnix > 0 {
		query = `SELECT session_id, parent_session_id, agent, model, role, started_at, updated_at,
			turns_count, subagents_count, input_tokens, cache_read_tokens, output_tokens,
			estimated_cost_usd, skills_used, is_routine, routine_name
			FROM session_telemetry
			WHERE started_at >= ?
			ORDER BY started_at DESC
			LIMIT ?;`
		args = []any{sinceUnix, limit}
	} else {
		query = `SELECT session_id, parent_session_id, agent, model, role, started_at, updated_at,
			turns_count, subagents_count, input_tokens, cache_read_tokens, output_tokens,
			estimated_cost_usd, skills_used, is_routine, routine_name
			FROM session_telemetry
			ORDER BY started_at DESC
			LIMIT ?;`
		args = []any{limit}
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query session telemetry: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var results []SessionTelemetryRecord
	for rows.Next() {
		var rec SessionTelemetryRecord
		var isRoutine int
		if err := rows.Scan(
			&rec.SessionID,
			&rec.ParentSessionID,
			&rec.Agent,
			&rec.Model,
			&rec.Role,
			&rec.StartedAt,
			&rec.UpdatedAt,
			&rec.TurnsCount,
			&rec.SubagentsCount,
			&rec.InputTokens,
			&rec.CacheReadTokens,
			&rec.OutputTokens,
			&rec.EstimatedCostUSD,
			&rec.SkillsUsed,
			&isRoutine,
			&rec.RoutineName,
		); err != nil {
			return nil, fmt.Errorf("failed to scan session telemetry: %w", err)
		}
		rec.IsRoutine = (isRoutine != 0)
		results = append(results, rec)
	}
	return results, rows.Err()
}

// GetRoutineWeightAggregates aggregates resource usage metrics grouped by routine name.
func (s *SQLiteStore) GetRoutineWeightAggregates(ctx context.Context, sinceUnix int64) ([]RoutineWeightAgg, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var query string
	var args []any
	if sinceUnix > 0 {
		query = `SELECT routine_name,
			COUNT(*) as total_runs,
			SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END) as success_runs,
			SUM(CASE WHEN status != 'success' THEN 1 ELSE 0 END) as failed_runs,
			COALESCE(AVG(duration_ms), 0.0) as avg_duration_ms,
			COALESCE(MAX(duration_ms), 0) as max_duration_ms,
			COALESCE(SUM(items_processed), 0) as total_items,
			COALESCE(SUM(input_tokens), 0) as total_input_tokens,
			COALESCE(SUM(cache_read_tokens), 0) as total_cache_tokens,
			COALESCE(SUM(output_tokens), 0) as total_output_tokens,
			COALESCE(SUM(estimated_cost_usd), 0.0) as total_cost_usd,
			COALESCE(MAX(started_at), 0) as last_run_at
			FROM routine_runs
			WHERE started_at >= ?
			GROUP BY routine_name
			ORDER BY total_runs DESC, last_run_at DESC;`
		args = []any{sinceUnix}
	} else {
		query = `SELECT routine_name,
			COUNT(*) as total_runs,
			SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END) as success_runs,
			SUM(CASE WHEN status != 'success' THEN 1 ELSE 0 END) as failed_runs,
			COALESCE(AVG(duration_ms), 0.0) as avg_duration_ms,
			COALESCE(MAX(duration_ms), 0) as max_duration_ms,
			COALESCE(SUM(items_processed), 0) as total_items,
			COALESCE(SUM(input_tokens), 0) as total_input_tokens,
			COALESCE(SUM(cache_read_tokens), 0) as total_cache_tokens,
			COALESCE(SUM(output_tokens), 0) as total_output_tokens,
			COALESCE(SUM(estimated_cost_usd), 0.0) as total_cost_usd,
			COALESCE(MAX(started_at), 0) as last_run_at
			FROM routine_runs
			GROUP BY routine_name
			ORDER BY total_runs DESC, last_run_at DESC;`
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query routine weight aggregates: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var results []RoutineWeightAgg
	for rows.Next() {
		var agg RoutineWeightAgg
		if err := rows.Scan(
			&agg.RoutineName,
			&agg.TotalRuns,
			&agg.SuccessRuns,
			&agg.FailedRuns,
			&agg.AvgDurationMs,
			&agg.MaxDurationMs,
			&agg.TotalItems,
			&agg.TotalInputTokens,
			&agg.TotalCacheTokens,
			&agg.TotalOutputTokens,
			&agg.TotalCostUSD,
			&agg.LastRunAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan routine weight aggregate: %w", err)
		}
		results = append(results, agg)
	}
	return results, rows.Err()
}

// GetModelUsageAggregates aggregates model and agent telemetry across sessions.
func (s *SQLiteStore) GetModelUsageAggregates(ctx context.Context, sinceUnix int64) ([]ModelUsageAgg, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var query string
	var args []any
	if sinceUnix > 0 {
		query = `SELECT model, agent,
			COUNT(*) as sessions_count,
			COALESCE(SUM(subagents_count), 0) as subagents_count,
			COALESCE(SUM(input_tokens), 0) as input_tokens,
			COALESCE(SUM(cache_read_tokens), 0) as cache_read_tokens,
			COALESCE(SUM(output_tokens), 0) as output_tokens,
			COALESCE(SUM(estimated_cost_usd), 0.0) as estimated_cost_usd
			FROM session_telemetry
			WHERE started_at >= ?
			GROUP BY model, agent
			ORDER BY estimated_cost_usd DESC, sessions_count DESC;`
		args = []any{sinceUnix}
	} else {
		query = `SELECT model, agent,
			COUNT(*) as sessions_count,
			COALESCE(SUM(subagents_count), 0) as subagents_count,
			COALESCE(SUM(input_tokens), 0) as input_tokens,
			COALESCE(SUM(cache_read_tokens), 0) as cache_read_tokens,
			COALESCE(SUM(output_tokens), 0) as output_tokens,
			COALESCE(SUM(estimated_cost_usd), 0.0) as estimated_cost_usd
			FROM session_telemetry
			GROUP BY model, agent
			ORDER BY estimated_cost_usd DESC, sessions_count DESC;`
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query model usage aggregates: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var results []ModelUsageAgg
	for rows.Next() {
		var agg ModelUsageAgg
		if err := rows.Scan(
			&agg.Model,
			&agg.Agent,
			&agg.SessionsCount,
			&agg.SubagentsCount,
			&agg.InputTokens,
			&agg.CacheReadTokens,
			&agg.OutputTokens,
			&agg.EstimatedCostUSD,
		); err != nil {
			return nil, fmt.Errorf("failed to scan model usage aggregate: %w", err)
		}
		results = append(results, agg)
	}
	return results, rows.Err()
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
