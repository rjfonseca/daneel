package telemetry_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rjfonseca/daneel/internal/config"
	"github.com/rjfonseca/daneel/internal/routine"
	"github.com/rjfonseca/daneel/internal/telemetry"
)

func TestParseAntigravitySession_ParentAndChildSubagent(t *testing.T) {
	tempBrainDir := t.TempDir()
	parentID := "parent-conv-001"
	childID := "child-conv-002"

	// 1. Create parent transcript
	parentLogsDir := filepath.Join(tempBrainDir, parentID, ".system_generated", "logs")
	if err := os.MkdirAll(parentLogsDir, 0750); err != nil {
		t.Fatalf("failed to create parent logs dir: %v", err)
	}

	t0 := time.Date(2026, 10, 28, 2, 0, 0, 0, time.UTC)
	t1 := t0.Add(1 * time.Minute)
	t2 := t0.Add(2 * time.Minute)

	parentTranscript := []string{
		fmt.Sprintf(`{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":%q,"content":"<USER_REQUEST>\nExecute the complete Autonomous Second Brain Strategic Dream Cycle protocol\n</USER_REQUEST>\n<USER_SETTINGS_CHANGE>\nThe user changed setting 'Model Selection' from None to Gemini 4 Argon (Medium).\n</USER_SETTINGS_CHANGE>"}`, t0.Format(time.RFC3339)),
		fmt.Sprintf(`{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":%q,"input_tokens":10000,"cache_read_tokens":5000,"output_tokens":800,"tool_calls":[{"name":"read_file","args":{"Path":"/vault/.agents/skills/vault-dream-cycle/SKILL.md"}},{"name":"invoke_subagent","args":{"Model":"\"flash\"","Role":"\"AI Session FinOps Parser Engineer\""}}]}`, t1.Format(time.RFC3339)),
		fmt.Sprintf(`{"step_index":2,"source":"MODEL","type":"GENERIC","status":"DONE","created_at":%q,"content":"Created the following subagent:\n{\n  \"conversationId\":  \"%s\",\n  \"logAbsoluteUri\":  \"file:///path/to/transcript.jsonl\"\n}"}`, t2.Format(time.RFC3339), childID),
	}

	parentFile := filepath.Join(parentLogsDir, "transcript.jsonl")
	if err := os.WriteFile(parentFile, []byte(joinLines(parentTranscript)), 0600); err != nil {
		t.Fatalf("failed to write parent transcript: %v", err)
	}

	// 2. Create child transcript
	childLogsDir := filepath.Join(tempBrainDir, childID)
	if err := os.MkdirAll(childLogsDir, 0750); err != nil {
		t.Fatalf("failed to create child logs dir: %v", err)
	}

	c0 := t0.Add(3 * time.Minute)
	c1 := t0.Add(4 * time.Minute)
	childTranscript := []string{
		fmt.Sprintf(`{"step_index":0,"source":"SYSTEM","type":"SYSTEM_MESSAGE","status":"DONE","created_at":%q,"content":"Run daneel doctor and fix issues"}`, c0.Format(time.RFC3339)),
		fmt.Sprintf(`{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":%q,"input_tokens":4000,"cache_read_tokens":2000,"output_tokens":400,"tool_calls":[{"name":"run_command","args":{"CommandLine":"daneel doctor --fix"}}]}`, c1.Format(time.RFC3339)),
	}

	childFile := filepath.Join(childLogsDir, "transcript.jsonl")
	if err := os.WriteFile(childFile, []byte(joinLines(childTranscript)), 0600); err != nil {
		t.Fatalf("failed to write child transcript: %v", err)
	}

	cfg := &config.Config{
		Telemetry: config.TelemetryConfig{
			FinOps: config.DefaultFinOpsConfig(),
		},
	}

	opts := telemetry.ParserOptions{
		BrainDir: tempBrainDir,
		Since:    24 * time.Hour,
		Config:   cfg,
	}

	summaries, err := telemetry.SyncSessionsToStore(context.Background(), nil, opts)
	if err != nil {
		t.Fatalf("SyncSessionsToStore failed: %v", err)
	}

	if len(summaries) != 2 {
		t.Fatalf("expected 2 summaries, got %d", len(summaries))
	}

	var parentSummary, childSummary *telemetry.SessionSummary
	for i := range summaries {
		switch summaries[i].Record.SessionID {
		case parentID:
			parentSummary = &summaries[i]
		case childID:
			childSummary = &summaries[i]
		}
	}

	if parentSummary == nil || childSummary == nil {
		t.Fatalf("expected parent and child summaries to be present")
	}

	// Verify Parent Record
	if parentSummary.Record.Model != "gemini-pro" {
		t.Errorf("expected parent model gemini-pro, got %s", parentSummary.Record.Model)
	}
	if !parentSummary.Record.IsRoutine {
		t.Errorf("expected parent IsRoutine to be true")
	}
	if parentSummary.Record.RoutineName != "dream" {
		t.Errorf("expected parent RoutineName dream, got %s", parentSummary.Record.RoutineName)
	}
	if parentSummary.Record.SubagentsCount != 1 {
		t.Errorf("expected parent SubagentsCount 1, got %d", parentSummary.Record.SubagentsCount)
	}
	if parentSummary.Record.SkillsUsed != "vault-dream-cycle" {
		t.Errorf("expected parent SkillsUsed vault-dream-cycle, got %s", parentSummary.Record.SkillsUsed)
	}

	// Verify Child Record
	if childSummary.Record.ParentSessionID != parentID {
		t.Errorf("expected child ParentSessionID %s, got %s", parentID, childSummary.Record.ParentSessionID)
	}
	if childSummary.Record.Model != "gemini-flash" {
		t.Errorf("expected child model gemini-flash, got %s", childSummary.Record.Model)
	}
	if childSummary.Record.Role != "AI Session FinOps Parser Engineer" {
		t.Errorf("expected child role 'AI Session FinOps Parser Engineer', got %s", childSummary.Record.Role)
	}
	if childSummary.Record.SkillsUsed != "daneel" {
		t.Errorf("expected child SkillsUsed daneel, got %s", childSummary.Record.SkillsUsed)
	}

	// Verify Rollup
	// Parent own tokens: 10000 in, 5000 cache, 800 out
	// Child own tokens: 4000 in, 2000 cache, 400 out
	expectedTotalIn := int64(10000 + 4000)
	expectedTotalCache := int64(5000 + 2000)
	expectedTotalOut := int64(800 + 400)

	if parentSummary.TotalInputTokens != expectedTotalIn {
		t.Errorf("expected parent TotalInputTokens %d, got %d", expectedTotalIn, parentSummary.TotalInputTokens)
	}
	if parentSummary.TotalCacheTokens != expectedTotalCache {
		t.Errorf("expected parent TotalCacheTokens %d, got %d", expectedTotalCache, parentSummary.TotalCacheTokens)
	}
	if parentSummary.TotalOutputTokens != expectedTotalOut {
		t.Errorf("expected parent TotalOutputTokens %d, got %d", expectedTotalOut, parentSummary.TotalOutputTokens)
	}

	parentCost := cfg.CalculateCostUSD("gemini-pro", 10000, 5000, 800)
	childCost := cfg.CalculateCostUSD("gemini-flash", 4000, 2000, 400)
	expectedTotalCost := parentCost + childCost

	if diff := parentSummary.TotalCostUSD - expectedTotalCost; diff < -1e-6 || diff > 1e-6 {
		t.Errorf("expected parent TotalCostUSD %f, got %f", expectedTotalCost, parentSummary.TotalCostUSD)
	}

	if len(parentSummary.ChildSessionIDs) != 1 || parentSummary.ChildSessionIDs[0] != childID {
		t.Errorf("expected parent ChildSessionIDs [%s], got %v", childID, parentSummary.ChildSessionIDs)
	}
}

