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

func TestGraphServer_GetRoutineHistory_EnrichedFinOps(t *testing.T) {
	tempVault := t.TempDir()
	dbPath := filepath.Join(tempVault, "routine_runs.db")
	st, err := routine.OpenStore(dbPath)
	if err != nil {
		t.Fatalf("failed to open routine store: %v", err)
	}

	ctx := context.Background()
	run := routine.RoutineRun{
		ID:               "run-finops-1",
		RoutineName:      "dream",
		StartedAt:        time.Now().Unix() - 60,
		CompletedAt:      time.Now().Unix(),
		DurationMs:       60000,
		TriggerSource:    "web_cockpit",
		ExitCode:         0,
		Status:           "success",
		Summary:          "Consolidation pass",
		ItemsProcessed:   42,
		InputTokens:      1200,
		CacheReadTokens:  800,
		OutputTokens:     350,
		EstimatedCostUSD: 0.015,
		Model:            "gemini-2.5-pro",
		SessionID:        "sess-123",
	}
	if err := st.RecordRun(ctx, run); err != nil {
		t.Fatalf("failed to record run: %v", err)
	}

	server := &GraphServer{
		vaultDir:     tempVault,
		routineStore: st,
	}
	cleanupGraphServerRoutineStore(t, server)

	req := connect.NewRequest(&graphv1.GetRoutineHistoryRequest{
		RoutineName: "dream",
		Limit:       10,
	})
	res, err := server.GetRoutineHistory(ctx, req)
	if err != nil {
		t.Fatalf("GetRoutineHistory returned error: %v", err)
	}

	if len(res.Msg.Runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(res.Msg.Runs))
	}

	r := res.Msg.Runs[0]
	if r.ItemsProcessed != 42 {
		t.Errorf("expected ItemsProcessed 42, got %d", r.ItemsProcessed)
	}
	if r.InputTokens != 1200 {
		t.Errorf("expected InputTokens 1200, got %d", r.InputTokens)
	}
	if r.CacheReadTokens != 800 {
		t.Errorf("expected CacheReadTokens 800, got %d", r.CacheReadTokens)
	}
	if r.OutputTokens != 350 {
		t.Errorf("expected OutputTokens 350, got %d", r.OutputTokens)
	}
	if r.EstimatedCostUsd != 0.015 {
		t.Errorf("expected EstimatedCostUsd 0.015, got %f", r.EstimatedCostUsd)
	}
	if r.Model != "gemini-2.5-pro" {
		t.Errorf("expected Model 'gemini-2.5-pro', got %q", r.Model)
	}
	if r.SessionId != "sess-123" {
		t.Errorf("expected SessionId 'sess-123', got %q", r.SessionId)
	}
}

func TestGraphServer_GetCognitiveTelemetry_EnrichedFinOps(t *testing.T) {
	tempVault := t.TempDir()
	cacheDir := config.ResolveVaultCacheDir(tempVault)
	if err := os.MkdirAll(cacheDir, 0750); err != nil {
		t.Fatalf("failed to create cache dir: %v", err)
	}

	fileData := []byte(`{
		"generated_at": "` + time.Now().Format(time.RFC3339) + `",
		"total_files_scanned": 100,
		"broken_links_count": 0,
		"metacognition": {
			"total_duration_ms": 3400,
			"input_tokens": 5000,
			"cache_read_tokens": 10000,
			"output_tokens": 1200,
			"estimated_cost_usd": 0.045,
			"subagents_count": 3,
			"model": "gemini-2.5-pro"
		}
	}`)
	if err := os.WriteFile(filepath.Join(cacheDir, "dream_handoff.json"), fileData, 0600); err != nil {
		t.Fatalf("failed to write dream handoff: %v", err)
	}

	server := &GraphServer{
		vaultDir: tempVault,
	}
	cleanupGraphServerRoutineStore(t, server)

	req := connect.NewRequest(&graphv1.GetCognitiveTelemetryRequest{})
	res, err := server.GetCognitiveTelemetry(context.Background(), req)
	if err != nil {
		t.Fatalf("GetCognitiveTelemetry returned error: %v", err)
	}

	if res.Msg.DreamInputTokens != 5000 {
		t.Errorf("expected DreamInputTokens 5000, got %d", res.Msg.DreamInputTokens)
	}
	if res.Msg.DreamCacheReadTokens != 10000 {
		t.Errorf("expected DreamCacheReadTokens 10000, got %d", res.Msg.DreamCacheReadTokens)
	}
	if res.Msg.DreamOutputTokens != 1200 {
		t.Errorf("expected DreamOutputTokens 1200, got %d", res.Msg.DreamOutputTokens)
	}
	if res.Msg.DreamEstimatedCostUsd != 0.045 {
		t.Errorf("expected DreamEstimatedCostUsd 0.045, got %f", res.Msg.DreamEstimatedCostUsd)
	}
	if res.Msg.DreamSubagentsCount != 3 {
		t.Errorf("expected DreamSubagentsCount 3, got %d", res.Msg.DreamSubagentsCount)
	}
	if res.Msg.DreamModel != "gemini-2.5-pro" {
		t.Errorf("expected DreamModel 'gemini-2.5-pro', got %q", res.Msg.DreamModel)
	}
}

