//nolint:revive // package comment not required
package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rjfonseca/daneel/internal/config"
)

func TestDoctorCmd_IncrementalAndDeletedTarget(t *testing.T) {
	// 1. Verify --force flag exists on doctorCmd and tidyCmd
	if doctorCmd.Flags().Lookup("force") == nil {
		t.Fatalf("expected doctorCmd to have --force flag")
	}
	if tidyCmd.Flags().Lookup("force") == nil {
		t.Fatalf("expected tidyCmd to have --force flag")
	}

	t.Setenv("DANEEL_VAULT_PATH", "")
	t.Setenv("DANEEL_CONFIG", "")
	tempVault := t.TempDir()

	notesArchDir := filepath.Join(tempVault, "notes", "arch")
	if err := os.MkdirAll(notesArchDir, 0750); err != nil {
		t.Fatalf("failed to create notes/arch: %v", err)
	}

	serviceDoc := filepath.Join(notesArchDir, "service.md")
	depDoc := filepath.Join(notesArchDir, "dep.md")

	serviceContent := `---
type: architecture
description: Service doc
---
# Service

Link: [Dep](dep.md)
`
	depContent := `---
type: architecture
description: Dep doc
---
# Dep
`

	if err := os.WriteFile(serviceDoc, []byte(serviceContent), 0600); err != nil {
		t.Fatalf("failed to write service doc: %v", err)
	}
	if err := os.WriteFile(depDoc, []byte(depContent), 0600); err != nil {
		t.Fatalf("failed to write dep doc: %v", err)
	}

	cfg = &config.Config{
		VaultPath: tempVault,
		DBPath:    filepath.Join(tempVault, ".daneel", "cache", "vault_index.db"),
		Vault:     config.VaultConfig{IndexedDirs: []string{"notes"}},
	}

	origJsonOut := jsonOut
	origDoctorForce := doctorForce
	jsonOut = true
	doctorForce = false
	defer func() {
		jsonOut = origJsonOut
		doctorForce = origDoctorForce
		cfg = nil
	}()

	// Warm-up run 1: both files exist on disk
	out1, err := captureStdout(t, func() error {
		return doctorCmd.RunE(doctorCmd, nil)
	})
	if err != nil {
		t.Fatalf("warm-up doctorCmd failed: %v", err)
	}

	var rep1 LintReport
	if err := json.Unmarshal([]byte(out1), &rep1); err != nil {
		t.Fatalf("failed to unmarshal run 1 report: %v\nOutput: %s", err, out1)
	}
	if rep1.IssuesCount != 0 {
		t.Fatalf("expected 0 issues in warm-up run, got %d: %+v", rep1.IssuesCount, rep1.Issues)
	}

	// Delete notes/arch/dep.md without modifying service.md
	if err := os.Remove(depDoc); err != nil {
		t.Fatalf("failed to remove dep.md: %v", err)
	}

	// Incremental run 2: service.md is cached in file_watermarks
	out2, err := captureStdout(t, func() error {
		return doctorCmd.RunE(doctorCmd, nil)
	})
	if err != nil {
		t.Fatalf("incremental doctorCmd failed: %v", err)
	}

	var rep2 LintReport
	if err := json.Unmarshal([]byte(out2), &rep2); err != nil {
		t.Fatalf("failed to unmarshal run 2 report: %v\nOutput: %s", err, out2)
	}
	if rep2.IssuesCount != 1 {
		t.Fatalf("expected 1 issue in incremental run, got %d: %+v", rep2.IssuesCount, rep2.Issues)
	}
	if !strings.Contains(rep2.Issues[0].Message, "dep.md") {
		t.Fatalf("expected issue to mention dep.md, got: %s", rep2.Issues[0].Message)
	}
}
