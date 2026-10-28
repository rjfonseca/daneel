package routine

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestStore_RecordAndGetHistory(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "routine_runs.db")

	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	now := time.Now().Unix()

	run1 := RoutineRun{
		ID:            "run-1",
		RoutineName:   "tidy",
		StartedAt:     now - 100,
		CompletedAt:   now - 98,
		DurationMs:    2000,
		TriggerSource: "systemd",
		ExitCode:      0,
		Status:        "success",
		Summary:       "Tidied 42 notes",
	}
	run2 := RoutineRun{
		ID:            "run-2",
		RoutineName:   "workspace",
		StartedAt:     now - 50,
		CompletedAt:   now - 45,
		DurationMs:    5000,
		TriggerSource: "cron",
		ExitCode:      0,
		Status:        "success",
		Summary:       "Synced 12 repos",
	}
	run3 := RoutineRun{
		ID:            "run-3",
		RoutineName:   "tidy",
		StartedAt:     now - 10,
		CompletedAt:   now - 8,
		DurationMs:    2100,
		TriggerSource: "web_cockpit",
		ExitCode:      1,
		Status:        "failed",
		Summary:       "Failed to parse note",
		ErrorMessage:  "syntax error in note.md",
	}

	for _, r := range []RoutineRun{run1, run2, run3} {
		if err := store.RecordRun(ctx, r); err != nil {
			t.Fatalf("failed to record run %s: %v", r.ID, err)
		}
	}

	// 1. Get all history
	history, err := store.GetHistory(ctx, "", 10)
	if err != nil {
		t.Fatalf("failed to get history: %v", err)
	}
	if len(history) != 3 {
		t.Fatalf("expected 3 runs, got %d", len(history))
	}
	// Verify descending order by StartedAt: run3 (now-10), run2 (now-50), run1 (now-100)
	if history[0].ID != "run-3" || history[1].ID != "run-2" || history[2].ID != "run-1" {
		t.Errorf("history not in descending order: %+v", history)
	}

	// 2. Filter by routine name
	tidyHistory, err := store.GetHistory(ctx, "tidy", 10)
	if err != nil {
		t.Fatalf("failed to get tidy history: %v", err)
	}
	if len(tidyHistory) != 2 {
		t.Fatalf("expected 2 tidy runs, got %d", len(tidyHistory))
	}
	if tidyHistory[0].ID != "run-3" || tidyHistory[1].ID != "run-1" {
		t.Errorf("unexpected tidy history: %+v", tidyHistory)
	}
	if tidyHistory[0].TriggerSource != "web_cockpit" || tidyHistory[0].Status != "failed" || tidyHistory[0].ErrorMessage != "syntax error in note.md" {
		t.Errorf("unexpected field values for run-3: %+v", tidyHistory[0])
	}

	// 3. Limit testing
	limited, err := store.GetHistory(ctx, "", 1)
	if err != nil {
		t.Fatalf("failed to get limited history: %v", err)
	}
	if len(limited) != 1 || limited[0].ID != "run-3" {
		t.Errorf("expected only run-3 in limited history, got %+v", limited)
	}
}

func TestStore_GetLatestRun(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "routine_runs.db")

	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	now := time.Now().Unix()

	// Non-existent routine
	latest, err := store.GetLatestRun(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("unexpected error for non-existent routine: %v", err)
	}
	if latest != nil {
		t.Errorf("expected nil for non-existent routine, got %+v", latest)
	}

	// Record older and newer runs
	_ = store.RecordRun(ctx, RoutineRun{
		ID:          "run-old",
		RoutineName: "dream",
		StartedAt:   now - 100,
		ExitCode:    0,
		Status:      "success",
	})
	_ = store.RecordRun(ctx, RoutineRun{
		ID:          "run-new",
		RoutineName: "dream",
		StartedAt:   now - 10,
		ExitCode:    0,
		Status:      "success",
		Summary:     "REM consolidation complete",
	})

	latest, err = store.GetLatestRun(ctx, "dream")
	if err != nil {
		t.Fatalf("failed to get latest run: %v", err)
	}
	if latest == nil || latest.ID != "run-new" || latest.Summary != "REM consolidation complete" {
		t.Errorf("expected run-new as latest, got %+v", latest)
	}
}

