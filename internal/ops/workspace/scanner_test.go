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

func TestStaleDirtyRepoAndWorktreePruning(t *testing.T) {
	tempDir := t.TempDir()
	workDir := filepath.Join(tempDir, "Work")
	vaultDir := filepath.Join(tempDir, "Vault")
	_ = os.MkdirAll(workDir, 0750)
	_ = os.MkdirAll(vaultDir, 0750)

	remoteDir := filepath.Join(tempDir, "remote.git")
	_ = exec.Command("git", "init", "--bare", "-b", "main", remoteDir).Run()

	initRepo := func(name string) string {
		repoPath := filepath.Join(workDir, name)
		_ = os.MkdirAll(repoPath, 0750)
		_ = exec.Command("git", "-C", repoPath, "init", "-b", "main").Run()
		_ = exec.Command("git", "-C", repoPath, "config", "user.name", "Ada Lovelace").Run()
		_ = exec.Command("git", "-C", repoPath, "config", "user.email", "ada@example.com").Run()
		_ = exec.Command("git", "-C", repoPath, "config", "commit.gpgsign", "false").Run()

		f1 := filepath.Join(repoPath, "file.txt")
		_ = os.WriteFile(f1, []byte("init"), 0600)
		_ = exec.Command("git", "-C", repoPath, "add", ".").Run()
		_ = exec.Command("git", "-C", repoPath, "commit", "-m", "init commit").Run()
		return repoPath
	}

	// 1. Stale dirty repo WITHOUT conflict -> should switch to main!
	repoStaleDirty := initRepo("stale-dirty-ok")
	_ = exec.Command("git", "-C", repoStaleDirty, "remote", "add", "origin", remoteDir).Run()
	_ = exec.Command("git", "-C", repoStaleDirty, "push", "-u", "origin", "main").Run()
	_ = exec.Command("git", "-C", repoStaleDirty, "checkout", "-b", "feat/old-dirty").Run()
	_ = os.WriteFile(filepath.Join(repoStaleDirty, "pushed.txt"), []byte("pushed work"), 0600)
	_ = exec.Command("git", "-C", repoStaleDirty, "add", "pushed.txt").Run()
	cmdOld := exec.Command("git", "-C", repoStaleDirty, "commit", "-m", "old commit")
	cmdOld.Env = append(os.Environ(), "GIT_COMMITTER_DATE=1600000000", "GIT_AUTHOR_DATE=1600000000")
	_ = cmdOld.Run()
	_ = exec.Command("git", "-C", repoStaleDirty, "push", "-u", "origin", "feat/old-dirty").Run()

	// Create an old uncommitted change on file.txt (which was NOT modified between main and feat/old-dirty)
	oldTime := time.Now().Add(-72 * time.Hour)
	_ = os.WriteFile(filepath.Join(repoStaleDirty, "file.txt"), []byte("local non-conflicting draft"), 0600)
	_ = os.Chtimes(filepath.Join(repoStaleDirty, "file.txt"), oldTime, oldTime)

	// Add a pushed worktree to repoStaleDirty inside workDir -> should be removed and not cause duplicate errors
	wtPushed := filepath.Join(workDir, "stale-dirty-ok-wt")
	_ = exec.Command("git", "-C", repoStaleDirty, "worktree", "add", "-b", "feat/wt-pushed", wtPushed, "main").Run()
	_ = os.WriteFile(filepath.Join(wtPushed, "wt.txt"), []byte("wt pushed"), 0600)
	_ = exec.Command("git", "-C", wtPushed, "add", "wt.txt").Run()
	cmdWTOld := exec.Command("git", "-C", wtPushed, "commit", "-m", "old wt commit")
	cmdWTOld.Env = append(os.Environ(), "GIT_COMMITTER_DATE=1600000000", "GIT_AUTHOR_DATE=1600000000")
	_ = cmdWTOld.Run()
	_ = exec.Command("git", "-C", wtPushed, "push", "-u", "origin", "feat/wt-pushed").Run()

	// 2. Stale dirty repo WITH conflict -> should stay on feat/conflict!
	repoConflict := initRepo("stale-dirty-conflict")
	_ = exec.Command("git", "-C", repoConflict, "checkout", "-b", "feat/conflict").Run()
	_ = os.WriteFile(filepath.Join(repoConflict, "file.txt"), []byte("committed on branch"), 0600)
	_ = exec.Command("git", "-C", repoConflict, "add", "file.txt").Run()
	cmdConflict := exec.Command("git", "-C", repoConflict, "commit", "-m", "branch commit")
	cmdConflict.Env = append(os.Environ(), "GIT_COMMITTER_DATE=1600000000", "GIT_AUTHOR_DATE=1600000000")
	_ = cmdConflict.Run()
	// Now modify file.txt again without committing (so switching to main conflicts with file.txt)
	_ = os.WriteFile(filepath.Join(repoConflict, "file.txt"), []byte("uncommitted conflict"), 0600)
	_ = os.Chtimes(filepath.Join(repoConflict, "file.txt"), oldTime, oldTime)

	// 3. Recent dirty repo (< 48h) -> should stay on feat/recent-dirty!
	repoRecentDirty := initRepo("recent-dirty")
	_ = exec.Command("git", "-C", repoRecentDirty, "checkout", "-b", "feat/recent-dirty").Run()
	_ = os.WriteFile(filepath.Join(repoRecentDirty, "file.txt"), []byte("recent uncommitted work"), 0600)

	scanner := NewScanner(workDir, vaultDir)
	rep, err := scanner.Scan(true)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Verify repoStaleDirty switched to main and pruned feat/old-dirty (pushed) and wtPushed
	outBranch1, _ := exec.Command("git", "-C", repoStaleDirty, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if got := string(outBranch1); got != "main\n" {
		t.Errorf("expected stale-dirty-ok to switch to 'main', got %q", got)
	}
	if err := exec.Command("git", "-C", repoStaleDirty, "rev-parse", "--verify", "refs/heads/feat/old-dirty").Run(); err == nil {
		t.Errorf("expected pushed branch feat/old-dirty to be deleted after switching to main")
	}
	if _, err := os.Stat(wtPushed); !os.IsNotExist(err) {
		t.Errorf("expected pushed worktree %s to be deleted", wtPushed)
	}
	if rep.PrunedWorktreesCount < 1 {
		t.Errorf("expected PrunedWorktreesCount >= 1, got %d", rep.PrunedWorktreesCount)
	}

	// Verify repoConflict remained on feat/conflict
	outBranch2, _ := exec.Command("git", "-C", repoConflict, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if got := string(outBranch2); got != "feat/conflict\n" {
		t.Errorf("expected stale-dirty-conflict to remain on 'feat/conflict', got %q", got)
	}

	// Verify repoRecentDirty remained on feat/recent-dirty
	outBranch3, _ := exec.Command("git", "-C", repoRecentDirty, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if got := string(outBranch3); got != "feat/recent-dirty\n" {
		t.Errorf("expected recent-dirty to remain on 'feat/recent-dirty', got %q", got)
	}
}
