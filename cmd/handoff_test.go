package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rjfonseca/daneel/internal/config"
	"github.com/rjfonseca/daneel/internal/dream"
	"github.com/rjfonseca/daneel/internal/ops/daily"
)

func TestHandoffRecordAndSummaryCLI(t *testing.T) {
	tempVault := t.TempDir()
	cfg = &config.Config{
		VaultPath: tempVault,
		Vault: config.VaultConfig{
			Owner: "Ada Lovelace",
		},
	}
	defer func() {
		cfg = nil
		jsonOut = false
		handoffKind = "agent_action"
		handoffSummary = ""
		handoffActions = nil
		handoffInbox = nil
		handoffAlerts = nil
		handoffResolvedAlerts = nil
		handoffAppendEvo = false
		handoffSince = ""
		handoffDate = ""
	}()

	// 1. Record an evening closure with --append-evolution
	handoffKind = string(dream.HandoffKindEveningClosure)
	handoffSummary = "Completed end-of-day telemetry and promoted architecture note"
	handoffActions = []string{"Created notes/arch/boundaries.md", "Reconciled 6.0h"}
	handoffResolvedAlerts = []string{"Pending refactoring"}
	handoffAppendEvo = true
	jsonOut = true

	recOut, err := captureStdout(t, func() error {
		return handoffRecordCmd.RunE(handoffRecordCmd, nil)
	})
	if err != nil {
		t.Fatalf("handoff record failed: %v", err)
	}
	if !strings.Contains(recOut, `"kind": "evening_closure"`) {
		t.Errorf("expected evening_closure in record JSON output, got: %s", recOut)
	}

	// Verify meta/guias/agente-evolucao.md was appended
	evoBytes, err := os.ReadFile(filepath.Join(tempVault, "meta", "guias", "agente-evolucao.md"))
	if err != nil {
		t.Fatalf("expected agente-evolucao.md to be created: %v", err)
	}
	if !strings.Contains(string(evoBytes), "Completed end-of-day telemetry and promoted architecture note") {
		t.Errorf("expected summary in evolution log, got:\n%s", string(evoBytes))
	}

	// 2. Append an automated dream cycle event afterwards (simulating next-morning REM tick)
	if err := dream.AppendHandoffEvent(tempVault, dream.HandoffEvent{
		Kind:         dream.HandoffKindDreamCycle,
		Date:         "2026-11-25",
		FilesScanned: 42,
		FixedLinks:   1,
	}); err != nil {
		t.Fatalf("append dream cycle failed: %v", err)
	}

	// 3. Summary must include BOTH evening closure and the subsequent REM cycle without creating daily/
	handoffDate = "2026-11-25"
	jsonOut = true
	sumOut, err := captureStdout(t, func() error {
		return handoffSummaryCmd.RunE(handoffSummaryCmd, nil)
	})
	if err != nil {
		t.Fatalf("handoff summary failed: %v", err)
	}

	var rep daily.MorningReport
	if err := json.Unmarshal([]byte(sumOut), &rep); err != nil {
		t.Fatalf("unmarshal handoff summary JSON failed: %v (raw: %s)", err, sumOut)
	}
	if len(rep.EveningSummaries) != 1 || !strings.Contains(rep.EveningSummaries[0], "Completed end-of-day telemetry") {
		t.Errorf("expected evening summary preserved in cumulative summary, got %v", rep.EveningSummaries)
	}
	if rep.DreamCyclesCount != 1 {
		t.Errorf("expected DreamCyclesCount=1, got %d", rep.DreamCyclesCount)
	}
	if _, err := os.Stat(filepath.Join(tempVault, "daily", "2026-11-25.md")); !os.IsNotExist(err) {
		t.Errorf("handoff summary must be read-only and not create daily/2026-11-25.md")
	}
}
