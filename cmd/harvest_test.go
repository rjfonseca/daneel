//nolint:revive // package comment not required
package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/rjfonseca/daneel/internal/config"
	"github.com/rjfonseca/daneel/internal/harvester"
)

func TestHarvestPruneCmd_DryRun(t *testing.T) {
	tempVault := t.TempDir()

	// Setup directory structure in sources/gitlab/group-a
	proj1 := filepath.Join(tempVault, "sources", "gitlab", "group-a", "proj-1", "docs")
	_ = os.MkdirAll(proj1, 0750)
	_ = os.WriteFile(filepath.Join(proj1, "arch.md"), []byte("# Architecture"), 0600)

	nodeModules := filepath.Join(tempVault, "sources", "gitlab", "group-a", "proj-1", "node_modules")
	_ = os.MkdirAll(nodeModules, 0750)
	_ = os.WriteFile(filepath.Join(nodeModules, "pkg.md"), []byte("# Toxic"), 0600)

	excludedProj := filepath.Join(tempVault, "sources", "gitlab", "group-a", "excluded-repo", "docs")
	_ = os.MkdirAll(excludedProj, 0750)
	_ = os.WriteFile(filepath.Join(excludedProj, "readme.md"), []byte("# Excluded"), 0600)

	cfg = &config.Config{
		VaultPath: tempVault,
		DBPath:    filepath.Join(tempVault, "test.db"),
		Harvester: config.HarvesterConfig{
			ExcludeProjects: []string{"excluded-*"},
		},
	}

	harvestDryRun = true
	harvestRemote = false
	harvestReindex = false
	jsonOut = false

	err := harvestPruneCmd.RunE(harvestPruneCmd, []string{})
	if err != nil {
		t.Fatalf("harvestPruneCmd failed: %v", err)
	}

	// Dry run: files should still exist
	if _, err := os.Stat(filepath.Join(nodeModules, "pkg.md")); os.IsNotExist(err) {
		t.Errorf("expected node_modules to still exist in dry-run")
	}
	if _, err := os.Stat(filepath.Join(excludedProj, "readme.md")); os.IsNotExist(err) {
		t.Errorf("expected excluded-repo to still exist in dry-run")
	}
}

func TestHarvestPruneCmd_Execute(t *testing.T) {
	tempVault := t.TempDir()

	proj1 := filepath.Join(tempVault, "sources", "gitlab", "group-a", "proj-1", "docs")
	_ = os.MkdirAll(proj1, 0750)
	archFile := filepath.Join(proj1, "arch.md")
	_ = os.WriteFile(archFile, []byte("# Architecture"), 0600)

	nodeModules := filepath.Join(tempVault, "sources", "gitlab", "group-a", "proj-1", "node_modules")
	_ = os.MkdirAll(nodeModules, 0750)
	_ = os.WriteFile(filepath.Join(nodeModules, "pkg.md"), []byte("# Toxic"), 0600)

	excludedProj := filepath.Join(tempVault, "sources", "gitlab", "group-a", "sandbox-repo", "docs")
	_ = os.MkdirAll(excludedProj, 0750)
	_ = os.WriteFile(filepath.Join(excludedProj, "readme.md"), []byte("# Sandbox"), 0600)

	cfg = &config.Config{
		VaultPath: tempVault,
		DBPath:    filepath.Join(tempVault, "test.db"),
		Harvester: config.HarvesterConfig{
			ExcludeProjects: []string{"sandbox-*"},
		},
	}

	harvestDryRun = false
	harvestRemote = false
	harvestReindex = false
	jsonOut = false

	err := harvestPruneCmd.RunE(harvestPruneCmd, []string{})
	if err != nil {
		t.Fatalf("harvestPruneCmd failed: %v", err)
	}

	// Valid project file must still exist
	if _, err := os.Stat(archFile); os.IsNotExist(err) {
		t.Errorf("expected arch.md to be preserved")
	}

	// Toxic node_modules must be removed
	if _, err := os.Stat(nodeModules); !os.IsNotExist(err) {
		t.Errorf("expected node_modules to be pruned")
	}

	// Excluded repo must be removed
	if _, err := os.Stat(filepath.Dir(excludedProj)); !os.IsNotExist(err) {
		t.Errorf("expected sandbox-repo to be pruned")
	}
}

func TestHarvestPruneCmd_JSONOutput(t *testing.T) {
	tempVault := t.TempDir()

	proj1 := filepath.Join(tempVault, "sources", "gitlab", "group-a", "proj-1", "docs")
	_ = os.MkdirAll(proj1, 0750)
	_ = os.WriteFile(filepath.Join(proj1, "arch.md"), []byte("# Architecture"), 0600)

	nodeModules := filepath.Join(tempVault, "sources", "gitlab", "group-a", "proj-1", "node_modules")
	_ = os.MkdirAll(nodeModules, 0750)
	_ = os.WriteFile(filepath.Join(nodeModules, "pkg.md"), []byte("# Toxic"), 0600)

	cfg = &config.Config{
		VaultPath: tempVault,
		DBPath:    filepath.Join(tempVault, "test.db"),
		Harvester: config.HarvesterConfig{},
	}

	harvestDryRun = false
	harvestRemote = false
	harvestReindex = false
	jsonOut = true
	defer func() { jsonOut = false }()

	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := harvestPruneCmd.RunE(harvestPruneCmd, []string{})
	_ = w.Close()
	os.Stdout = origStdout

	if err != nil {
		t.Fatalf("harvestPruneCmd failed: %v", err)
	}

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	var stats harvester.PruneStats
	if err := json.Unmarshal([]byte(output), &stats); err != nil {
		t.Fatalf("failed to unmarshal JSON output %q: %v", output, err)
	}

	if stats.ToxicDirsCount != 1 {
		t.Errorf("expected 1 toxic dir pruned, got %d", stats.ToxicDirsCount)
	}
}
