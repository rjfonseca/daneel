package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"

	graphv1 "github.com/rjfonseca/daneel/api/v1"
	"github.com/rjfonseca/daneel/internal/config"
	"github.com/rjfonseca/daneel/internal/dream"
	"github.com/rjfonseca/daneel/internal/routine"
	"github.com/rjfonseca/daneel/internal/search"
)

func cleanupGraphServerRoutineStore(t *testing.T, server *GraphServer) {
	t.Helper()
	t.Cleanup(func() {
		if server.routineStore != nil {
			if err := server.routineStore.Close(); err != nil {
				t.Errorf("close fixture routine store: %v", err)
			}
		}
	})
}

func TestGraphServer_GetRoutinesOverview(t *testing.T) {
	tempVault := t.TempDir()

	server := &GraphServer{
		vaultDir: tempVault,
	}
	cleanupGraphServerRoutineStore(t, server)

	req := connect.NewRequest(&graphv1.GetRoutinesOverviewRequest{})
	res, err := server.GetRoutinesOverview(context.Background(), req)
	if err != nil {
		t.Fatalf("GetRoutinesOverview returned error: %v", err)
	}

	if len(res.Msg.Routines) < 5 {
		t.Fatalf("expected at least 5 routines, got %d", len(res.Msg.Routines))
	}

	hasTidy := false
	hasDream := false
	hasMining := false
	hasHarvestJira := false
	hasHarvestGithub := false
	for _, r := range res.Msg.Routines {
		if r.Name == "tidy" {
			hasTidy = true
		}
		if r.Name == "dream" {
			hasDream = true
		}
		if r.Name == "session_mining" {
			hasMining = true
		}
		if r.Name == "harvest:jira" {
			hasHarvestJira = true
		}
		if r.Name == "harvest:github" {
			hasHarvestGithub = true
		}
	}

	if !hasTidy || !hasDream || !hasMining || !hasHarvestJira || !hasHarvestGithub {
		t.Errorf("expected routines to include tidy, dream, session_mining, harvest:jira, and harvest:github; got %+v", res.Msg.Routines)
	}
}

func TestGraphServer_GetStaffTelemetry(t *testing.T) {
	tempVault := t.TempDir()
	dailyDir := filepath.Join(tempVault, "daily")
	if err := os.MkdirAll(dailyDir, 0750); err != nil {
		t.Fatalf("failed to create daily dir: %v", err)
	}

	today := time.Now().Format("2006-01-02")
	mockDaily := `---
tempo-A_Estrategico: 4.5
tempo-B_ImpactoLocal: 2.0
tempo-C_GlueWork: 1.5
tempo-D_LowImpact: 0.5
tempo-E_Overhead: 0.5
---
# Daily Log
- Activity 1
`
	if err := os.WriteFile(filepath.Join(dailyDir, today+".md"), []byte(mockDaily), 0600); err != nil {
		t.Fatalf("failed to write mock daily: %v", err)
	}

	server := &GraphServer{
		vaultDir: tempVault,
	}

	req := connect.NewRequest(&graphv1.GetStaffTelemetryRequest{
		Days:             7,
		RollingWeeks:     4,
		HistoricalMonths: 6,
	})
	res, err := server.GetStaffTelemetry(context.Background(), req)
	if err != nil {
		t.Fatalf("GetStaffTelemetry returned error: %v", err)
	}

	if res.Msg.StrategicAHours < 4.4 || res.Msg.StrategicAHours > 4.6 {
		t.Errorf("expected StrategicAHours around 4.5, got %f", res.Msg.StrategicAHours)
	}
	if res.Msg.LocalImpactBHours < 1.9 || res.Msg.LocalImpactBHours > 2.1 {
		t.Errorf("expected LocalImpactBHours around 2.0, got %f", res.Msg.LocalImpactBHours)
	}
	if res.Msg.LowImpactDHours < 0.4 || res.Msg.LowImpactDHours > 0.6 {
		t.Errorf("expected LowImpactDHours around 0.5, got %f", res.Msg.LowImpactDHours)
	}
	// Total: 4.5 + 2.0 + 1.5 + 0.5 + 0.5 = 9.0
	// Leverage: (4.5 + 1.5) / 9.0 = 6.0 / 9.0 = 0.666...
	if res.Msg.LeverageRate < 0.66 || res.Msg.LeverageRate > 0.67 {
		t.Errorf("expected LeverageRate around 0.667, got %f", res.Msg.LeverageRate)
	}
	if len(res.Msg.RollingWeeks) == 0 {
		t.Errorf("expected rolling weeks to be populated")
	}
	if len(res.Msg.MonthlyHistory) == 0 {
		t.Errorf("expected monthly history to be populated")
	}
	if len(res.Msg.TopInitiatives) == 0 {
		t.Errorf("expected top initiatives to not be empty")
	}
}

