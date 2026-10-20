//nolint:revive // package comment not required
package workspace

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestWorkspaceScanner_EmptyWorkspaceReport(t *testing.T) {
	for _, exists := range []bool{false, true} {
		t.Run(fmt.Sprintf("directory_exists_%t", exists), func(t *testing.T) {
			root := t.TempDir()
			workDir := filepath.Join(root, "workspace")
			vaultDir := filepath.Join(root, "vault")
			if exists {
				if err := os.Mkdir(workDir, 0750); err != nil {
					t.Fatal(err)
				}
			}
			scanner := NewScanner(workDir, vaultDir)
			report, err := scanner.Scan(false)
			if err != nil {
				t.Fatal(err)
			}
			if report.TotalRepos != 0 || report.Categories == nil || report.DirtyRepos == nil {
				t.Fatalf("expected an initialized empty workspace report: %+v", report)
			}
			wantPath := filepath.Join(vaultDir, "notes", "organizacao", "moc-workspace-work.md")
			if report.MOCFilePath != wantPath {
				t.Errorf("catalog path = %q, want %q", report.MOCFilePath, wantPath)
			}
			if _, err := time.Parse(time.RFC3339, report.ScannedAt); err != nil {
				t.Errorf("invalid scan timestamp: %v", err)
			}
			if !exists {
				if _, err := os.Stat(workDir); !os.IsNotExist(err) {
					t.Fatalf("scan must not create the missing workspace: %v", err)
				}
			}
		})
	}
}

func TestWorkspaceScanner(t *testing.T) {
	tempDir := t.TempDir()

	workDir := filepath.Join(tempDir, "Work")
	vaultDir := filepath.Join(tempDir, "Vault")
	notesDir := filepath.Join(vaultDir, "notes", "sistemas")

	_ = os.MkdirAll(workDir, 0750)
	_ = os.MkdirAll(notesDir, 0750)

	// Create test git repo 1: Go repo
	repo1 := filepath.Join(workDir, "acme-api")
	_ = os.MkdirAll(repo1, 0750)
	_ = exec.Command("git", "-C", repo1, "init").Run() //nolint:gosec // intentional
	_ = os.WriteFile(filepath.Join(repo1, "go.mod"), []byte("module github.com/test/acme-api\n\ngo 1.26\n"), 0600)
	_ = os.WriteFile(filepath.Join(repo1, "README.md"), []byte("# Acme Corp API\n\nMain public API service for the platform.\n"), 0600)

	// Create vault note
	note1 := filepath.Join(notesDir, "acme-api.md")
	_ = os.WriteFile(note1, []byte("# Acme Corp API\n\nDocumentação técnica."), 0600)

	scanner := NewScanner(workDir, vaultDir)
	rep, err := scanner.Scan(false)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if rep.TotalRepos != 1 {
		t.Errorf("expected 1 repo, got %d", rep.TotalRepos)
	}

	if len(rep.DirtyRepos) != 1 { // Untracked files in newly initialized repo
		t.Errorf("expected 1 dirty repo, got %d", len(rep.DirtyRepos))
	}

	r := rep.DirtyRepos[0]
	if r.Name != "acme-api" {
		t.Errorf("expected name 'acme-api', got '%s'", r.Name)
	}
	if r.Stack != "Go" {
		t.Errorf("expected stack 'Go', got '%s'", r.Stack)
	}
	if r.ComponentType != "🌐 API / Gateway" {
		t.Errorf("expected component type API, got '%s'", r.ComponentType)
	}
	if r.VaultLink != "notes/sistemas/acme-api.md" {
		t.Errorf("expected vault link 'notes/sistemas/acme-api.md', got '%s'", r.VaultLink)
	}

	moc := scanner.GenerateMOCContent(rep)
	if len(moc) < 50 {
		t.Errorf("expected MOC content, got empty string")
	}
}

