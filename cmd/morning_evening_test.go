package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rjfonseca/daneel/internal/config"
	"github.com/rjfonseca/daneel/internal/ops/daily"
)

func TestMorningAndEveningCLI(t *testing.T) {
	tempVault := t.TempDir()
	dailyDir := filepath.Join(tempVault, "daily")
	if err := os.MkdirAll(dailyDir, 0750); err != nil {
		t.Fatalf("failed to create daily dir: %v", err)
	}

	cfg = &config.Config{
		VaultPath: tempVault,
		Vault: config.VaultConfig{
			Owner: "Alan Turing",
		},
	}

	defer func() {
		cfg = nil
		jsonOut = false
		morningDate = ""
		eveningDate = ""
	}()

	// 1. Test 'daneel morning' text output
	morningDate = "2026-11-08"
	jsonOut = false
	out, err := captureStdout(t, func() error {
		return morningCmd.RunE(morningCmd, nil)
	})
	if err != nil {
		t.Fatalf("morningCmd failed: %v", err)
	}
	if !strings.Contains(out, "Morning Briefing") && !strings.Contains(out, "2026-11-08") {
		t.Errorf("expected morning output to contain Morning Briefing / date, got: %s", out)
	}

	// 2. Test 'daneel morning --json'
	morningDate = "2026-11-09"
	jsonOut = true
	jsonOutStr, err := captureStdout(t, func() error {
		return morningCmd.RunE(morningCmd, nil)
	})
	if err != nil {
		t.Fatalf("morningCmd --json failed: %v", err)
	}
	var morningRep daily.MorningReport
	if err := json.Unmarshal([]byte(jsonOutStr), &morningRep); err != nil {
		t.Fatalf("failed to parse morning JSON output: %v (raw: %s)", err, jsonOutStr)
	}
	if morningRep.Date != "2026-11-09" {
		t.Errorf("expected date 2026-11-09 in JSON, got %s", morningRep.Date)
	}

	// 3. Prepare file for evening with explicit annotations
	eveningDaily := filepath.Join(dailyDir, "2026-11-09.md")
	eveningContent := `---
type: daily
date: 2026-11-09
tempo-A_Estrategico: 0
tempo-B_ImpactoLocal: 0
tempo-C_GlueWork: 0
tempo-D_LowImpact: 0
tempo-E_Overhead: 0
tags: [daily]
---
# 📅 Diário de Bordo — 2026-11-09

## ⏱️ Notas de Tempo & Log de Atividades
- [A] 2.5h Architecture design
- [C] 1.0h Team mentorship
`
	if err := os.WriteFile(eveningDaily, []byte(eveningContent), 0600); err != nil {
		t.Fatalf("failed to write evening daily file: %v", err)
	}

	// 4. Test 'daneel evening' text output
	eveningDate = "2026-11-09"
	jsonOut = false
	eveningText, err := captureStdout(t, func() error {
		return eveningCmd.RunE(eveningCmd, nil)
	})
	if err != nil {
		t.Fatalf("eveningCmd failed: %v", err)
	}
	if !strings.Contains(eveningText, "Evening Reconciliation") && !strings.Contains(eveningText, "2026-11-09") {
		t.Errorf("expected evening output to contain Evening Reconciliation / date, got: %s", eveningText)
	}

	// 5. Test 'daneel evening --json'
	jsonOut = true
	eveningJsonStr, err := captureStdout(t, func() error {
		return eveningCmd.RunE(eveningCmd, nil)
	})
	if err != nil {
		t.Fatalf("eveningCmd --json failed: %v", err)
	}
	var eveningRep daily.EveningReport
	if err := json.Unmarshal([]byte(eveningJsonStr), &eveningRep); err != nil {
		t.Fatalf("failed to parse evening JSON output: %v (raw: %s)", err, eveningJsonStr)
	}
	if !eveningRep.Updated {
		t.Errorf("expected evening Updated = true in JSON report")
	}
	if eveningRep.TotalHours != 3.5 {
		t.Errorf("expected evening TotalHours = 3.5 in JSON report, got %v", eveningRep.TotalHours)
	}
}
