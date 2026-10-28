package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rjfonseca/daneel/internal/config"
	"github.com/rjfonseca/daneel/internal/routine"
)

func TestRoutineCmd_ListAndStatus(t *testing.T) {
	t.Setenv("DANEEL_VAULT_PATH", "")
	t.Setenv("DANEEL_CONFIG", "")
	tempDir := t.TempDir()
	cfg = &config.Config{
		VaultPath: tempDir,
		DBPath:    filepath.Join(tempDir, "test.db"),
		Routines:  config.DefaultRoutinesConfig(),
	}
	defer func() { cfg = nil; RootCmd.SetArgs(nil) }()

	// 1. Test routine list (table output)
	var buf bytes.Buffer
	RootCmd.SetOut(&buf)
	RootCmd.SetArgs([]string{"routine", "list"})

	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := RootCmd.Execute()
	_ = w.Close()
	os.Stdout = origStdout

	if err != nil {
		t.Fatalf("routine list failed: %v", err)
	}

	var outBuf bytes.Buffer
	_, _ = outBuf.ReadFrom(r)
	output := outBuf.String()

	if len(output) == 0 {
		t.Errorf("expected output from routine list, got empty")
	}

	// 2. Test routine list --json
	r, w, _ = os.Pipe()
	os.Stdout = w
	RootCmd.SetArgs([]string{"routine", "list", "--json"})
	jsonOut = true
	defer func() { jsonOut = false }()

	err = RootCmd.Execute()
	_ = w.Close()
	os.Stdout = origStdout

	if err != nil {
		t.Fatalf("routine list --json failed: %v", err)
	}

	var jsonBuf bytes.Buffer
	_, _ = jsonBuf.ReadFrom(r)

	var statuses []routine.RoutineStatus
	if err := json.Unmarshal(jsonBuf.Bytes(), &statuses); err != nil {
		t.Fatalf("failed to unmarshal JSON output: %v, raw: %s", err, jsonBuf.String())
	}

	if len(statuses) != 3 {
		t.Errorf("expected 3 routine statuses, got %d", len(statuses))
	}
}

func TestRoutineCmd_ExecAndRecordLedger(t *testing.T) {
	t.Setenv("DANEEL_VAULT_PATH", "")
	t.Setenv("DANEEL_CONFIG", "")
	tempDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tempDir, ".daneel.yaml"), []byte("vault: "+tempDir+"\n"), 0600)
	cfg = &config.Config{
		VaultPath: tempDir,
		DBPath:    filepath.Join(tempDir, "test.db"),
		Routines:  config.DefaultRoutinesConfig(),
	}
	defer func() { vaultPath = ""; cfg = nil; RootCmd.SetArgs(nil) }()

	// Mock routineExecutor
	origExecutor := routineExecutor
	defer func() { routineExecutor = origExecutor }()

	executedCmd := ""
	executedArgs := []string{}
	routineExecutor = func(ctx context.Context, cmdName string, cmdArgs []string, stdout, stderr io.Writer) (int, error) {
		executedCmd = cmdName
		executedArgs = cmdArgs
		_, _ = stdout.Write([]byte("mock execution output"))
		return 0, nil
	}

	RootCmd.SetArgs([]string{"--vault-path", tempDir, "routine", "exec", "workspace", "--trigger=systemd"})
	err := RootCmd.Execute()
	if err != nil {
		t.Fatalf("routine exec failed: %v", err)
	}

	if executedCmd != "workspace" {
		t.Errorf("expected executedCmd to be 'workspace', got %q", executedCmd)
	}
	if len(executedArgs) != 0 {
		t.Errorf("expected empty executedArgs, got %v", executedArgs)
	}

	// Verify ledger in SQLite store
	store, err := routine.OpenDefaultStore(tempDir)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	latest, err := store.GetLatestRun(context.Background(), "workspace")
	if err != nil {
		t.Fatalf("failed to get latest run: %v", err)
	}
	if latest == nil {
		t.Fatalf("expected recorded run for workspace, got nil")
	}
	if latest.TriggerSource != "systemd" {
		t.Errorf("expected trigger_source 'systemd', got %q", latest.TriggerSource)
	}
	if latest.Status != "success" || latest.ExitCode != 0 {
		t.Errorf("expected success status and exit code 0, got status=%s, code=%d", latest.Status, latest.ExitCode)
	}
}

func TestRoutineCmd_Exec_ExtractsItemsAndCorrelatesFinOps(t *testing.T) {
	t.Setenv("DANEEL_VAULT_PATH", "")
	t.Setenv("DANEEL_CONFIG", "")
	tempDir := t.TempDir()
	tempBrainDir := filepath.Join(tempDir, "brain")

	sessID := "routine-sess-123"
	sessLogsDir := filepath.Join(tempBrainDir, sessID)
	_ = os.MkdirAll(sessLogsDir, 0750)

	t0 := time.Now()
	transcript := fmt.Sprintf(`{"step_index":0,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":%q,"input_tokens":5000,"output_tokens":1000,"tool_calls":[]}`+"\n", t0.Format(time.RFC3339))
	_ = os.WriteFile(filepath.Join(sessLogsDir, "transcript.jsonl"), []byte(transcript), 0600)

	cfg = &config.Config{
		VaultPath: tempDir,
		DBPath:    filepath.Join(tempDir, "test.db"),
		Routines:  config.DefaultRoutinesConfig(),
		Telemetry: config.TelemetryConfig{
			FinOps: config.DefaultFinOpsConfig(),
		},
	}
	defer func() { vaultPath = ""; cfg = nil; RootCmd.SetArgs(nil) }()

	origExecutor := routineExecutor
	defer func() { routineExecutor = origExecutor }()

	routineExecutor = func(ctx context.Context, cmdName string, cmdArgs []string, stdout, stderr io.Writer) (int, error) {
		_, _ = stdout.Write([]byte("Hygiene complete: scanned 42 files and 15 notes\n"))
		return 0, nil
	}

	RootCmd.SetArgs([]string{"--vault-path", tempDir, "routine", "exec", "workspace"})
	err := RootCmd.Execute()
	if err != nil {
		t.Fatalf("routine exec failed: %v", err)
	}

	store, err := routine.OpenDefaultStore(tempDir)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	latest, err := store.GetLatestRun(context.Background(), "workspace")
	if err != nil {
		t.Fatalf("failed to get latest run: %v", err)
	}
	if latest == nil {
		t.Fatalf("expected recorded run for workspace, got nil")
	}

	if latest.ItemsProcessed != 42 {
		t.Errorf("expected ItemsProcessed 42, got %d", latest.ItemsProcessed)
	}
	if latest.InputTokens != 5000 {
		t.Errorf("expected InputTokens 5000, got %d", latest.InputTokens)
	}
	if latest.OutputTokens != 1000 {
		t.Errorf("expected OutputTokens 1000, got %d", latest.OutputTokens)
	}
}
