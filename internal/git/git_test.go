//nolint:revive // package comment not required
package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func initTestRepo(t *testing.T, dir string) string {
	t.Helper()
	_ = os.MkdirAll(dir, 0750)
	_ = exec.Command("git", "-C", dir, "init", "-b", "main").Run()
	_ = exec.Command("git", "-C", dir, "config", "user.name", "Ada Lovelace").Run()
	_ = exec.Command("git", "-C", dir, "config", "user.email", "ada@example.com").Run()
	_ = exec.Command("git", "-C", dir, "config", "commit.gpgsign", "false").Run()
	_ = os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Test Repo\n"), 0600)
	_ = exec.Command("git", "-C", dir, "add", ".").Run()
	_ = exec.Command("git", "-C", dir, "commit", "-m", "initial commit").Run()
	return dir
}

func commitWithTime(t *testing.T, dir string, msg string, ts time.Time) {
	t.Helper()
	dateStr := ts.Format(time.RFC3339)
	cmd := exec.Command("git", "-C", dir, "commit", "-m", msg)
	cmd.Env = append(os.Environ(), "GIT_COMMITTER_DATE="+dateStr, "GIT_AUTHOR_DATE="+dateStr)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit failed: %v (%s)", err, string(out))
	}
}

func TestStatusAndLatestModificationTime(t *testing.T) {
	tempDir := t.TempDir()
	repo := initTestRepo(t, filepath.Join(tempDir, "repo"))

	st, err := GetStatus(repo)
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}
	if st.Dirty {
		t.Fatalf("expected clean repo, got dirty")
	}

	// Create an untracked file and set its mtime to 72 hours ago
	oldTime := time.Now().Add(-72 * time.Hour)
	untrackedFile := filepath.Join(repo, "notes.tmp")
	_ = os.WriteFile(untrackedFile, []byte("scratch"), 0600)
	_ = os.Chtimes(untrackedFile, oldTime, oldTime)

	st, err = GetStatus(repo)
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}
	if !st.Dirty || st.UntrackedCount != 1 {
		t.Fatalf("expected 1 untracked file, got %+v", st)
	}
	if time.Since(st.LatestDirtyModTime) < 70*time.Hour {
		t.Errorf("expected LatestDirtyModTime ~72h ago, got %v (since: %v)", st.LatestDirtyModTime, time.Since(st.LatestDirtyModTime))
	}
}

func TestCheckoutIfNoConflict(t *testing.T) {
	tempDir := t.TempDir()

	t.Run("switches_when_dirty_file_does_not_conflict", func(t *testing.T) {
		repo := initTestRepo(t, filepath.Join(tempDir, "non-conflict"))
		_ = exec.Command("git", "-C", repo, "checkout", "-b", "feat/old-work").Run()
		// Modify README.md which has the same base on main and feat/old-work
		_ = os.WriteFile(filepath.Join(repo, "README.md"), []byte("# Updated local draft\n"), 0600)

		if !CheckoutBranch(repo, "main") {
			t.Fatalf("expected CheckoutBranch to succeed when dirty file has no conflict with main")
		}
		if got := CurrentBranch(repo); got != "main" {
			t.Errorf("expected branch 'main', got %q", got)
		}
	})

	t.Run("aborts_and_preserves_files_when_dirty_file_conflicts", func(t *testing.T) {
		repo := initTestRepo(t, filepath.Join(tempDir, "conflict"))
		_ = exec.Command("git", "-C", repo, "checkout", "-b", "feat/diverged").Run()
		_ = os.WriteFile(filepath.Join(repo, "README.md"), []byte("# Committed on feature\n"), 0600)
		_ = exec.Command("git", "-C", repo, "add", "README.md").Run()
		commitWithTime(t, repo, "diverge README", time.Now().Add(-96*time.Hour))

		// Now modify README.md in working tree without committing
		dirtyContent := "# Uncommitted conflicting edits\n"
		_ = os.WriteFile(filepath.Join(repo, "README.md"), []byte(dirtyContent), 0600)

		if CheckoutBranch(repo, "main") {
			t.Fatalf("expected CheckoutBranch to fail on conflicting working tree file")
		}
		if got := CurrentBranch(repo); got != "feat/diverged" {
			t.Errorf("expected branch to remain 'feat/diverged', got %q", got)
		}
		data, _ := os.ReadFile(filepath.Join(repo, "README.md"))
		if string(data) != dirtyContent {
			t.Errorf("expected dirty file content to be preserved, got %q", string(data))
		}
	})
}