func TestBranchInactivityAndPruning(t *testing.T) {
	tempDir := t.TempDir()
	workDir := filepath.Join(tempDir, "Work")
	vaultDir := filepath.Join(tempDir, "Vault")
	_ = os.MkdirAll(workDir, 0750)
	_ = os.MkdirAll(vaultDir, 0750)

	initRepo := func(name string) string {
		repoPath := filepath.Join(workDir, name)
		_ = os.MkdirAll(repoPath, 0750)
		_ = exec.Command("git", "-C", repoPath, "init", "-b", "main").Run()
		_ = exec.Command("git", "-C", repoPath, "config", "user.name", "Tester").Run()
		_ = exec.Command("git", "-C", repoPath, "config", "user.email", "test@test.com").Run()
		_ = exec.Command("git", "-C", repoPath, "config", "commit.gpgsign", "false").Run()

		f1 := filepath.Join(repoPath, "file.txt")
		_ = os.WriteFile(f1, []byte("init"), 0600)
		_ = exec.Command("git", "-C", repoPath, "add", ".").Run()
		_ = exec.Command("git", "-C", repoPath, "commit", "-m", "init commit").Run()
		return repoPath
	}

	// Repo 1: Active branch with recent commit (< 48h)
	// Must NOT be reset to main
	repoActive := initRepo("active-repo")
	_ = exec.Command("git", "-C", repoActive, "checkout", "-b", "feat/active").Run()
	_ = os.WriteFile(filepath.Join(repoActive, "feature.txt"), []byte("active work"), 0600)
	_ = exec.Command("git", "-C", repoActive, "add", ".").Run()
	_ = exec.Command("git", "-C", repoActive, "commit", "-m", "recent active commit").Run()

	// Repo 2: Stale branch with old commit (> 48h)
	// Must BE reset to main
	repoStale := initRepo("stale-repo")
	_ = exec.Command("git", "-C", repoStale, "checkout", "-b", "feat/stale").Run()
	_ = os.WriteFile(filepath.Join(repoStale, "stale.txt"), []byte("stale work"), 0600)
	_ = exec.Command("git", "-C", repoStale, "add", ".").Run()
	cmdStale := exec.Command("git", "-C", repoStale, "commit", "-m", "old commit")
	cmdStale.Env = append(os.Environ(), "GIT_COMMITTER_DATE=1600000000", "GIT_AUTHOR_DATE=1600000000")
	_ = cmdStale.Run()

	// Repo 3: Repo on main with one merged branch and one unmerged branch
	repoPrune := initRepo("prune-repo")
	// Merged branch
	_ = exec.Command("git", "-C", repoPrune, "checkout", "-b", "feat/merged-pr").Run()
	_ = os.WriteFile(filepath.Join(repoPrune, "merged.txt"), []byte("merged work"), 0600)
	_ = exec.Command("git", "-C", repoPrune, "add", ".").Run()
	_ = exec.Command("git", "-C", repoPrune, "commit", "-m", "merged pr commit").Run()
	_ = exec.Command("git", "-C", repoPrune, "checkout", "main").Run()
	_ = exec.Command("git", "-C", repoPrune, "merge", "feat/merged-pr").Run()
	// Unmerged branch
	_ = exec.Command("git", "-C", repoPrune, "checkout", "-b", "feat/unmerged-pr").Run()
	_ = os.WriteFile(filepath.Join(repoPrune, "unmerged.txt"), []byte("wip work"), 0600)
	_ = exec.Command("git", "-C", repoPrune, "add", ".").Run()
	_ = exec.Command("git", "-C", repoPrune, "commit", "-m", "unmerged commit").Run()
	_ = exec.Command("git", "-C", repoPrune, "checkout", "main").Run()

	// Run scanner with syncClean = true and default 48h threshold
	scanner := NewScanner(workDir, vaultDir)
	rep, err := scanner.Scan(true)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// 1. Check active repo: branch should STILL be feat/active
	outBranchActive, _ := exec.Command("git", "-C", repoActive, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if got := string(outBranchActive); got != "feat/active\n" {
		t.Errorf("active-repo should remain on 'feat/active', got %q", got)
	}

	// 2. Check stale repo: branch should have been reset to main
	outBranchStale, _ := exec.Command("git", "-C", repoStale, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if got := string(outBranchStale); got != "main\n" {
		t.Errorf("stale-repo should have reset to 'main', got %q", got)
	}

	// 3. Check prune repo: feat/merged-pr should be deleted, feat/unmerged-pr should be preserved
	errMerged := exec.Command("git", "-C", repoPrune, "rev-parse", "--verify", "feat/merged-pr").Run()
	if errMerged == nil {
		t.Errorf("expected feat/merged-pr to be deleted, but it still exists")
	}

	errUnmerged := exec.Command("git", "-C", repoPrune, "rev-parse", "--verify", "feat/unmerged-pr").Run()
	if errUnmerged != nil {
		t.Errorf("expected feat/unmerged-pr to be preserved, but it was deleted")
	}

	if rep.PrunedBranchesCount < 1 {
		t.Errorf("expected PrunedBranchesCount >= 1, got %d", rep.PrunedBranchesCount)
	}

	// 4. Test custom threshold: If threshold is very long (e.g. 10000h), even stale repo should NOT be reset
	repoStale2 := initRepo("stale-repo-2")
	_ = exec.Command("git", "-C", repoStale2, "checkout", "-b", "feat/stale-protected").Run()
	_ = os.WriteFile(filepath.Join(repoStale2, "stale.txt"), []byte("stale work"), 0600)
	_ = exec.Command("git", "-C", repoStale2, "add", ".").Run()
	cmdStale2 := exec.Command("git", "-C", repoStale2, "commit", "-m", "old commit")
	cmdStale2.Env = append(os.Environ(), "GIT_COMMITTER_DATE=1600000000", "GIT_AUTHOR_DATE=1600000000")
	_ = cmdStale2.Run()

	scannerLongThreshold := NewScanner(workDir, vaultDir)
	scannerLongThreshold.SetInactivityThreshold(1000000 * time.Hour)
	_, _ = scannerLongThreshold.Scan(true)

	outBranchProtected, _ := exec.Command("git", "-C", repoStale2, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if got := string(outBranchProtected); got != "feat/stale-protected\n" {
		t.Errorf("expected repo with long threshold to remain on 'feat/stale-protected', got %q", got)
	}
}