func TestStore_RetentionPrune(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "routine_runs.db")

	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	baseTime := time.Now().Unix()

	// Set small retention for test
	store.SetRetentionLimit(5)

	for i := 0; i < 10; i++ {
		err := store.RecordRun(ctx, RoutineRun{
			ID:          string(rune('a' + i)),
			RoutineName: "tidy",
			StartedAt:   baseTime + int64(i),
			ExitCode:    0,
			Status:      "success",
		})
		if err != nil {
			t.Fatalf("failed to insert run %d: %v", i, err)
		}
	}

	history, err := store.GetHistory(ctx, "", 50)
	if err != nil {
		t.Fatalf("failed to get history: %v", err)
	}
	if len(history) != 5 {
		t.Fatalf("expected exactly 5 runs retained, got %d", len(history))
	}
	// The oldest runs ('a', 'b', 'c', 'd', 'e') should have been pruned; 'j' (i=9) should be newest
	if history[0].ID != "j" || history[4].ID != "f" {
		t.Errorf("unexpected retained items: %+v", history)
	}
}

func TestStore_ExtendedRoutineRunFields(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "routine_runs.db")

	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	now := time.Now().Unix()

	run := RoutineRun{
		ID:               "run-ext-1",
		RoutineName:      "dream",
		StartedAt:        now - 20,
		CompletedAt:      now - 10,
		DurationMs:       10000,
		TriggerSource:    "cron",
		ExitCode:         0,
		Status:           "success",
		Summary:          "Consolidated 42 notes",
		ErrorMessage:     "",
		ItemsProcessed:   42,
		InputTokens:      12000,
		CacheReadTokens:  4000,
		OutputTokens:     1500,
		EstimatedCostUSD: 0.045,
		Model:            "gemini-2.5-flash",
		SessionID:        "session-test-456",
	}

	if err := store.RecordRun(ctx, run); err != nil {
		t.Fatalf("failed to record extended run: %v", err)
	}

	history, err := store.GetHistory(ctx, "dream", 10)
	if err != nil {
		t.Fatalf("failed to get history: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("expected 1 run, got %d", len(history))
	}
	got := history[0]
	if got.ItemsProcessed != 42 {
		t.Errorf("expected ItemsProcessed 42, got %d", got.ItemsProcessed)
	}
	if got.InputTokens != 12000 {
		t.Errorf("expected InputTokens 12000, got %d", got.InputTokens)
	}
	if got.CacheReadTokens != 4000 {
		t.Errorf("expected CacheReadTokens 4000, got %d", got.CacheReadTokens)
	}
	if got.OutputTokens != 1500 {
		t.Errorf("expected OutputTokens 1500, got %d", got.OutputTokens)
	}
	if got.EstimatedCostUSD != 0.045 {
		t.Errorf("expected EstimatedCostUSD 0.045, got %f", got.EstimatedCostUSD)
	}
	if got.Model != "gemini-2.5-flash" {
		t.Errorf("expected Model gemini-2.5-flash, got %s", got.Model)
	}
	if got.SessionID != "session-test-456" {
		t.Errorf("expected SessionID session-test-456, got %s", got.SessionID)
	}

	latest, err := store.GetLatestRun(ctx, "dream")
	if err != nil {
		t.Fatalf("failed to get latest run: %v", err)
	}
	if latest == nil {
		t.Fatal("expected latest run, got nil")
	}
	if latest.ItemsProcessed != 42 || latest.Model != "gemini-2.5-flash" || latest.SessionID != "session-test-456" {
		t.Errorf("latest run does not match: %+v", latest)
	}
}