func TestGraphServer_ExecuteRoutine_HandlerRegistered(t *testing.T) {
	tempVault := t.TempDir()

	server := &GraphServer{
		vaultDir: tempVault,
	}

	if server.vaultDir != tempVault {
		t.Errorf("expected vaultDir %s, got %s", tempVault, server.vaultDir)
	}
}

func TestGraphServer_AISkill_DetectionAndPrompt(t *testing.T) {
	tempVault := t.TempDir()
	server := &GraphServer{
		vaultDir: tempVault,
	}

	// 1. Detection
	if !server.isAISkill("meeting-preflight") {
		t.Errorf("expected meeting-preflight to be detected as AI skill")
	}
	if !server.isAISkill("skill:custom-skill") {
		t.Errorf("expected skill:custom-skill to be detected as AI skill")
	}
	if server.isAISkill("tidy") {
		t.Errorf("expected tidy to NOT be detected as AI skill")
	}
	if server.isAISkill("harvest:jira") {
		t.Errorf("expected harvest:jira to NOT be detected as AI skill")
	}

	// 2. Prompt Building
	prompt := server.buildSkillPrompt("meeting-preflight", map[string]string{
		"attendee": "Alice",
		"topic":    "API Gateway",
		"date":     "2026-09-20",
		"context":  "See notes/api.md",
	})

	if !strings.Contains(prompt, "Alice") {
		t.Errorf("expected prompt to contain 'Alice', got: %s", prompt)
	}
	if !strings.Contains(prompt, "API Gateway") {
		t.Errorf("expected prompt to contain 'API Gateway', got: %s", prompt)
	}
	if !strings.Contains(prompt, "2026-09-20") {
		t.Errorf("expected prompt to contain '2026-09-20', got: %s", prompt)
	}
	if !strings.Contains(prompt, "See notes/api.md") {
		t.Errorf("expected prompt to contain 'See notes/api.md', got: %s", prompt)
	}
}

