//nolint:revive,misspell // package comment not required
package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rjfonseca/daneel/internal/config"
	"github.com/rjfonseca/daneel/internal/dream"
	"github.com/rjfonseca/daneel/internal/routine"
)

func TestDreamCmd_NoAgentFlag(t *testing.T) {
	tempVault := t.TempDir()

	_ = os.MkdirAll(filepath.Join(tempVault, "notes", "ia"), 0750)
	_ = os.MkdirAll(filepath.Join(tempVault, "daily"), 0750)

	dailyNote := filepath.Join(tempVault, "daily", "2026-08-19.md")
	_ = os.WriteFile(dailyNote, []byte("---\ndata: 2026-08-19\ntempo-A_Estrategico: 2\n---\n# Daily\n"), 0600)

	cfg = &config.Config{
		VaultPath: tempVault,
		DBPath:    filepath.Join(tempVault, ".daneel", "cache", "vault_index.db"),
		Vault:     config.VaultConfig{IndexedDirs: []string{"notes", "daily"}},
	}

	dreamDate = "2026-08-19"
	dreamNoAgent = true
	dreamLearn = false

	err := dreamCmd.RunE(dreamCmd, []string{})
	if err != nil {
		t.Fatalf("dreamCmd with --no-agent failed: %v", err)
	}
}

func TestDreamCmd_StatsAndSynthesize(t *testing.T) {
	tempVault := t.TempDir()

	_ = os.MkdirAll(filepath.Join(tempVault, "notes", "ia"), 0750)
	_ = os.MkdirAll(filepath.Join(tempVault, "daily"), 0750)

	dailyNote := filepath.Join(tempVault, "daily", "2026-08-20.md")
	_ = os.WriteFile(dailyNote, []byte("---\ndata: 2026-08-20\ntempo-A_Estrategico: 3\n---\n# Daily\n[[Emerging System]]\n"), 0600)

	cfg = &config.Config{
		VaultPath: tempVault,
		DBPath:    filepath.Join(tempVault, ".daneel", "cache", "vault_index.db"),
		Vault:     config.VaultConfig{IndexedDirs: []string{"notes", "daily"}},
	}

	dreamDate = "2026-08-20"
	dreamNoAgent = true
	dreamStats = true
	dreamSynthesize = true
	jsonOut = false

	err := dreamCmd.RunE(dreamCmd, []string{})
	if err != nil {
		t.Fatalf("dreamCmd with --stats --synthesize failed: %v", err)
	}

	// Verify evolution log was written
	evoPath := filepath.Join(tempVault, "meta", "guias", "agente-evolucao.md")
	if _, err := os.Stat(evoPath); os.IsNotExist(err) {
		t.Errorf("expected evolution log at %s, but file does not exist", evoPath)
	}
}

func TestDreamCmd_JSONOutput(t *testing.T) {
	tempVault := t.TempDir()

	_ = os.MkdirAll(filepath.Join(tempVault, "notes"), 0750)
	_ = os.MkdirAll(filepath.Join(tempVault, "daily"), 0750)

	dailyNote := filepath.Join(tempVault, "daily", "2026-08-21.md")
	_ = os.WriteFile(dailyNote, []byte("---\ndata: 2026-08-21\n---\n# Daily\n"), 0600)

	cfg = &config.Config{
		VaultPath: tempVault,
		DBPath:    filepath.Join(tempVault, ".daneel", "cache", "vault_index.db"),
		Vault:     config.VaultConfig{IndexedDirs: []string{"notes", "daily"}},
	}

	dreamDate = "2026-08-21"
	dreamNoAgent = true
	dreamStats = false
	dreamSynthesize = false
	jsonOut = true

	err := dreamCmd.RunE(dreamCmd, []string{})
	if err != nil {
		t.Fatalf("dreamCmd with --json failed: %v", err)
	}
	jsonOut = false
}

func TestBuildDreamPrompt_WithRecentSessions(t *testing.T) {
	report := &dream.FullDreamReport{
		RecentSessions: []dream.SessionLogTriage{
			{
				File:            "2026-10-05-14-00-claude.md",
				Agent:           "claude",
				Directory:       "/home/user/work/project",
				DurationMinutes: 25,
				EventsCount:     14,
			},
		},
	}

	prompt := buildDreamPrompt(report, "2026-10-05", "")
	if !strings.Contains(prompt, "Triaged Agent Sessions (inbox/sessions/)") {
		t.Errorf("expected prompt to contain agent sessions section, got: %s", prompt)
	}
	if !strings.Contains(prompt, "2026-10-05-14-00-claude.md") {
		t.Errorf("expected prompt to mention session file, got: %s", prompt)
	}
}

