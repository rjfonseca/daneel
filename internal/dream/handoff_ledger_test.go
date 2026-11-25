//nolint:revive // package comment not required
package dream

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAppendAndAggregateHandoffWindow(t *testing.T) {
	tempVault := t.TempDir()
	base := time.Date(2026, 11, 24, 9, 0, 0, 0, time.UTC)

	// Event before window (must be excluded)
	if err := AppendHandoffEvent(tempVault, HandoffEvent{
		Kind:         HandoffKindDreamCycle,
		Timestamp:    base.Add(-1 * time.Hour).Format(time.RFC3339),
		Date:         "2026-11-24",
		FilesScanned: 90,
		FixedLinks:   5,
		BrokenLinks:  3,
		Alerts:       []string{"Old alert from yesterday"},
	}); err != nil {
		t.Fatalf("AppendHandoffEvent pre-window failed: %v", err)
	}

	// Event 1 inside window: REM cycle with an alert that gets resolved later
	if err := AppendHandoffEvent(tempVault, HandoffEvent{
		Kind:             HandoffKindDreamCycle,
		Timestamp:        base.Add(2 * time.Hour).Format(time.RFC3339),
		Date:             "2026-11-24",
		FilesScanned:     100,
		FixedLinks:       2,
		BrokenLinks:      2,
		MissingMOCs:      []string{"notes/arch/moc-arch.md"},
		SyncedPeople:     []string{"Ada Lovelace"},
		IngestedMeetings: 1,
		Alerts:           []string{"2 broken links pending review", "Pending skill refactoring"},
		Insights:         []string{"Consolidate learnings"},
	}); err != nil {
		t.Fatalf("AppendHandoffEvent 1 failed: %v", err)
	}

	// Event 2 inside window: Evening closure
	if err := AppendHandoffEvent(tempVault, HandoffEvent{
		Kind:           HandoffKindEveningClosure,
		Timestamp:      base.Add(9 * time.Hour).Format(time.RFC3339),
		Date:           "2026-11-24",
		Summary:        "Closed daily log with 6.5h and promoted architecture note",
		Actions:        []string{"Created notes/arch/service-boundaries.md", "Updated telemetry"},
		ResolvedAlerts: []string{"Pending skill refactoring"},
	}); err != nil {
		t.Fatalf("AppendHandoffEvent 2 failed: %v", err)
	}

	// Event 3 inside window: Agent action
	if err := AppendHandoffEvent(tempVault, HandoffEvent{
		Kind:         HandoffKindAgentAction,
		Timestamp:    base.Add(10 * time.Hour).Format(time.RFC3339),
		Date:         "2026-11-24",
		Summary:      "Archived processed session transcripts",
		InboxPending: []string{"open-question.md"},
	}); err != nil {
		t.Fatalf("AppendHandoffEvent 3 failed: %v", err)
	}

	// Event 4 inside window: Morning REM cycle (later timestamp, 0 broken links, 0 alerts)
	if err := AppendHandoffEvent(tempVault, HandoffEvent{
		Kind:             HandoffKindDreamCycle,
		Timestamp:        base.Add(23 * time.Hour).Format(time.RFC3339),
		Date:             "2026-11-25",
		FilesScanned:     105,
		FixedLinks:       2,
		BrokenLinks:      0,
		MissingMOCs:      []string{"notes/arch/moc-arch.md", "notes/ia/moc-ia.md"},
		SyncedPeople:     []string{"Ada Lovelace", "Grace Hopper"},
		IngestedMeetings: 1,
		Syntheses:        []string{"notes/syntheses/event-driven.md"},
	}); err != nil {
		t.Fatalf("AppendHandoffEvent 4 failed: %v", err)
	}

	until := base.Add(24 * time.Hour)
	agg, err := AggregateHandoffWindow(tempVault, base, until)
	if err != nil {
		t.Fatalf("AggregateHandoffWindow failed: %v", err)
	}

	if agg.DreamCyclesCount != 2 {
		t.Errorf("expected DreamCyclesCount=2, got %d", agg.DreamCyclesCount)
	}
	if agg.FixedLinksTotal != 4 {
		t.Errorf("expected FixedLinksTotal=4, got %d", agg.FixedLinksTotal)
	}
	if agg.IngestedMeetingsTotal != 2 {
		t.Errorf("expected IngestedMeetingsTotal=2, got %d", agg.IngestedMeetingsTotal)
	}
	if agg.FilesScanned != 105 {
		t.Errorf("expected latest FilesScanned=105, got %d", agg.FilesScanned)
	}
	if agg.BrokenLinksLatest != 0 {
		t.Errorf("expected latest BrokenLinksLatest=0, got %d", agg.BrokenLinksLatest)
	}
	if len(agg.MissingMOCs) != 2 {
		t.Errorf("expected 2 deduplicated MissingMOCs, got %v", agg.MissingMOCs)
	}
	if len(agg.SyncedPeople) != 2 {
		t.Errorf("expected 2 deduplicated SyncedPeople, got %v", agg.SyncedPeople)
	}
	if len(agg.EveningClosures) != 1 {
		t.Errorf("expected 1 evening closure, got %d", len(agg.EveningClosures))
	}
	if len(agg.AgentActions) != 1 {
		t.Errorf("expected 1 agent action, got %d", len(agg.AgentActions))
	}
	// Latest dream cycle cleared alerts & insights, so stale alerts must NOT remain active
	if len(agg.Alerts) != 0 {
		t.Errorf("expected stale alerts to be cleared by latest cycle, got %v", agg.Alerts)
	}
	if len(agg.ResolvedAlerts) != 1 || agg.ResolvedAlerts[0] != "Pending skill refactoring" {
		t.Errorf("expected ResolvedAlerts=[Pending skill refactoring], got %v", agg.ResolvedAlerts)
	}
	if len(agg.InboxPending) != 1 || agg.InboxPending[0] != "open-question.md" {
		t.Errorf("expected InboxPending=[open-question.md], got %v", agg.InboxPending)
	}
}