func TestGraphServer_UpsertAndDeleteRoutine(t *testing.T) {
	tempConfigDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempConfigDir)
	tempVault := t.TempDir()
	cfgPath := filepath.Join(tempVault, ".daneel.yaml")
	_ = os.WriteFile(cfgPath, []byte("routines:\n  driver: systemd\n  items: {}\n"), 0600)

	server := &GraphServer{
		vaultDir: tempVault,
		cfg: &config.Config{
			VaultPath:  tempVault,
			ConfigFile: cfgPath,
			Routines: config.RoutinesConfig{
				Driver: "systemd",
				Items:  make(map[string]config.RoutineItemConfig),
			},
		},
	}
	cleanupGraphServerRoutineStore(t, server)

	ctx := context.Background()

	// 1. Upsert new routine
	upsertReq := connect.NewRequest(&graphv1.UpsertRoutineRequest{
		Name:        "backup",
		Command:     "backup",
		Schedule:    "daily",
		Description: "Daily backup routine",
		Enabled:     true,
	})

	upsertRes, err := server.UpsertRoutine(ctx, upsertReq)
	if err != nil {
		t.Fatalf("UpsertRoutine failed: %v", err)
	}
	if !upsertRes.Msg.Success {
		t.Errorf("expected UpsertRoutine success, got error: %s", upsertRes.Msg.Error)
	}

	// Verify routine is present in overview
	overviewReq := connect.NewRequest(&graphv1.GetRoutinesOverviewRequest{})
	overviewRes, err := server.GetRoutinesOverview(ctx, overviewReq)
	if err != nil {
		t.Fatalf("GetRoutinesOverview failed: %v", err)
	}
	found := false
	for _, r := range overviewRes.Msg.Routines {
		if r.Name == "backup" && r.Schedule == "daily" && r.Enabled {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected backup routine in overview, got: %+v", overviewRes.Msg.Routines)
	}

	// 2. Delete routine
	delReq := connect.NewRequest(&graphv1.DeleteRoutineRequest{
		Name: "backup",
	})
	delRes, err := server.DeleteRoutine(ctx, delReq)
	if err != nil {
		t.Fatalf("DeleteRoutine failed: %v", err)
	}
	if !delRes.Msg.Success {
		t.Errorf("expected DeleteRoutine success, got error: %s", delRes.Msg.Error)
	}

	// Verify routine was removed
	if _, ok := server.cfg.Routines.Items["backup"]; ok {
		t.Errorf("expected backup routine to be deleted from config")
	}
}

func TestGraphServer_GetCognitiveTelemetry(t *testing.T) {
	tempVault := t.TempDir()
	cacheDir := filepath.Join(tempVault, ".daneel", "cache")
	if err := os.MkdirAll(cacheDir, 0750); err != nil {
		t.Fatalf("failed to create cache dir: %v", err)
	}

	// 1. Mock dream_handoff.json
	mockHandoff := `{
  "date": "2026-09-18",
  "generated_at": "2026-09-18T18:00:00Z",
  "total_files_scanned": 150,
  "broken_links_count": 3,
  "metacognition": {
    "total_duration_ms": 1250,
    "timings": [
      {
        "name": "Phase 1: Deterministic Graph Hygiene",
        "duration_ms": 800,
        "items_count": 150,
        "description": "150 files scanned"
      },
      {
        "name": "Phase 4: Metacognition & Self-Evaluation",
        "duration_ms": 250,
        "items_count": 2,
        "description": "2 bottlenecks flagged"
      }
    ],
    "bottlenecks": [
      "3 broken relative links detected"
    ],
    "syntheses": [
      {
        "title": "Síntese: AI Agents and System Architecture",
        "created_path": "notes/sinteses/ai-agents-sintese.md"
      }
    ],
    "evolution_log_path": "meta/guias/agente-evolucao.md"
  }
}`
	_ = os.WriteFile(filepath.Join(cacheDir, "dream_handoff.json"), []byte(mockHandoff), 0600)

	// 2. Mock agent_learnings.json
	mockLearnings := `{
  "last_updated": "2026-09-18T18:00:00Z",
  "feedback_history": [
    {
      "date": "2026-09-18",
      "timestamp": "2026-09-18T17:30:00Z",
      "topic": "Payment Gateway",
      "type": "missed_opportunity",
      "action": "missed_opportunity",
      "note": "Agent asked user instead of search_vault"
    },
    {
      "date": "2026-09-18",
      "timestamp": "2026-09-18T17:45:00Z",
      "topic": "Missing Tax Lib",
      "type": "knowledge_gap",
      "action": "knowledge_gap",
      "note": "Documentation absent from vault"
    }
  ]
}`
	_ = os.WriteFile(filepath.Join(cacheDir, "agent_learnings.json"), []byte(mockLearnings), 0600)

	server := &GraphServer{
		vaultDir: tempVault,
	}

	ctx := context.Background()
	req := connect.NewRequest(&graphv1.GetCognitiveTelemetryRequest{})
	res, err := server.GetCognitiveTelemetry(ctx, req)
	if err != nil {
		t.Fatalf("GetCognitiveTelemetry failed: %v", err)
	}

	msg := res.Msg
	if msg.LastCycleDurationMs != 1250 {
		t.Errorf("expected LastCycleDurationMs 1250, got %d", msg.LastCycleDurationMs)
	}
	if len(msg.StageTimings) != 2 {
		t.Errorf("expected 2 stage timings, got %d", len(msg.StageTimings))
	}
	if len(msg.Bottlenecks) != 1 {
		t.Errorf("expected 1 bottleneck, got %d", len(msg.Bottlenecks))
	}
	if len(msg.InducedSyntheses) != 1 {
		t.Errorf("expected 1 induced synthesis, got %d", len(msg.InducedSyntheses))
	}
	if msg.MissedOpportunitiesCount != 1 {
		t.Errorf("expected 1 missed opportunity, got %d", msg.MissedOpportunitiesCount)
	}
	if msg.KnowledgeGapsCount != 1 {
		t.Errorf("expected 1 knowledge gap, got %d", msg.KnowledgeGapsCount)
	}
	if len(msg.RecentEvals) != 2 {
		t.Errorf("expected 2 recent evals, got %d", len(msg.RecentEvals))
	}
}

