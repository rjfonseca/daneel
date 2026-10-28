//nolint:revive // package comment not required
package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rjfonseca/daneel/internal/config"
	"github.com/rjfonseca/daneel/internal/routine"
)

func seedTestTelemetryStore(t *testing.T, vaultDir string) {
	t.Helper()
	store, err := routine.OpenDefaultStore(vaultDir)
	if err != nil {
		t.Fatalf("OpenDefaultStore failed: %v", err)
	}
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	now := time.Now().Unix()

	// 1. Routine run
	err = store.RecordRun(ctx, routine.RoutineRun{
		RoutineName:      "dream",
		StartedAt:        now - 300,
		CompletedAt:      now - 200,
		DurationMs:       100000,
		TriggerSource:    "cli",
		ExitCode:         0,
		Status:           "success",
		ItemsProcessed:   120,
		InputTokens:      15000,
		CacheReadTokens:  3000,
		OutputTokens:     1200,
		EstimatedCostUSD: 0.045,
		Model:            "gemini-pro",
		SessionID:        "sess-seed-1",
	})
	if err != nil {
		t.Fatalf("RecordRun failed: %v", err)
	}

	// 2. Session record
	err = store.UpsertSessionTelemetry(ctx, routine.SessionTelemetryRecord{
		SessionID:        "sess-seed-1",
		Agent:            "antigravity",
		Model:            "gemini-pro",
		Role:             "orchestrator",
		StartedAt:        now - 300,
		UpdatedAt:        now - 200,
		TurnsCount:       12,
		SubagentsCount:   2,
		InputTokens:      15000,
		CacheReadTokens:  3000,
		OutputTokens:     1200,
		EstimatedCostUSD: 0.045,
		SkillsUsed:       "vault-dream-cycle,doctor",
	})
	if err != nil {
		t.Fatalf("UpsertSessionTelemetry failed: %v", err)
	}

	// 3. Command events
	err = store.RecordCommandEvent(ctx, routine.CommandTelemetryEvent{
		Surface:    "cli",
		Command:    "search",
		Scope:      "microservices",
		StartedAt:  now - 100,
		DurationMs: 15,
		IsError:    false,
	})
	if err != nil {
		t.Fatalf("RecordCommandEvent failed: %v", err)
	}
	err = store.RecordCommandEvent(ctx, routine.CommandTelemetryEvent{
		Surface:    "mcp",
		Command:    "resolve_note",
		Scope:      "adr-0001",
		StartedAt:  now - 50,
		DurationMs: 3,
		IsError:    false,
	})
	if err != nil {
		t.Fatalf("RecordCommandEvent failed: %v", err)
	}
}

func TestTelemetryCmd_JSONOutput(t *testing.T) {
	tempVault := t.TempDir()
	seedTestTelemetryStore(t, tempVault)

	configYaml := `telemetry:
  finops:
    daily_budget_usd: 2.0
    monthly_budget_usd: 50.0
    default_model: gemini-pro
`
	_ = os.WriteFile(filepath.Join(tempVault, ".daneel.yaml"), []byte(configYaml), 0600)

	cfg = &config.Config{
		VaultPath: tempVault,
		DBPath:    filepath.Join(tempVault, ".daneel", "cache", "vault_index.db"),
		Telemetry: config.TelemetryConfig{
			FinOps: config.FinOpsConfig{
				DailyBudgetUSD:   2.0,
				MonthlyBudgetUSD: 50.0,
				DefaultModel:     "gemini-pro",
			},
		},
	}
	defer func() { cfg = nil; jsonOut = false; RootCmd.SetArgs(nil) }()

	var stdoutBuf bytes.Buffer
	RootCmd.SetOut(&stdoutBuf)
	RootCmd.SetArgs([]string{"--vault-path", tempVault, "telemetry", "--json", "--days=7", "--sync=false"})

	pipeOut, err := captureStdout(t, RootCmd.Execute)
	if err != nil {
		t.Fatalf("telemetry --json failed: %v", err)
	}

	outBytes := []byte(pipeOut)
	if len(outBytes) == 0 {
		outBytes = stdoutBuf.Bytes()
	}

	var report OperationalTelemetryReport
	if err := json.Unmarshal(outBytes, &report); err != nil {
		t.Fatalf("failed to unmarshal JSON output: %v, raw:\n%s", err, string(outBytes))
	}

	if report.WindowDays != 7 {
		t.Errorf("expected WindowDays 7, got %d", report.WindowDays)
	}
	if report.TotalCostUSD <= 0 {
		t.Errorf("expected positive TotalCostUSD, got %f", report.TotalCostUSD)
	}
	if report.TotalInputTokens != 15000 {
		t.Errorf("expected 15000 input tokens, got %d", report.TotalInputTokens)
	}
	if report.DailyBudgetUSD != 2.0 {
		t.Errorf("expected DailyBudgetUSD 2.0, got %f", report.DailyBudgetUSD)
	}
	if len(report.RoutineWeights) == 0 {
		t.Errorf("expected non-empty routine weights")
	}
	if len(report.RecentSessions) == 0 {
		t.Errorf("expected non-empty recent sessions")
	}
	if len(report.CommandMetrics) == 0 {
		t.Errorf("expected non-empty command metrics")
	}
}

func TestTelemetryCmd_TerminalOutput(t *testing.T) {
	tempVault := t.TempDir()
	seedTestTelemetryStore(t, tempVault)

	cfg = &config.Config{
		VaultPath: tempVault,
		DBPath:    filepath.Join(tempVault, ".daneel", "cache", "vault_index.db"),
		Telemetry: config.TelemetryConfig{
			FinOps: config.DefaultFinOpsConfig(),
		},
	}
	defer func() { cfg = nil; jsonOut = false; RootCmd.SetArgs(nil) }()

	RootCmd.SetArgs([]string{"--vault-path", tempVault, "telemetry", "--days=7", "--sync=false"})
	out, err := captureStdout(t, RootCmd.Execute)
	if err != nil {
		t.Fatalf("telemetry failed: %v", err)
	}

	sections := []string{
		"AI FinOps & Budget Summary",
		"Routine Weight & Cost Leaderboard",
		"Recent AI Sessions & Subagents",
		"Vault Primitives & MCP Latency",
	}

	for _, sec := range sections {
		if !strings.Contains(out, sec) {
			t.Errorf("expected terminal output to contain section %q, got:\n%s", sec, out)
		}
	}
}

func TestTelemetryCmd_FilteredSections(t *testing.T) {
	tempVault := t.TempDir()
	seedTestTelemetryStore(t, tempVault)

	cfg = &config.Config{
		VaultPath: tempVault,
		DBPath:    filepath.Join(tempVault, ".daneel", "cache", "vault_index.db"),
		Telemetry: config.TelemetryConfig{
			FinOps: config.DefaultFinOpsConfig(),
		},
	}
	defer func() { cfg = nil; jsonOut = false; telemetryRoutines = false; RootCmd.SetArgs(nil) }()

	// Test --routines filter
	RootCmd.SetArgs([]string{"--vault-path", tempVault, "telemetry", "--routines", "--sync=false"})
	out, err := captureStdout(t, RootCmd.Execute)
	if err != nil {
		t.Fatalf("telemetry --routines failed: %v", err)
	}

	if !strings.Contains(out, "Routine Weight & Cost Leaderboard") {
		t.Errorf("expected output to contain Routine Weight, got:\n%s", out)
	}
	if strings.Contains(out, "Vault Primitives & MCP Latency") {
		t.Errorf("expected output NOT to contain Primitives when filtered to --routines, got:\n%s", out)
	}
}