func TestDreamCmd_SessionTriageIntegration(t *testing.T) {
	tempVault := t.TempDir()

	_ = os.MkdirAll(filepath.Join(tempVault, "notes"), 0750)
	_ = os.MkdirAll(filepath.Join(tempVault, "daily"), 0750)
	sessionsDir := filepath.Join(tempVault, "inbox", "sessions")
	_ = os.MkdirAll(sessionsDir, 0750)

	sessionFile := filepath.Join(sessionsDir, "2026-10-05-10-00-antigravity.md")
	sessionContent := `---
type: session_log
agent: "antigravity"
directory: "/work/app"
duration_minutes: 15
events_count: 5
session_id: "test-sess-1"
date: "2026-10-05"
---

<untrusted_content>
## Command & Tool Summary
- Executed ` + "`go test ./...`" + `
</untrusted_content>
`
	_ = os.WriteFile(sessionFile, []byte(sessionContent), 0600)

	dailyNote := filepath.Join(tempVault, "daily", "2026-10-05.md")
	_ = os.WriteFile(dailyNote, []byte("---\ndata: 2026-10-05\n---\n# Daily\n"), 0600)

	cfg = &config.Config{
		VaultPath: tempVault,
		DBPath:    filepath.Join(tempVault, ".daneel", "cache", "vault_index.db"),
		Vault:     config.VaultConfig{IndexedDirs: []string{"notes", "daily"}},
	}

	dreamDate = "2026-10-05"
	dreamNoAgent = true
	dreamStats = false
	dreamSynthesize = false
	jsonOut = false

	err := dreamCmd.RunE(dreamCmd, []string{})
	if err != nil {
		t.Fatalf("dreamCmd with session log failed: %v", err)
	}
}

func TestDreamCmd_FinOpsIntegration(t *testing.T) {
	tempVault := t.TempDir()

	_ = os.MkdirAll(filepath.Join(tempVault, "notes"), 0750)
	_ = os.MkdirAll(filepath.Join(tempVault, "daily"), 0750)
	dailyNote := filepath.Join(tempVault, "daily", "2026-10-28.md")
	_ = os.WriteFile(dailyNote, []byte("---\ndata: 2026-10-28\n---\n# Daily\n"), 0600)

	cfg = &config.Config{
		VaultPath: tempVault,
		DBPath:    filepath.Join(tempVault, ".daneel", "cache", "vault_index.db"),
		Vault:     config.VaultConfig{IndexedDirs: []string{"notes", "daily"}},
		Telemetry: config.TelemetryConfig{
			FinOps: config.DefaultFinOpsConfig(),
		},
	}
	dreamDate = "2026-10-28"
	dreamNoAgent = true
	dreamStats = true
	dreamSynthesize = false
	jsonOut = false

	err := dreamCmd.RunE(dreamCmd, []string{})
	if err != nil {
		t.Fatalf("dreamCmd failed: %v", err)
	}

	store, err := routine.OpenDefaultStore(tempVault)
	if err != nil {
		t.Fatalf("OpenDefaultStore failed: %v", err)
	}
	defer func() { _ = store.Close() }()

	run, err := store.GetLatestRun(context.Background(), "dream")
	if err != nil {
		t.Fatalf("GetLatestRun failed: %v", err)
	}
	if run == nil {
		t.Fatalf("expected recorded run for routine 'dream', got nil")
	}

	dashPath := filepath.Join(tempVault, "meta", "dashboard-metricas.md")
	dashData, err := os.ReadFile(dashPath)
	if err != nil {
		t.Fatalf("failed to read dashboard: %v", err)
	}
	if !strings.Contains(string(dashData), "## 💰 5. Telemetria Operacional, Custos de IA (FinOps) & Uso do Vault") { //nolint:misspell
		t.Errorf("expected dashboard to contain Section 5 FinOps, got:\n%s", string(dashData))
	}
}
