package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rjfonseca/daneel/internal/config"
)

func TestLogCmd_WeeksAndMonthly(t *testing.T) {
	tempVault := t.TempDir()
	dailyDir := filepath.Join(tempVault, "daily")
	_ = os.MkdirAll(dailyDir, 0750)

	// Create daily logs
	files := map[string]string{
		"2026-09-28.md": `---
data: 2026-09-28
tempo-A_Estrategico: 3.0
tempo-B_ImpactoLocal: 2.0
tempo-C_GlueWork: 2.0
tempo-D_LowImpact: 0.5
tempo-E_Overhead: 0.5
---
`,
		"2026-09-21.md": `---
data: 2026-09-21
tempo-A_Estrategico: 4.0
tempo-B_ImpactoLocal: 1.0
tempo-C_GlueWork: 2.0
tempo-D_LowImpact: 0.0
tempo-E_Overhead: 1.0
---
`,
	}
	for name, content := range files {
		_ = os.WriteFile(filepath.Join(dailyDir, name), []byte(content), 0600)
	}

	cfg = &config.Config{
		VaultPath: tempVault,
	}

	// 1. Test --weeks flag execution
	logWeeks = 4
	logMonthly = false
	jsonOut = false
	if err := logCmd.RunE(logCmd, nil); err != nil {
		t.Fatalf("logCmd with --weeks failed: %v", err)
	}

	// 2. Test --monthly flag execution
	logWeeks = 0
	logMonthly = true
	if err := logCmd.RunE(logCmd, nil); err != nil {
		t.Fatalf("logCmd with --monthly failed: %v", err)
	}

	// 3. Test with jsonOut = true
	jsonOut = true
	logWeeks = 2
	logMonthly = false
	if err := logCmd.RunE(logCmd, nil); err != nil {
		t.Fatalf("logCmd with --weeks and --json failed: %v", err)
	}

	// Reset flags
	logWeeks = 0
	logMonthly = false
	jsonOut = false
}