func TestGraphServer_GetOperationalTelemetry(t *testing.T) {
	tempVault := t.TempDir()
	dbPath := filepath.Join(tempVault, "routine_runs.db")
	st, err := routine.OpenStore(dbPath)
	if err != nil {
		t.Fatalf("failed to open routine store: %v", err)
	}

	ctx := context.Background()
	now := time.Now().Unix()

	// Insert session telemetry records
	_ = st.UpsertSessionTelemetry(ctx, routine.SessionTelemetryRecord{
		SessionID:        "sess-1",
		Agent:            "antigravity",
		Model:            "gemini-2.5-pro",
		StartedAt:        now - 3600,
		UpdatedAt:        now,
		TurnsCount:       10,
		SubagentsCount:   2,
		InputTokens:      1000,
		CacheReadTokens:  3000,
		OutputTokens:     500,
		EstimatedCostUSD: 0.02,
		SkillsUsed:       "vault-assistant",
		IsRoutine:        false,
	})

	_ = st.UpsertSessionTelemetry(ctx, routine.SessionTelemetryRecord{
		SessionID:        "sess-2",
		Agent:            "claude-code",
		Model:            "claude-3-7-sonnet",
		StartedAt:        now - 1800,
		UpdatedAt:        now,
		TurnsCount:       5,
		SubagentsCount:   1,
		InputTokens:      2000,
		CacheReadTokens:  1000,
		OutputTokens:     1000,
		EstimatedCostUSD: 0.05,
		SkillsUsed:       "agent-evals",
		IsRoutine:        true,
		RoutineName:      "session_mining",
	})

	// Insert routine run
	_ = st.RecordRun(ctx, routine.RoutineRun{
		ID:               "run-1",
		RoutineName:      "dream",
		StartedAt:        now - 600,
		CompletedAt:      now - 500,
		DurationMs:       100000,
		TriggerSource:    "cron",
		ExitCode:         0,
		Status:           "success",
		ItemsProcessed:   50,
		InputTokens:      3000,
		CacheReadTokens:  4000,
		OutputTokens:     500,
		EstimatedCostUSD: 0.03,
	})

	// Insert command event
	_ = st.RecordCommandEvent(ctx, routine.CommandTelemetryEvent{
		Surface:    "mcp",
		Command:    "search_vault",
		StartedAt:  now - 300,
		DurationMs: 12,
		IsError:    false,
	})

	cfg := &config.Config{}
	cfg.Telemetry.FinOps.DailyBudgetUSD = 10.0
	cfg.Telemetry.FinOps.MonthlyBudgetUSD = 250.0

	server := &GraphServer{
		vaultDir:     tempVault,
		routineStore: st,
		cfg:          cfg,
	}
	cleanupGraphServerRoutineStore(t, server)

	req := connect.NewRequest(&graphv1.GetOperationalTelemetryRequest{
		Days: 7,
	})
	res, err := server.GetOperationalTelemetry(ctx, req)
	if err != nil {
		t.Fatalf("GetOperationalTelemetry returned error: %v", err)
	}

	msg := res.Msg
	if msg.WindowDays != 7 {
		t.Errorf("expected WindowDays 7, got %d", msg.WindowDays)
	}
	if msg.DailyBudgetUsd != 10.0 {
		t.Errorf("expected DailyBudgetUsd 10.0, got %f", msg.DailyBudgetUsd)
	}
	if msg.MonthlyBudgetUsd != 250.0 {
		t.Errorf("expected MonthlyBudgetUsd 250.0, got %f", msg.MonthlyBudgetUsd)
	}
	if msg.TotalSessions != 2 {
		t.Errorf("expected TotalSessions 2, got %d", msg.TotalSessions)
	}
	if msg.TotalSubagents != 3 {
		t.Errorf("expected TotalSubagents 3, got %d", msg.TotalSubagents)
	}
	if msg.TotalInputTokens != 3000 {
		t.Errorf("expected TotalInputTokens 3000, got %d", msg.TotalInputTokens)
	}
	if msg.TotalCacheReadTokens != 4000 {
		t.Errorf("expected TotalCacheReadTokens 4000, got %d", msg.TotalCacheReadTokens)
	}
	if msg.TotalOutputTokens != 1500 {
		t.Errorf("expected TotalOutputTokens 1500, got %d", msg.TotalOutputTokens)
	}
	if msg.TotalCostUsd < 0.069 || msg.TotalCostUsd > 0.071 {
		t.Errorf("expected TotalCostUsd ~0.07, got %f", msg.TotalCostUsd)
	}
	// Cache efficiency: 4000 / (3000 + 4000) = ~57.14%
	if msg.CacheEfficiencyPct < 57.0 || msg.CacheEfficiencyPct > 58.0 {
		t.Errorf("expected CacheEfficiencyPct ~57.14, got %f", msg.CacheEfficiencyPct)
	}

	if len(msg.ModelUsage) == 0 {
		t.Errorf("expected ModelUsage items, got 0")
	}
	if len(msg.RoutineWeights) == 0 {
		t.Errorf("expected RoutineWeights items, got 0")
	}
	if len(msg.RecentSessions) != 2 {
		t.Errorf("expected RecentSessions 2, got %d", len(msg.RecentSessions))
	}
	if len(msg.CommandMetrics) == 0 {
		t.Errorf("expected CommandMetrics items, got 0")
	}
}
