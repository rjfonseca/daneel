package routine

import (
	"context"
	"path/filepath"
	"testing"
	"time"
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