func TestStore_Migration_ExistingDB(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "legacy_routine_runs.db")

	// Create legacy database with old schema (without the 7 new columns)
	rawDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open raw sqlite: %v", err)
	}
	legacySchema := `CREATE TABLE routine_runs (
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
	);`
	if _, err := rawDB.Exec(legacySchema); err != nil {
		t.Fatalf("failed to create legacy schema: %v", err)
	}
	// Insert legacy row
	_, err = rawDB.Exec(`INSERT INTO routine_runs (
		id, routine_name, started_at, completed_at, duration_ms, trigger_source, exit_code, status, summary, error_message
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`,
		"run-legacy-1", "tidy", 1700000000, 1700000005, 5000, "cli", 0, "success", "Legacy run", "",
	)
	if err != nil {
		t.Fatalf("failed to insert legacy row: %v", err)
	}
	_ = rawDB.Close()

	// Open with OpenStore which executes initSchema and migrations
	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("OpenStore failed on legacy DB: %v", err)
	}
	defer func() { _ = store.Close() }()

	ctx := context.Background()

	// Legacy row should be readable with zero values for new fields
	legacyRun, err := store.GetLatestRun(ctx, "tidy")
	if err != nil {
		t.Fatalf("failed to get legacy run: %v", err)
	}
	if legacyRun == nil || legacyRun.ID != "run-legacy-1" {
		t.Fatalf("expected legacy run, got %+v", legacyRun)
	}
	if legacyRun.ItemsProcessed != 0 || legacyRun.InputTokens != 0 || legacyRun.Model != "" {
		t.Errorf("expected zero defaults for legacy row, got %+v", legacyRun)
	}

	// Insert new row with new fields
	newRun := RoutineRun{
		ID:               "run-migrated-2",
		RoutineName:      "tidy",
		StartedAt:        1700000010,
		CompletedAt:      1700000015,
		DurationMs:       5000,
		TriggerSource:    "systemd",
		ExitCode:         0,
		Status:           "success",
		Summary:          "Migrated run",
		ItemsProcessed:   10,
		InputTokens:      500,
		CacheReadTokens:  200,
		OutputTokens:     100,
		EstimatedCostUSD: 0.002,
		Model:            "gemini-2.5-flash",
		SessionID:        "session-migrated",
	}
	if err := store.RecordRun(ctx, newRun); err != nil {
		t.Fatalf("failed to record run after migration: %v", err)
	}

	history, err := store.GetHistory(ctx, "tidy", 10)
	if err != nil {
		t.Fatalf("failed to get history: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("expected 2 runs in history, got %d", len(history))
	}
	if history[0].ID != "run-migrated-2" || history[0].ItemsProcessed != 10 {
		t.Errorf("new run not parsed properly: %+v", history[0])
	}
	if history[1].ID != "run-legacy-1" || history[1].ItemsProcessed != 0 {
		t.Errorf("legacy run not parsed properly: %+v", history[1])
	}
}

func TestStore_CommandTelemetry(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "routine_runs.db")

	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	now := time.Now().Unix()

	events := []CommandTelemetryEvent{
		{Surface: "cli", Command: "search", Scope: "tech", StartedAt: now - 30, DurationMs: 120, IsError: false},
		{Surface: "cli", Command: "search", Scope: "all", StartedAt: now - 20, DurationMs: 80, IsError: false},
		{Surface: "cli", Command: "search", Scope: "meta", StartedAt: now - 10, DurationMs: 200, IsError: true},
		{Surface: "mcp", Command: "resolve", Scope: "note", StartedAt: now - 25, DurationMs: 5, IsError: false},
		{Surface: "mcp", Command: "resolve", Scope: "person", StartedAt: now - 15, DurationMs: 15, IsError: false},
		{Surface: "cli", Command: "dream", Scope: "", StartedAt: now - 5, DurationMs: 3000, IsError: false},
	}

	for _, ev := range events {
		if err := store.RecordCommandEvent(ctx, ev); err != nil {
			t.Fatalf("failed to record command event: %v", err)
		}
	}

	aggs, err := store.GetCommandAggregates(ctx, now-100, 10)
	if err != nil {
		t.Fatalf("failed to get command aggregates: %v", err)
	}

	if len(aggs) != 3 {
		t.Fatalf("expected 3 command aggregate groups, got %d: %+v", len(aggs), aggs)
	}

	// Verify cli search aggregate
	var cliSearch *CommandTelemetryAgg
	for i := range aggs {
		if aggs[i].Surface == "cli" && aggs[i].Command == "search" {
			cliSearch = &aggs[i]
			break
		}
	}
	if cliSearch == nil {
		t.Fatal("expected cli:search aggregate group")
	}
	if cliSearch.TotalCalls != 3 {
		t.Errorf("expected 3 total calls for cli:search, got %d", cliSearch.TotalCalls)
	}
	if cliSearch.ErrorCalls != 1 {
		t.Errorf("expected 1 error call for cli:search, got %d", cliSearch.ErrorCalls)
	}
	expectedAvg := (120.0 + 80.0 + 200.0) / 3.0
	if cliSearch.AvgDurationMs < expectedAvg-0.1 || cliSearch.AvgDurationMs > expectedAvg+0.1 {
		t.Errorf("expected avg duration ~%f, got %f", expectedAvg, cliSearch.AvgDurationMs)
	}
	if cliSearch.MaxDurationMs != 200 {
		t.Errorf("expected max duration 200, got %d", cliSearch.MaxDurationMs)
	}
	if cliSearch.LastCalledAt != now-10 {
		t.Errorf("expected last called at %d, got %d", now-10, cliSearch.LastCalledAt)
	}

	// Test command telemetry retention pruning
	store.SetCommandRetentionLimit(2)
	// Recording one more event should prune down to 2 events.
	if err := store.RecordCommandEvent(ctx, CommandTelemetryEvent{
		Surface: "cli", Command: "doctor", Scope: "", StartedAt: now, DurationMs: 50, IsError: false,
	}); err != nil {
		t.Fatalf("failed to record doctor command event: %v", err)
	}
	aggsAfterPrune, err := store.GetCommandAggregates(ctx, 0, 100)
	if err != nil {
		t.Fatalf("failed to get aggregates after prune: %v", err)
	}
	totalEvents := 0
	for _, a := range aggsAfterPrune {
		totalEvents += a.TotalCalls
	}
	if totalEvents != 2 {
		t.Errorf("expected 2 total events retained, got %d", totalEvents)
	}
}

