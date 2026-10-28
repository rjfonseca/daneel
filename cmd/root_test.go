//nolint:revive // package comment not required
package cmd

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/rjfonseca/daneel/internal/config"
	"github.com/rjfonseca/daneel/internal/routine"
)

func TestRootCmd_CLICommandTelemetry(t *testing.T) {
	tempVault := t.TempDir()
	_ = os.MkdirAll(filepath.Join(tempVault, "notes"), 0750)

	cfg = &config.Config{
		VaultPath: tempVault,
	}
	cmdStartTime = time.Now().Add(-50 * time.Millisecond)

	dummyCmd := &cobra.Command{
		Use: "search",
	}

	if RootCmd.PersistentPostRun == nil {
		t.Fatalf("expected RootCmd.PersistentPostRun to be defined")
	}

	RootCmd.PersistentPostRun(dummyCmd, []string{"microservices", "architecture"})

	store, err := routine.OpenDefaultStore(tempVault)
	if err != nil {
		t.Fatalf("OpenDefaultStore failed: %v", err)
	}
	defer func() { _ = store.Close() }()

	aggs, err := store.GetCommandAggregates(context.Background(), 0, 50)
	if err != nil {
		t.Fatalf("GetCommandAggregates failed: %v", err)
	}

	found := false
	for _, a := range aggs {
		if a.Surface == "cli" && a.Command == "search" {
			found = true
			if a.TotalCalls != 1 {
				t.Errorf("expected 1 call, got %d", a.TotalCalls)
			}
			break
		}
	}

	if !found {
		t.Fatalf("expected CLI command telemetry for search, got: %+v", aggs)
	}

	// Verify skipped commands are not recorded
	hookCmd := &cobra.Command{Use: "hook"}
	RootCmd.PersistentPostRun(hookCmd, []string{"session", "start"})

	aggsAfter, _ := store.GetCommandAggregates(context.Background(), 0, 50)
	for _, a := range aggsAfter {
		if a.Command == "hook" {
			t.Fatalf("hook command should be skipped from telemetry recording")
		}
	}
}
