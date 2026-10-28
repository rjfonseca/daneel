package tui

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/rjfonseca/daneel/internal/routine"
)

func TestTelemetryView_EmptyState(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "routine_runs.db")
	store, err := routine.OpenStore(dbPath)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	styles := NewStyles()
	tv := NewTelemetryView(tempDir, styles)
	tv.SetStore(store)
	tv.SetSize(120, 40)
	tv.Refresh()

	if !tv.loaded {
		t.Errorf("expected view to be marked loaded after Refresh")
	}

	rendered := tv.View()
	if !strings.Contains(rendered, "AI FinOps") {
		t.Errorf("expected rendered view to contain 'AI FinOps', got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "Routine Weights") {
		t.Errorf("expected rendered view to contain 'Routine Weights', got:\n%s", rendered)
	}
}

func TestTelemetryView_RenderWithData(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "routine_runs.db")
	store, err := routine.OpenStore(dbPath)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	now := time.Now().Unix()

	// 1. Seed RoutineRun
	err = store.RecordRun(ctx, routine.RoutineRun{
		ID:               "run-dream-1",
		RoutineName:      "dream",
		StartedAt:        now - 3600,
		CompletedAt:      now - 3300,
		DurationMs:       300000,
		TriggerSource:    "cli",
		Status:           "success",
		Summary:          "Consolidated 12 notes",
		ItemsProcessed:   12,
		InputTokens:      150000,
		CacheReadTokens:  50000,
		OutputTokens:     12000,
		EstimatedCostUSD: 0.245,
		Model:            "gemini-pro",
	})
	if err != nil {
		t.Fatalf("failed to record routine run: %v", err)
	}

	// 2. Seed SessionTelemetryRecord (parent)
	err = store.UpsertSessionTelemetry(ctx, routine.SessionTelemetryRecord{
		SessionID:        "session-abc-123",
		Agent:            "antigravity",
		Model:            "gemini-pro",
		StartedAt:        now - 7200,
		UpdatedAt:        now - 3600,
		TurnsCount:       15,
		SubagentsCount:   1,
		InputTokens:      200000,
		CacheReadTokens:  100000,
		OutputTokens:     25000,
		EstimatedCostUSD: 0.375,
		SkillsUsed:       "vault-assistant,codebase-design",
	})
	if err != nil {
		t.Fatalf("failed to record session: %v", err)
	}

	// 3. Seed SessionTelemetryRecord (subagent)
	err = store.UpsertSessionTelemetry(ctx, routine.SessionTelemetryRecord{
		SessionID:        "subagent-child-1",
		ParentSessionID:  "session-abc-123",
		Agent:            "antigravity",
		Model:            "gemini-flash",
		StartedAt:        now - 5400,
		UpdatedAt:        now - 5100,
		TurnsCount:       4,
		SubagentsCount:   0,
		InputTokens:      50000,
		CacheReadTokens:  25000,
		OutputTokens:     5000,
		EstimatedCostUSD: 0.015,
		SkillsUsed:       "codebase-design",
	})
	if err != nil {
		t.Fatalf("failed to record child session: %v", err)
	}

	// 4. Seed CommandTelemetryEvents
	err = store.RecordCommandEvent(ctx, routine.CommandTelemetryEvent{
		Surface:    "cli",
		Command:    "resolve",
		StartedAt:  now - 1800,
		DurationMs: 12,
		IsError:    false,
	})
	if err != nil {
		t.Fatalf("failed to record command event: %v", err)
	}
	err = store.RecordCommandEvent(ctx, routine.CommandTelemetryEvent{
		Surface:    "mcp",
		Command:    "search",
		StartedAt:  now - 1500,
		DurationMs: 45,
		IsError:    false,
	})
	if err != nil {
		t.Fatalf("failed to record command event: %v", err)
	}

	styles := NewStyles()
	tv := NewTelemetryView(tempDir, styles)
	tv.SetStore(store)
	tv.SetSize(140, 45)
	tv.Refresh()

	if !tv.loaded {
		t.Fatal("expected view to be loaded")
	}

	// Verify metrics computation
	if tv.totalCostUSD <= 0 {
		t.Errorf("expected totalCostUSD > 0, got %f", tv.totalCostUSD)
	}
	if tv.totalInputTokens <= 0 {
		t.Errorf("expected totalInputTokens > 0, got %d", tv.totalInputTokens)
	}
	if tv.totalCacheTokens <= 0 {
		t.Errorf("expected totalCacheTokens > 0, got %d", tv.totalCacheTokens)
	}
	if tv.totalOutputTokens <= 0 {
		t.Errorf("expected totalOutputTokens > 0, got %d", tv.totalOutputTokens)
	}
	if tv.cacheEfficiencyPct <= 0 {
		t.Errorf("expected cacheEfficiencyPct > 0, got %f", tv.cacheEfficiencyPct)
	}

	rendered := tv.View()

	// Verify expected text fragments
	expectedWords := []string{
		"AI FinOps",
		"gemini-pro",
		"antigravity",
		"dream",
		"resolve",
		"search",
		"Vault Primitives",
	}
	for _, word := range expectedWords {
		if !strings.Contains(rendered, word) {
			t.Errorf("expected rendered view to contain %q, but was missing", word)
		}
	}

	// Test Key interactions:
	// 1. 'w' cycles window: 7 -> 30 -> 1 -> 7
	if tv.windowDays != 7 {
		t.Errorf("expected initial windowDays=7, got %d", tv.windowDays)
	}
	tv, _ = tv.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("w")})
	if tv.windowDays != 30 {
		t.Errorf("expected windowDays=30 after pressing 'w', got %d", tv.windowDays)
	}
	tv, _ = tv.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("w")})
	if tv.windowDays != 1 {
		t.Errorf("expected windowDays=1 after pressing 'w' again, got %d", tv.windowDays)
	}
	tv, _ = tv.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("w")})
	if tv.windowDays != 7 {
		t.Errorf("expected windowDays=7 after cycling 'w', got %d", tv.windowDays)
	}

	// 2. 'l' and 'h' switch section
	if tv.selectedSection != 0 {
		t.Errorf("expected initial selectedSection=0, got %d", tv.selectedSection)
	}
	tv, _ = tv.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	if tv.selectedSection != 1 {
		t.Errorf("expected selectedSection=1 after 'l', got %d", tv.selectedSection)
	}
	tv, _ = tv.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	if tv.selectedSection != 2 {
		t.Errorf("expected selectedSection=2 after 'l', got %d", tv.selectedSection)
	}
	tv, _ = tv.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	if tv.selectedSection != 1 {
		t.Errorf("expected selectedSection=1 after 'h', got %d", tv.selectedSection)
	}

	// 3. 'j' and 'k' scroll
	tv, _ = tv.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	tv, _ = tv.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})

	// 4. 'r' refreshes
	tv, _ = tv.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	if !tv.loaded {
		t.Errorf("expected view to remain loaded after 'r'")
	}
}