func TestStore_SessionTelemetry(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "routine_runs.db")

	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	now := time.Now().Unix()

	rec1 := SessionTelemetryRecord{
		SessionID:        "sess-001",
		ParentSessionID:  "",
		Agent:            "antigravity",
		Model:            "gemini-2.5-pro",
		Role:             "architect",
		StartedAt:        now - 50,
		UpdatedAt:        now - 40,
		TurnsCount:       4,
		SubagentsCount:   1,
		InputTokens:      50000,
		CacheReadTokens:  20000,
		OutputTokens:     3000,
		EstimatedCostUSD: 0.12,
		SkillsUsed:       "second-brain,vault-assistant",
		IsRoutine:        false,
		RoutineName:      "",
	}

	if err := store.UpsertSessionTelemetry(ctx, rec1); err != nil {
		t.Fatalf("failed to upsert session telemetry: %v", err)
	}

	// Update the session record (upsert)
	rec1Updated := rec1
	rec1Updated.UpdatedAt = now - 30
	rec1Updated.TurnsCount = 8
	rec1Updated.OutputTokens = 6000
	rec1Updated.EstimatedCostUSD = 0.20
	if err := store.UpsertSessionTelemetry(ctx, rec1Updated); err != nil {
		t.Fatalf("failed to update session telemetry: %v", err)
	}

	// Insert another session
	rec2 := SessionTelemetryRecord{
		SessionID:        "sess-002",
		ParentSessionID:  "sess-001",
		Agent:            "claude-code",
		Model:            "claude-3-7-sonnet",
		Role:             "implementer",
		StartedAt:        now - 20,
		UpdatedAt:        now - 10,
		TurnsCount:       2,
		SubagentsCount:   0,
		InputTokens:      15000,
		CacheReadTokens:  5000,
		OutputTokens:     1000,
		EstimatedCostUSD: 0.05,
		SkillsUsed:       "test-driven-development",
		IsRoutine:        true,
		RoutineName:      "dream",
	}
	if err := store.UpsertSessionTelemetry(ctx, rec2); err != nil {
		t.Fatalf("failed to upsert second session: %v", err)
	}

	sessions, err := store.GetRecentSessionsTelemetry(ctx, now-100, 10)
	if err != nil {
		t.Fatalf("failed to get recent sessions: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}
	// Ordered by started_at DESC: sess-002 then sess-001
	if sessions[0].SessionID != "sess-002" || sessions[1].SessionID != "sess-001" {
		t.Errorf("sessions not ordered by started_at DESC: %+v", sessions)
	}
	if sessions[1].TurnsCount != 8 || sessions[1].OutputTokens != 6000 || sessions[1].EstimatedCostUSD != 0.20 {
		t.Errorf("upsert update not reflected: %+v", sessions[1])
	}
	if sessions[0].IsRoutine != true || sessions[0].RoutineName != "dream" {
		t.Errorf("routine fields not preserved: %+v", sessions[0])
	}
}

func TestStore_RoutineWeightAggregates(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "routine_runs.db")

	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	now := time.Now().Unix()

	runs := []RoutineRun{
		{
			ID: "r1", RoutineName: "tidy", StartedAt: now - 30, CompletedAt: now - 28, DurationMs: 2000,
			Status: "success", ItemsProcessed: 10, InputTokens: 1000, CacheReadTokens: 500, OutputTokens: 100, EstimatedCostUSD: 0.005,
		},
		{
			ID: "r2", RoutineName: "tidy", StartedAt: now - 20, CompletedAt: now - 17, DurationMs: 3000,
			Status: "success", ItemsProcessed: 15, InputTokens: 2000, CacheReadTokens: 1000, OutputTokens: 200, EstimatedCostUSD: 0.010,
		},
		{
			ID: "r3", RoutineName: "doctor", StartedAt: now - 15, CompletedAt: now - 11, DurationMs: 4000,
			Status: "failed", ItemsProcessed: 5, InputTokens: 3000, CacheReadTokens: 1000, OutputTokens: 300, EstimatedCostUSD: 0.015,
		},
	}

	for _, r := range runs {
		if err := store.RecordRun(ctx, r); err != nil {
			t.Fatalf("failed to record run: %v", err)
		}
	}

	aggs, err := store.GetRoutineWeightAggregates(ctx, now-100)
	if err != nil {
		t.Fatalf("failed to get routine weight aggregates: %v", err)
	}
	if len(aggs) != 2 {
		t.Fatalf("expected 2 routine weight agg groups, got %d", len(aggs))
	}

	var tidyAgg, doctorAgg *RoutineWeightAgg
	for i := range aggs {
		if aggs[i].RoutineName == "tidy" {
			tidyAgg = &aggs[i]
		}
		if aggs[i].RoutineName == "doctor" {
			doctorAgg = &aggs[i]
		}
	}

	if tidyAgg == nil || doctorAgg == nil {
		t.Fatalf("expected both tidy and doctor aggregates: %+v", aggs)
	}

	if tidyAgg.TotalRuns != 2 || tidyAgg.SuccessRuns != 2 || tidyAgg.FailedRuns != 0 {
		t.Errorf("tidy runs counts incorrect: %+v", tidyAgg)
	}
	if tidyAgg.TotalItems != 25 || tidyAgg.TotalInputTokens != 3000 || tidyAgg.TotalCacheTokens != 1500 || tidyAgg.TotalOutputTokens != 300 {
		t.Errorf("tidy token totals incorrect: %+v", tidyAgg)
	}
	if tidyAgg.AvgDurationMs != 2500 || tidyAgg.MaxDurationMs != 3000 {
		t.Errorf("tidy duration metrics incorrect: %+v", tidyAgg)
	}
	if tidyAgg.TotalCostUSD < 0.014 || tidyAgg.TotalCostUSD > 0.016 {
		t.Errorf("tidy total cost incorrect: %f", tidyAgg.TotalCostUSD)
	}

	if doctorAgg.TotalRuns != 1 || doctorAgg.SuccessRuns != 0 || doctorAgg.FailedRuns != 1 {
		t.Errorf("doctor runs counts incorrect: %+v", doctorAgg)
	}
}