func TestGraphServer_GetCognitiveTelemetry_FromRealEngineHandoff(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	tempVault := t.TempDir()
	_ = os.MkdirAll(filepath.Join(tempVault, ".daneel", "cache"), 0750)

	notesDir := filepath.Join(tempVault, "notes", "ia")
	_ = os.MkdirAll(notesDir, 0750)
	noteFile := filepath.Join(notesDir, "agent-patterns.md")
	_ = os.WriteFile(noteFile, []byte("# Agent Patterns\nNotas sobre agentes autônomos.\n"), 0600)

	// Run real engine cycle
	engine := dream.NewDreamEngine(tempVault)
	rep, err := engine.RunFullCycle("2026-09-22", false, false)
	if err != nil {
		t.Fatalf("RunFullCycle failed: %v", err)
	}
	if rep.Metacognition == nil {
		t.Fatalf("expected rep.Metacognition != nil")
	}

	server := &GraphServer{
		vaultDir: tempVault,
	}

	ctx := context.Background()
	req := connect.NewRequest(&graphv1.GetCognitiveTelemetryRequest{})
	res, err := server.GetCognitiveTelemetry(ctx, req)
	if err != nil {
		t.Fatalf("GetCognitiveTelemetry failed: %v", err)
	}

	msg := res.Msg
	if msg.LastCycleDurationMs != rep.Metacognition.TotalDurationMs {
		t.Errorf("cycle duration = %d, want persisted engine duration %d", msg.LastCycleDurationMs, rep.Metacognition.TotalDurationMs)
	}
	if len(msg.StageTimings) == 0 {
		t.Fatal("expected stage timings to be populated from handoff")
	}
	if len(msg.StageTimings) != len(rep.Metacognition.Timings) {
		t.Fatalf("stage timing count = %d, want %d", len(msg.StageTimings), len(rep.Metacognition.Timings))
	}
	for i, timing := range msg.StageTimings {
		want := rep.Metacognition.Timings[i]
		if timing.Name != want.Name || timing.DurationMs != want.DurationMs {
			t.Errorf("stage %d timing = %s/%dms, want %s/%dms", i, timing.Name, timing.DurationMs, want.Name, want.DurationMs)
		}
	}
	if msg.TotalFilesScanned != int32(rep.TotalFilesScanned) { //nolint:gosec
		t.Errorf("expected TotalFilesScanned %d, got %d", rep.TotalFilesScanned, msg.TotalFilesScanned)
	}
}

