//nolint:revive // package comment not required
package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/rjfonseca/daneel/internal/config"
)

func TestWorkspaceCmd(t *testing.T) {
	t.Setenv("DANEEL_VAULT_PATH", "")
	t.Setenv("DANEEL_CONFIG", "")
	tempVault := t.TempDir()
	tempWork := filepath.Join(tempVault, "Work")
	_ = os.MkdirAll(tempWork, 0750)

	// Create test git repo
	repo := filepath.Join(tempWork, "my-repo")
	_ = os.MkdirAll(repo, 0750)
	_ = exec.Command("git", "-C", repo, "init", "-b", "main").Run()
	_ = exec.Command("git", "-C", repo, "config", "user.name", "Tester").Run()
	_ = exec.Command("git", "-C", repo, "config", "user.email", "test@test.com").Run()
	_ = exec.Command("git", "-C", repo, "config", "commit.gpgsign", "false").Run()
	_ = os.WriteFile(filepath.Join(repo, "README.md"), []byte("# My Repo"), 0600)
	_ = exec.Command("git", "-C", repo, "add", ".").Run()
	_ = exec.Command("git", "-C", repo, "commit", "-m", "init").Run()

	cfg = &config.Config{
		VaultPath: tempVault,
		DBPath:    filepath.Join(tempVault, ".daneel", "cache", "vault_index.db"),
		Workspace: config.WorkspaceConfig{
			WorkDir:          tempWork,
			BranchInactivity: "24h",
		},
	}

	RootCmd.SetArgs([]string{"workspace", "--root=" + tempWork, "--inactivity-threshold=48h", "--write=false"})
	if err := RootCmd.Execute(); err != nil {
		t.Fatalf("workspace command failed: %v", err)
	}
}