func TestStore_ModelUsageAggregates(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "routine_runs.db")

	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	now := time.Now().Unix()

	sessions := []SessionTelemetryRecord{
		{
			SessionID: "s1", Agent: "antigravity", Model: "gemini-2.5-pro",
			StartedAt: now - 30, SubagentsCount: 2, InputTokens: 10000, CacheReadTokens: 5000, OutputTokens: 1000, EstimatedCostUSD: 0.05,
		},
		{
			SessionID: "s2", Agent: "antigravity", Model: "gemini-2.5-pro",
			StartedAt: now - 20, SubagentsCount: 1, InputTokens: 20000, CacheReadTokens: 10000, OutputTokens: 2000, EstimatedCostUSD: 0.10,
		},
		{
			SessionID: "s3", Agent: "cursor", Model: "claude-3-7-sonnet",
			StartedAt: now - 10, SubagentsCount: 0, InputTokens: 5000, CacheReadTokens: 2000, OutputTokens: 500, EstimatedCostUSD: 0.02,
		},
	}

	for _, s := range sessions {
		if err := store.UpsertSessionTelemetry(ctx, s); err != nil {
			t.Fatalf("failed to upsert session: %v", err)
		}
	}

	aggs, err := store.GetModelUsageAggregates(ctx, now-100)
	if err != nil {
		t.Fatalf("failed to get model usage aggregates: %v", err)
	}

	if len(aggs) != 2 {
		t.Fatalf("expected 2 model usage aggregates, got %d", len(aggs))
	}

	if aggs[0].Model != "gemini-2.5-pro" || aggs[0].Agent != "antigravity" {
		t.Errorf("first model agg should be gemini-2.5-pro/antigravity: %+v", aggs[0])
	}
	if aggs[0].SessionsCount != 2 || aggs[0].SubagentsCount != 3 {
		t.Errorf("session/subagent counts mismatch: %+v", aggs[0])
	}
	if aggs[0].InputTokens != 30000 || aggs[0].CacheReadTokens != 15000 || aggs[0].OutputTokens != 3000 {
		t.Errorf("token sums mismatch: %+v", aggs[0])
	}
	if aggs[0].EstimatedCostUSD < 0.14 || aggs[0].EstimatedCostUSD > 0.16 {
		t.Errorf("cost sum mismatch: %f", aggs[0].EstimatedCostUSD)
	}
}