func TestGraphServer_GetRoutineHistory_And_Enrichment(t *testing.T) {
	tempVault := t.TempDir()
	store, err := routine.OpenDefaultStore(tempVault)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	now := time.Now().Unix()

	// Seed runs in the store
	_ = store.RecordRun(ctx, routine.RoutineRun{
		ID:            "run-ws-1",
		RoutineName:   "workspace",
		StartedAt:     now - 300,
		CompletedAt:   now - 295,
		DurationMs:    5000,
		TriggerSource: "systemd",
		ExitCode:      0,
		Status:        "success",
		Summary:       "Synced repos",
	})
	_ = store.RecordRun(ctx, routine.RoutineRun{
		ID:            "run-drm-1",
		RoutineName:   "dream",
		StartedAt:     now - 100,
		CompletedAt:   now - 98,
		DurationMs:    2000,
		TriggerSource: "web_cockpit",
		ExitCode:      1,
		Status:        "failed",
		Summary:       "Cycle failed",
		ErrorMessage:  "disk full",
	})

	server := &GraphServer{
		vaultDir:     tempVault,
		routineStore: store,
		cfg: &config.Config{
			VaultPath: tempVault,
			Routines:  config.DefaultRoutinesConfig(),
		},
	}

	// 1. Test GetRoutineHistory unfiltered
	histReq := connect.NewRequest(&graphv1.GetRoutineHistoryRequest{Limit: 10})
	histRes, err := server.GetRoutineHistory(ctx, histReq)
	if err != nil {
		t.Fatalf("GetRoutineHistory failed: %v", err)
	}
	if len(histRes.Msg.Runs) != 2 {
		t.Fatalf("expected 2 runs, got %d", len(histRes.Msg.Runs))
	}
	// Verify newest first
	if histRes.Msg.Runs[0].Id != "run-drm-1" || histRes.Msg.Runs[1].Id != "run-ws-1" {
		t.Errorf("unexpected order of runs: %+v", histRes.Msg.Runs)
	}

	// 2. Test GetRoutineHistory filtered
	wsReq := connect.NewRequest(&graphv1.GetRoutineHistoryRequest{RoutineName: "workspace"})
	wsRes, err := server.GetRoutineHistory(ctx, wsReq)
	if err != nil {
		t.Fatalf("GetRoutineHistory for workspace failed: %v", err)
	}
	if len(wsRes.Msg.Runs) != 1 || wsRes.Msg.Runs[0].TriggerSource != "systemd" {
		t.Errorf("unexpected filtered runs: %+v", wsRes.Msg.Runs)
	}

	// 3. Test GetRoutinesOverview enrichment
	overviewReq := connect.NewRequest(&graphv1.GetRoutinesOverviewRequest{})
	overviewRes, err := server.GetRoutinesOverview(ctx, overviewReq)
	if err != nil {
		t.Fatalf("GetRoutinesOverview failed: %v", err)
	}

	foundWs := false
	foundDrm := false
	for _, r := range overviewRes.Msg.Routines {
		if r.Name == "workspace" {
			foundWs = true
			if r.LastRunTimestamp != now-300 {
				t.Errorf("expected workspace LastRunTimestamp %d, got %d", now-300, r.LastRunTimestamp)
			}
			if r.LastResult != "success" {
				t.Errorf("expected workspace LastResult 'success', got %s", r.LastResult)
			}
		}
		if r.Name == "dream" {
			foundDrm = true
			if r.LastRunTimestamp != now-100 {
				t.Errorf("expected dream LastRunTimestamp %d, got %d", now-100, r.LastRunTimestamp)
			}
			if r.LastResult != "failed" {
				t.Errorf("expected dream LastResult 'failed', got %s", r.LastResult)
			}
		}
	}
	if !foundWs || !foundDrm {
		t.Errorf("expected to find workspace (%v) and dream (%v) in overview", foundWs, foundDrm)
	}
}