func TestResolveLastMorningHandoff(t *testing.T) {
	tempVault := t.TempDir()
	now := time.Date(2026, 11, 25, 8, 30, 0, 0, time.UTC)

	// 1. Fallback when neither state nor daily logs exist -> now - 24h
	got := ResolveLastMorningHandoff(tempVault, "2026-11-25", now)
	wantFallback := now.Add(-24 * time.Hour)
	if !got.Equal(wantFallback) {
		t.Errorf("expected 24h fallback %v, got %v", wantFallback, got)
	}

	// 2. Daily file with Window marker from previous day
	dailyDir := filepath.Join(tempVault, "daily")
	if err := os.MkdirAll(dailyDir, 0750); err != nil {
		t.Fatalf("mkdir daily: %v", err)
	}
	prevDaily := filepath.Join(dailyDir, "2026-11-24.md")
	prevContent := `# Daily 2026-11-24

## 🤝 Handoff do Copiloto (Daneel ➔ Partner)

> **Window:** ` + "`2026-11-23T09:00:00Z`" + ` ➔ ` + "`2026-11-24T09:15:00Z`" + `
`
	if err := os.WriteFile(prevDaily, []byte(prevContent), 0600); err != nil {
		t.Fatalf("write prev daily: %v", err)
	}

	got = ResolveLastMorningHandoff(tempVault, "2026-11-25", now)
	wantFromDaily := time.Date(2026, 11, 24, 9, 15, 0, 0, time.UTC)
	if !got.Equal(wantFromDaily) {
		t.Errorf("expected timestamp from daily window %v, got %v", wantFromDaily, got)
	}

	// 3. State file newer than daily marker takes precedence
	stateTime := time.Date(2026, 11, 24, 10, 0, 0, 0, time.UTC)
	if err := SaveHandoffState(tempVault, stateTime, "daily/2026-11-24.md"); err != nil {
		t.Fatalf("SaveHandoffState failed: %v", err)
	}
	got = ResolveLastMorningHandoff(tempVault, "2026-11-25", now)
	if !got.Equal(stateTime) {
		t.Errorf("expected state timestamp %v, got %v", stateTime, got)
	}
}

func TestAggregateHandoffWindow_FallbackToDreamHandoffJSON(t *testing.T) {
	tempVault := t.TempDir()
	cacheDir := filepath.Join(tempVault, ".daneel", "cache")
	if err := os.MkdirAll(cacheDir, 0750); err != nil {
		t.Fatalf("mkdir cache: %v", err)
	}

	rep := FullDreamReport{
		Date:                  "2026-11-24",
		TotalFilesScanned:     50,
		FixedLinksCount:       3,
		BrokenLinksCount:      1,
		MissingMOCs:           []string{"notes/arch/service.md"},
		SyncedPeople:          []string{"Ada Lovelace"},
		IngestedMeetingsCount: 2,
		TemporalAnalysis: &TemporalWindowData{
			DriftAlerts: []string{"Strategic time below 30%"},
		},
	}
	b, _ := json.Marshal(rep)
	if err := os.WriteFile(filepath.Join(cacheDir, "dream_handoff.json"), b, 0600); err != nil {
		t.Fatalf("write dream_handoff.json: %v", err)
	}

	now := time.Date(2026, 11, 25, 8, 30, 0, 0, time.UTC)
	agg, err := AggregateHandoffWindow(tempVault, now.Add(-24*time.Hour), now)
	if err != nil {
		t.Fatalf("AggregateHandoffWindow failed: %v", err)
	}
	if !agg.UsedLegacyFallback {
		t.Errorf("expected UsedLegacyFallback=true")
	}
	if agg.FixedLinksTotal != 3 || agg.BrokenLinksLatest != 1 {
		t.Errorf("unexpected fallback stats: fixed=%d broken=%d", agg.FixedLinksTotal, agg.BrokenLinksLatest)
	}
	if len(agg.Alerts) != 2 {
		t.Errorf("expected 2 fallback alerts, got %v", agg.Alerts)
	}
}

func TestAppendHandoffEvent_PrunesOlderThanRetention(t *testing.T) {
	tempVault := t.TempDir()
	now := time.Now().UTC()

	if err := AppendHandoffEvent(tempVault, HandoffEvent{
		Kind:      HandoffKindDreamCycle,
		Timestamp: now.AddDate(0, 0, -40).Format(time.RFC3339),
		Date:      "2026-10-01",
	}); err != nil {
		t.Fatalf("append old failed: %v", err)
	}
	if err := AppendHandoffEvent(tempVault, HandoffEvent{
		Kind:      HandoffKindDreamCycle,
		Timestamp: now.Format(time.RFC3339),
		Date:      now.Format("2006-01-02"),
	}); err != nil {
		t.Fatalf("append recent failed: %v", err)
	}

	ledgerPath := filepath.Join(tempVault, ".daneel", "cache", "handoff_events.jsonl")
	raw, err := os.ReadFile(ledgerPath) //nolint:gosec
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 1 {
		t.Errorf("expected 1 retained event after pruning >30d, got %d: %s", len(lines), string(raw))
	}
}