func TestParseAntigravity_ModelVariants(t *testing.T) {
	tempBrainDir := t.TempDir()
	sessID := "flash-lite-conv-001"

	sessLogsDir := filepath.Join(tempBrainDir, sessID, ".system_generated", "logs")
	_ = os.MkdirAll(sessLogsDir, 0750)

	t0 := time.Now().Add(-10 * time.Minute)
	content := fmt.Sprintf(`{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":%q,"content":"<USER_SETTINGS_CHANGE>\nThe user changed setting 'Model Selection' to Flash Lite.\n</USER_SETTINGS_CHANGE>"}`+"\n", t0.Format(time.RFC3339))
	_ = os.WriteFile(filepath.Join(sessLogsDir, "transcript.jsonl"), []byte(content), 0600)

	cfg := &config.Config{
		Telemetry: config.TelemetryConfig{
			FinOps: config.DefaultFinOpsConfig(),
		},
	}

	opts := telemetry.ParserOptions{
		BrainDir: tempBrainDir,
		Since:    24 * time.Hour,
		Config:   cfg,
	}

	summaries, err := telemetry.SyncSessionsToStore(context.Background(), nil, opts)
	if err != nil {
		t.Fatalf("SyncSessionsToStore failed: %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(summaries))
	}
	if summaries[0].Record.Model != "gemini-flash-lite" {
		t.Errorf("expected model gemini-flash-lite, got %s", summaries[0].Record.Model)
	}
}

func TestFindSessionInWindow(t *testing.T) {
	t0 := time.Date(2026, 10, 28, 3, 0, 0, 0, time.UTC)
	summaries := []telemetry.SessionSummary{
		{
			Record: routine.SessionTelemetryRecord{
				SessionID:   "routine-session",
				RoutineName: "dream",
				IsRoutine:   true,
				StartedAt:   t0.Unix(),
				UpdatedAt:   t0.Add(5 * time.Minute).Unix(),
			},
			TotalCostUSD: 0.05,
		},
		{
			Record: routine.SessionTelemetryRecord{
				SessionID:       "child-session",
				ParentSessionID: "routine-session",
				StartedAt:       t0.Add(1 * time.Minute).Unix(),
				UpdatedAt:       t0.Add(2 * time.Minute).Unix(),
			},
			TotalCostUSD: 0.01,
		},
		{
			Record: routine.SessionTelemetryRecord{
				SessionID: "other-session",
				StartedAt: t0.Add(2 * time.Hour).Unix(),
				UpdatedAt: t0.Add(2*time.Hour + 10*time.Minute).Unix(),
			},
			TotalCostUSD: 0.02,
		},
	}

	// 1. Match by routineName
	match := telemetry.FindSessionInWindow(summaries, t0.Add(-10*time.Minute), t0.Add(10*time.Minute), "dream")
	if match == nil || match.Record.SessionID != "routine-session" {
		t.Fatalf("expected to match routine-session by routine name, got %v", match)
	}

	// 2. Match by window overlapping with preference for root session
	matchWindow := telemetry.FindSessionInWindow(summaries, t0.Add(30*time.Second), t0.Add(4*time.Minute), "")
	if matchWindow == nil || matchWindow.Record.SessionID != "routine-session" {
		t.Fatalf("expected to match root routine-session in window, got %v", matchWindow)
	}

	// 3. No match when completely out of window
	noMatch := telemetry.FindSessionInWindow(summaries, t0.Add(10*time.Hour), t0.Add(11*time.Hour), "nonexistent")
	if noMatch != nil {
		t.Fatalf("expected nil when no session in window, got %v", noMatch)
	}
}

func TestSyncSessionsToStore(t *testing.T) {
	tempBrainDir := t.TempDir()
	tempDBDir := t.TempDir()
	dbPath := filepath.Join(tempDBDir, "routine_runs.db")

	store, err := routine.OpenStore(dbPath)
	if err != nil {
		t.Fatalf("failed to open test SQLite store: %v", err)
	}
	defer func() { _ = store.Close() }()

	sessID := "test-sync-session"
	sessLogsDir := filepath.Join(tempBrainDir, sessID)
	_ = os.MkdirAll(sessLogsDir, 0750)

	t0 := time.Now().Add(-5 * time.Minute)
	transcript := fmt.Sprintf(`{"step_index":0,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":%q,"input_tokens":5000,"cache_read_tokens":1000,"output_tokens":200,"tool_calls":[]}`+"\n", t0.Format(time.RFC3339))
	_ = os.WriteFile(filepath.Join(sessLogsDir, "transcript.jsonl"), []byte(transcript), 0600)

	cfg := &config.Config{
		Telemetry: config.TelemetryConfig{
			FinOps: config.DefaultFinOpsConfig(),
		},
	}

	opts := telemetry.ParserOptions{
		BrainDir: tempBrainDir,
		Since:    1 * time.Hour,
		Config:   cfg,
	}

	ctx := context.Background()
	summaries, err := telemetry.SyncSessionsToStore(ctx, store, opts)
	if err != nil {
		t.Fatalf("SyncSessionsToStore failed: %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(summaries))
	}

	// Verify persistence in SQLite
	records, err := store.GetRecentSessionsTelemetry(ctx, 0, 10)
	if err != nil {
		t.Fatalf("GetRecentSessionsTelemetry failed: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 persisted record, got %d", len(records))
	}
	if records[0].SessionID != sessID {
		t.Errorf("expected session ID %s, got %s", sessID, records[0].SessionID)
	}
	if records[0].InputTokens != 5000 || records[0].OutputTokens != 200 {
		t.Errorf("unexpected tokens in persisted record: %+v", records[0])
	}
}

func TestParseClaudeAndSpooler(t *testing.T) {
	tempClaudeDir := t.TempDir()
	tempSpoolDir := t.TempDir()

	// 1. Create a Claude Code project log
	claudeProjDir := filepath.Join(tempClaudeDir, "projects", "my-project")
	_ = os.MkdirAll(claudeProjDir, 0750)
	claudeFile := filepath.Join(claudeProjDir, "claude-session-001.jsonl")

	t0 := time.Now().Add(-15 * time.Minute)
	claudeLines := []string{
		fmt.Sprintf(`{"timestamp":%q,"model":"claude-3-5-sonnet-20241022","usage":{"input_tokens":1200,"cache_read_input_tokens":300,"output_tokens":150}}`, t0.Format(time.RFC3339)),
	}
	_ = os.WriteFile(claudeFile, []byte(joinLines(claudeLines)), 0600)

	// 2. Create a Spooler entry without explicit usage (testing character length fallback)
	spoolFile := filepath.Join(tempSpoolDir, "spool-session-002.jsonl")
	payload, _ := json.Marshal(map[string]interface{}{
		"prompt": "Hello world this is a test prompt that is 56 characters long", // 56 chars / 4 = 14 tokens
		"result": "Here is a result output of exactly 44 chars",                  // 44 chars / 4 = 11 tokens
	})
	spoolEntry := fmt.Sprintf(`{"session_id":"spool-session-002","agent":"codex","timestamp":%q,"payload":%s}`+"\n", t0.Add(5*time.Minute).Format(time.RFC3339), string(payload))
	_ = os.WriteFile(spoolFile, []byte(spoolEntry), 0600)

	cfg := &config.Config{
		Telemetry: config.TelemetryConfig{
			FinOps: config.DefaultFinOpsConfig(),
		},
	}

	opts := telemetry.ParserOptions{
		ClaudeDir: tempClaudeDir,
		SpoolDir:  tempSpoolDir,
		Since:     1 * time.Hour,
		Config:    cfg,
	}

	summaries, err := telemetry.SyncSessionsToStore(context.Background(), nil, opts)
	if err != nil {
		t.Fatalf("SyncSessionsToStore failed: %v", err)
	}

	if len(summaries) != 2 {
		t.Fatalf("expected 2 summaries from Claude and Spooler, got %d", len(summaries))
	}

	var claudeSum, spoolSum *telemetry.SessionSummary
	for i := range summaries {
		switch summaries[i].Record.SessionID {
		case "claude-session-001":
			claudeSum = &summaries[i]
		case "spool-session-002":
			spoolSum = &summaries[i]
		}
	}

	if claudeSum == nil || spoolSum == nil {
		t.Fatalf("expected both claudeSum and spoolSum to be parsed")
	}

	if claudeSum.Record.InputTokens != 1200 || claudeSum.Record.OutputTokens != 150 {
		t.Errorf("unexpected tokens in claudeSum: %+v", claudeSum.Record)
	}
	if spoolSum.Record.InputTokens != 15 || spoolSum.Record.OutputTokens != 10 {
		t.Errorf("unexpected tokens in spoolSum fallback estimation: in=%d, out=%d", spoolSum.Record.InputTokens, spoolSum.Record.OutputTokens)
	}
}

func TestParseAntigravity_MultiLevelHierarchyAndInherit(t *testing.T) {
	tempBrainDir := t.TempDir()
	rootID := "root-agent-1"
	childID := "child-agent-2"
	grandchildID := "grandchild-agent-3"

	t0 := time.Now().Add(-30 * time.Minute)

	// 1. Root session (model = gemini-pro)
	rootLogsDir := filepath.Join(tempBrainDir, rootID, ".system_generated", "logs")
	_ = os.MkdirAll(rootLogsDir, 0750)
	rootLines := []string{
		fmt.Sprintf(`{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":%q,"content":"<USER_SETTINGS_CHANGE>\nThe user changed setting 'Model Selection' to Gemini 4 Argon (Medium).\n</USER_SETTINGS_CHANGE>"}`, t0.Format(time.RFC3339)),
		fmt.Sprintf(`{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":%q,"input_tokens":1000,"output_tokens":100,"tool_calls":[{"name":"invoke_subagent","args":{"Model":"\"inherit\"","Role":"\"Mid-level Coordinator\""}}]}`, t0.Add(1*time.Minute).Format(time.RFC3339)),
		fmt.Sprintf(`{"step_index":2,"source":"MODEL","type":"GENERIC","status":"DONE","created_at":%q,"content":"Created the following subagent:\n{\n  \"conversationId\":  \"%s\"\n}"}`, t0.Add(2*time.Minute).Format(time.RFC3339), childID),
	}
	_ = os.WriteFile(filepath.Join(rootLogsDir, "transcript.jsonl"), []byte(joinLines(rootLines)), 0600)

	// 2. Child session (inherits root model, invokes grandchild with "flash_lite")
	childLogsDir := filepath.Join(tempBrainDir, childID, ".system_generated", "logs")
	_ = os.MkdirAll(childLogsDir, 0750)
	childLines := []string{
		fmt.Sprintf(`{"step_index":0,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":%q,"input_tokens":500,"output_tokens":50,"tool_calls":[{"name":"invoke_subagent","args":{"Model":"\"flash_lite\"","Role":"\"Worker Leaf\""}}]}`, t0.Add(3*time.Minute).Format(time.RFC3339)),
		fmt.Sprintf(`{"step_index":1,"source":"MODEL","type":"GENERIC","status":"DONE","created_at":%q,"content":"Created the following subagent:\n{\n  \"conversationId\":  \"%s\"\n}"}`, t0.Add(4*time.Minute).Format(time.RFC3339), grandchildID),
	}
	_ = os.WriteFile(filepath.Join(childLogsDir, "transcript.jsonl"), []byte(joinLines(childLines)), 0600)

	// 3. Grandchild session
	grandchildLogsDir := filepath.Join(tempBrainDir, grandchildID, ".system_generated", "logs")
	_ = os.MkdirAll(grandchildLogsDir, 0750)
	grandchildLines := []string{
		fmt.Sprintf(`{"step_index":0,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":%q,"input_tokens":200,"output_tokens":20,"tool_calls":[]}`, t0.Add(5*time.Minute).Format(time.RFC3339)),
	}
	_ = os.WriteFile(filepath.Join(grandchildLogsDir, "transcript.jsonl"), []byte(joinLines(grandchildLines)), 0600)

	cfg := &config.Config{
		Telemetry: config.TelemetryConfig{
			FinOps: config.DefaultFinOpsConfig(),
		},
	}

	opts := telemetry.ParserOptions{
		BrainDir: tempBrainDir,
		Since:    2 * time.Hour,
		Config:   cfg,
	}

	summaries, err := telemetry.SyncSessionsToStore(context.Background(), nil, opts)
	if err != nil {
		t.Fatalf("SyncSessionsToStore failed: %v", err)
	}

	if len(summaries) != 3 {
		t.Fatalf("expected 3 summaries, got %d", len(summaries))
	}

	var rootSum, childSum, grandSum *telemetry.SessionSummary
	for i := range summaries {
		switch summaries[i].Record.SessionID {
		case rootID:
			rootSum = &summaries[i]
		case childID:
			childSum = &summaries[i]
		case grandchildID:
			grandSum = &summaries[i]
		}
	}

	if rootSum == nil || childSum == nil || grandSum == nil {
		t.Fatalf("missing one of the 3 sessions in hierarchy")
	}

	// Child should inherit root's model
	if childSum.Record.Model != "gemini-pro" {
		t.Errorf("expected child model to inherit 'gemini-pro', got %s", childSum.Record.Model)
	}
	if childSum.Record.Role != "Mid-level Coordinator" {
		t.Errorf("expected child role 'Mid-level Coordinator', got %s", childSum.Record.Role)
	}

	// Grandchild model should be gemini-flash-lite
	if grandSum.Record.Model != "gemini-flash-lite" {
		t.Errorf("expected grandchild model 'gemini-flash-lite', got %s", grandSum.Record.Model)
	}
	if grandSum.Record.Role != "Worker Leaf" {
		t.Errorf("expected grandchild role 'Worker Leaf', got %s", grandSum.Record.Role)
	}

	// Verify recursive rollup across 3 levels:
	// Root own: in=1000, out=100
	// Child own: in=500, out=50
	// Grandchild own: in=200, out=20
	expectedRootIn := int64(1000 + 500 + 200)
	expectedRootOut := int64(100 + 50 + 20)

	if rootSum.TotalInputTokens != expectedRootIn {
		t.Errorf("expected root TotalInputTokens %d, got %d", expectedRootIn, rootSum.TotalInputTokens)
	}
	if rootSum.TotalOutputTokens != expectedRootOut {
		t.Errorf("expected root TotalOutputTokens %d, got %d", expectedRootOut, rootSum.TotalOutputTokens)
	}

	expectedChildIn := int64(500 + 200)
	expectedChildOut := int64(50 + 20)
	if childSum.TotalInputTokens != expectedChildIn {
		t.Errorf("expected child TotalInputTokens %d, got %d", expectedChildIn, childSum.TotalInputTokens)
	}
	if childSum.TotalOutputTokens != expectedChildOut {
		t.Errorf("expected child TotalOutputTokens %d, got %d", expectedChildOut, childSum.TotalOutputTokens)
	}
}

func TestSyncSessionsToStore_NonexistentDirs(t *testing.T) {
	opts := telemetry.ParserOptions{
		BrainDir:  "/nonexistent/brain/dir/12345",
		ClaudeDir: "/nonexistent/claude/dir/12345",
		SpoolDir:  "/nonexistent/spool/dir/12345",
		Since:     time.Hour,
	}

	summaries, err := telemetry.SyncSessionsToStore(context.Background(), nil, opts)
	if err != nil {
		t.Fatalf("expected no error for nonexistent directories, got %v", err)
	}
	if len(summaries) != 0 {
		t.Errorf("expected 0 summaries, got %d", len(summaries))
	}
}

func joinLines(lines []string) string {
	res := ""
	for _, l := range lines {
		res += l + "\n"
	}
	return res
}