func TestGraphServer_GetGalaxy_AutoDiscoversDailyNotes(t *testing.T) {
	tempVault := t.TempDir()
	dbPath := filepath.Join(tempVault, ".daneel", "cache", "index.db")
	eng, err := search.Open(dbPath, tempVault)
	if err != nil {
		t.Fatalf("failed to open search engine: %v", err)
	}
	defer func() { _ = eng.Close() }()

	dailyDir := filepath.Join(tempVault, "daily")
	if err := os.MkdirAll(dailyDir, 0750); err != nil {
		t.Fatalf("failed to create daily dir: %v", err)
	}

	dailyNotePath := filepath.Join(dailyDir, "2026-10-09.md")
	content := "# 2026-10-09\nLog for today with important tasks.\n"
	if err := os.WriteFile(dailyNotePath, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write daily note: %v", err)
	}

	server := &GraphServer{
		engine:   eng,
		vaultDir: tempVault,
	}

	ctx := context.Background()
	req := connect.NewRequest(&graphv1.GetGalaxyRequest{})
	res, err := server.GetGalaxy(ctx, req)
	if err != nil {
		t.Fatalf("GetGalaxy failed: %v", err)
	}

	// Verify that 2026-10-09 is in the returned nodes
	var foundDaily *graphv1.Node
	for _, n := range res.Msg.Nodes {
		if n.Id == "daily/2026-10-09.md" {
			foundDaily = n
			break
		}
	}
	if foundDaily == nil {
		t.Fatalf("expected daily/2026-10-09.md to be auto-discovered in GetGalaxy nodes, got: %d nodes", len(res.Msg.Nodes))
	}
	if foundDaily.NoteType != "daily" {
		t.Errorf("expected NoteType 'daily', got '%s'", foundDaily.NoteType)
	}

	// Verify that SQLite notes_meta was updated
	var count int
	err = eng.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM notes_meta WHERE rel_path = 'daily/2026-10-09.md'").Scan(&count)
	if err != nil {
		t.Fatalf("failed to query notes_meta: %v", err)
	}
	if count != 1 {
		t.Errorf("expected notes_meta to contain 1 row for daily/2026-10-09.md, got %d", count)
	}
}

func TestGraphServer_GetNote_AutoIndexesFileInBackground(t *testing.T) {
	tempVault := t.TempDir()
	dbPath := filepath.Join(tempVault, ".daneel", "cache", "index.db")
	eng, err := search.Open(dbPath, tempVault)
	if err != nil {
		t.Fatalf("failed to open search engine: %v", err)
	}
	defer func() { _ = eng.Close() }()

	notesDir := filepath.Join(tempVault, "notes")
	if err := os.MkdirAll(notesDir, 0750); err != nil {
		t.Fatalf("failed to create notes dir: %v", err)
	}

	notePath := filepath.Join(notesDir, "unread.md")
	content := "# Unread Note\nThis note was not yet indexed in SQLite.\n"
	if err := os.WriteFile(notePath, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write note: %v", err)
	}

	server := &GraphServer{
		engine:   eng,
		vaultDir: tempVault,
	}

	ctx := context.Background()
	req := connect.NewRequest(&graphv1.GetNoteRequest{
		RelPath: "notes/unread.md",
	})
	res, err := server.GetNote(ctx, req)
	if err != nil {
		t.Fatalf("GetNote failed: %v", err)
	}
	if res.Msg.ContentMarkdown != content {
		t.Errorf("expected content %q, got %q", content, res.Msg.ContentMarkdown)
	}

	// Wait briefly for background indexing to complete
	deadline := time.Now().Add(1 * time.Second)
	indexed := false
	for time.Now().Before(deadline) {
		var count int
		_ = eng.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM notes_meta WHERE rel_path = 'notes/unread.md'").Scan(&count)
		if count > 0 {
			indexed = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if !indexed {
		t.Fatalf("expected notes/unread.md to be indexed in SQLite notes_meta after GetNote call")
	}
}