func TestPruneBranches_MergedAndPushed(t *testing.T) {
	tempDir := t.TempDir()
	remoteDir := filepath.Join(tempDir, "remote.git")
	_ = exec.Command("git", "init", "--bare", "-b", "main", remoteDir).Run()

	repo := initTestRepo(t, filepath.Join(tempDir, "local"))
	_ = exec.Command("git", "-C", repo, "remote", "add", "origin", remoteDir).Run()
	_ = exec.Command("git", "-C", repo, "push", "-u", "origin", "main").Run()

	// 1. Merged local branch (never pushed) -> should be deleted
	_ = exec.Command("git", "-C", repo, "checkout", "-b", "feat/merged-local").Run()
	_ = os.WriteFile(filepath.Join(repo, "merged.txt"), []byte("merged"), 0600)
	_ = exec.Command("git", "-C", repo, "add", ".").Run()
	commitWithTime(t, repo, "merged work", time.Now().Add(-10*time.Hour))
	_ = exec.Command("git", "-C", repo, "checkout", "main").Run()
	_ = exec.Command("git", "-C", repo, "merge", "feat/merged-local").Run()

	// 2. Pushed unmerged branch -> should be deleted because it is pushed to origin
	_ = exec.Command("git", "-C", repo, "checkout", "-b", "feat/pushed-unmerged").Run()
	_ = os.WriteFile(filepath.Join(repo, "pushed.txt"), []byte("pushed"), 0600)
	_ = exec.Command("git", "-C", repo, "add", ".").Run()
	commitWithTime(t, repo, "pushed work", time.Now().Add(-72*time.Hour))
	_ = exec.Command("git", "-C", repo, "push", "-u", "origin", "feat/pushed-unmerged").Run()
	_ = exec.Command("git", "-C", repo, "checkout", "main").Run()

	// 3. Unpushed and unmerged local branch -> MUST be kept!
	_ = exec.Command("git", "-C", repo, "checkout", "-b", "feat/local-only").Run()
	_ = os.WriteFile(filepath.Join(repo, "local.txt"), []byte("local only"), 0600)
	_ = exec.Command("git", "-C", repo, "add", ".").Run()
	commitWithTime(t, repo, "local unpushed work", time.Now().Add(-72*time.Hour))
	_ = exec.Command("git", "-C", repo, "checkout", "main").Run()

	pruned := PruneBranches(repo, "main", "main")

	hasBranch := func(b string) bool {
		return exec.Command("git", "-C", repo, "rev-parse", "--verify", "refs/heads/"+b).Run() == nil
	}

	if hasBranch("feat/merged-local") {
		t.Errorf("expected feat/merged-local to be pruned, pruned=%v", pruned)
	}
	if hasBranch("feat/pushed-unmerged") {
		t.Errorf("expected feat/pushed-unmerged to be pruned because it is pushed to remote, pruned=%v", pruned)
	}
	if !hasBranch("feat/local-only") {
		t.Errorf("expected feat/local-only (unpushed & unmerged) to be preserved!")
	}
}

func TestPruneWorktrees(t *testing.T) {
	tempDir := t.TempDir()
	remoteDir := filepath.Join(tempDir, "remote.git")
	_ = exec.Command("git", "init", "--bare", "-b", "main", remoteDir).Run()

	repo := initTestRepo(t, filepath.Join(tempDir, "main-repo"))
	_ = exec.Command("git", "-C", repo, "remote", "add", "origin", remoteDir).Run()
	_ = exec.Command("git", "-C", repo, "push", "-u", "origin", "main").Run()

	// Worktree 1: Pushed branch -> should be removed even if unmerged into main
	wtPushed := filepath.Join(tempDir, "wt-pushed")
	_ = exec.Command("git", "-C", repo, "worktree", "add", "-b", "feat/wt-pushed", wtPushed).Run()
	_ = os.WriteFile(filepath.Join(wtPushed, "wt1.txt"), []byte("pushed wt"), 0600)
	_ = exec.Command("git", "-C", wtPushed, "add", ".").Run()
	commitWithTime(t, wtPushed, "wt pushed commit", time.Now().Add(-72*time.Hour))
	_ = exec.Command("git", "-C", wtPushed, "push", "-u", "origin", "feat/wt-pushed").Run()

	// Worktree 2: Merged branch with stale untracked file -> should be removed
	wtMerged := filepath.Join(tempDir, "wt-merged")
	_ = exec.Command("git", "-C", repo, "worktree", "add", "-b", "feat/wt-merged", wtMerged).Run()
	_ = os.WriteFile(filepath.Join(wtMerged, "wt2.txt"), []byte("merged wt"), 0600)
	_ = exec.Command("git", "-C", wtMerged, "add", ".").Run()
	commitWithTime(t, wtMerged, "wt merged commit", time.Now().Add(-10*time.Hour))
	_ = exec.Command("git", "-C", repo, "merge", "feat/wt-merged").Run()
	// Add stale dirty file in wtMerged
	staleFile := filepath.Join(wtMerged, "stale.tmp")
	_ = os.WriteFile(staleFile, []byte("old scratch"), 0600)
	oldTime := time.Now().Add(-72 * time.Hour)
	_ = os.Chtimes(staleFile, oldTime, oldTime)

	// Worktree 3: Unpushed, unmerged branch with local commits -> MUST be preserved
	wtActive := filepath.Join(tempDir, "wt-unpushed")
	_ = exec.Command("git", "-C", repo, "worktree", "add", "-b", "feat/wt-unpushed", wtActive).Run()
	_ = os.WriteFile(filepath.Join(wtActive, "wt3.txt"), []byte("unpushed wt"), 0600)
	_ = exec.Command("git", "-C", wtActive, "add", ".").Run()
	commitWithTime(t, wtActive, "wt unpushed commit", time.Now().Add(-72*time.Hour))

	prunedWTs := PruneWorktrees(repo, "main", 48*time.Hour)

	if _, err := os.Stat(wtPushed); !os.IsNotExist(err) {
		t.Errorf("expected pushed worktree %s to be removed, prunedWTs=%v", wtPushed, prunedWTs)
	}
	if _, err := os.Stat(wtMerged); !os.IsNotExist(err) {
		t.Errorf("expected merged worktree %s to be removed, prunedWTs=%v", wtMerged, prunedWTs)
	}
	if _, err := os.Stat(wtActive); err != nil {
		t.Errorf("expected unpushed unmerged worktree %s to be preserved, err=%v", wtActive, err)
	}
}
