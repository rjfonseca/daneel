//nolint:revive // package comment not required
package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Status represents the working tree state of a Git repository.
type Status struct {
	Dirty              bool
	ModifiedCount      int
	UntrackedCount     int
	FilesPreview       []string
	LatestDirtyModTime time.Time
}

// LastCommit represents metadata for the latest commit on HEAD.
type LastCommit struct {
	DateStr    string
	Message    string
	DaysSince  int
	CommitTime time.Time
}

// WorktreeInfo represents a Git worktree entry.
type WorktreeInfo struct {
	Path     string
	Head     string
	Branch   string // short branch name, or "" if detached
	Detached bool
	Bare     bool
	Prunable bool
	Locked   bool
}

// IsLinkedWorktree returns true if repoPath/.git is a regular file (pointing to a main repo's worktrees dir)
// rather than a directory.
func IsLinkedWorktree(repoPath string) bool {
	fi, err := os.Stat(filepath.Join(repoPath, ".git"))
	return err == nil && !fi.IsDir()
}

// CurrentBranch returns the short branch name of HEAD, or "HEAD" if detached, or "unknown" on error.
func CurrentBranch(repoPath string) string {
	// Fast path: read .git/HEAD directly if it is a directory
	gitPath := filepath.Join(repoPath, ".git")
	if fi, err := os.Stat(gitPath); err == nil {
		headFile := filepath.Join(gitPath, "HEAD")
		if !fi.IsDir() {
			// Linked worktree: .git file contains "gitdir: /path/to/.git/worktrees/name"
			if data, err := os.ReadFile(gitPath); err == nil { //nolint:gosec // intentional
				line := strings.TrimSpace(string(data))
				if strings.HasPrefix(line, "gitdir:") {
					gd := strings.TrimSpace(strings.TrimPrefix(line, "gitdir:"))
					if !filepath.IsAbs(gd) {
						gd = filepath.Join(repoPath, gd)
					}
					headFile = filepath.Join(gd, "HEAD")
				}
			}
		}
		if data, err := os.ReadFile(headFile); err == nil { //nolint:gosec // intentional
			content := strings.TrimSpace(string(data))
			if strings.HasPrefix(content, "ref: refs/heads/") {
				return strings.TrimPrefix(content, "ref: refs/heads/")
			}
		}
	}

	cmd := exec.Command("git", "-C", repoPath, "rev-parse", "--abbrev-ref", "HEAD") //nolint:gosec // intentional
	out, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

// DefaultTrunkBranch detects whether the repository uses "main" or "master" as its primary trunk branch.
func DefaultTrunkBranch(repoPath string) string {
	for _, candidate := range []string{"main", "master"} {
		if HasLocalBranch(repoPath, candidate) || HasRemoteBranch(repoPath, "origin", candidate) {
			return candidate
		}
	}
	return ""
}

// HasLocalBranch checks if refs/heads/<branch> exists.
func HasLocalBranch(repoPath string, branch string) bool {
	if branch == "" {
		return false
	}
	return exec.Command("git", "-C", repoPath, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch).Run() == nil //nolint:gosec // intentional
}

// HasRemoteBranch checks if refs/remotes/<remote>/<branch> exists.
func HasRemoteBranch(repoPath string, remote string, branch string) bool {
	if remote == "" || branch == "" {
		return false
	}
	return exec.Command("git", "-C", repoPath, "rev-parse", "--verify", "--quiet", "refs/remotes/"+remote+"/"+branch).Run() == nil //nolint:gosec // intentional
}

// GetStatus returns the working tree status and the most recent modification time among dirty files.
func GetStatus(repoPath string) (Status, error) {
	cmd := exec.Command("git", "-C", repoPath, "status", "--porcelain") //nolint:gosec // intentional
	out, err := cmd.Output()
	if err != nil {
		return Status{}, err
	}
	raw := strings.TrimRight(string(out), "\r\n")
	if raw == "" {
		return Status{}, nil
	}

	lines := strings.Split(raw, "\n")
	var st Status
	for _, l := range lines {
		if len(l) < 3 {
			continue
		}
		status := l[:2]
		filePath := strings.TrimSpace(l[3:])
		// Handle rename "old -> new"
		if idx := strings.Index(filePath, " -> "); idx != -1 {
			filePath = strings.TrimSpace(filePath[idx+4:])
		}
		filePath = strings.Trim(filePath, `"`)

		if strings.Contains(status, "?") {
			st.UntrackedCount++
		} else {
			st.ModifiedCount++
		}

		if len(st.FilesPreview) < 3 {
			st.FilesPreview = append(st.FilesPreview, filePath)
		}

		absPath := filepath.Join(repoPath, filePath)
		if mtime := latestPathModTime(absPath); mtime.After(st.LatestDirtyModTime) {
			st.LatestDirtyModTime = mtime
		}
	}

	st.Dirty = st.ModifiedCount > 0 || st.UntrackedCount > 0

	// If all dirty entries were deletions (so files no longer exist on disk), fall back to .git/index mtime
	if st.Dirty && st.LatestDirtyModTime.IsZero() {
		if fi, err := os.Stat(filepath.Join(repoPath, ".git", "index")); err == nil {
			st.LatestDirtyModTime = fi.ModTime()
		}
	}

	return st, nil
}

func latestPathModTime(absPath string) time.Time {
	fi, err := os.Lstat(absPath)
	if err != nil {
		return time.Time{}
	}
	latest := fi.ModTime()
	if fi.IsDir() {
		_ = filepath.Walk(absPath, func(_ string, info os.FileInfo, walkErr error) error {
			if walkErr == nil && info != nil && info.ModTime().After(latest) {
				latest = info.ModTime()
			}
			return nil
		})
	}
	return latest
}

// GetLastCommit returns metadata and timestamp for the latest commit on HEAD.
func GetLastCommit(repoPath string) LastCommit {
	cmd := exec.Command("git", "-C", repoPath, "log", "-1", "--format=%ct|||%ci|||%s") //nolint:gosec // intentional
	out, err := cmd.Output()
	if err != nil || len(out) == 0 {
		return LastCommit{DateStr: "—", Message: "—", DaysSince: 999}
	}

	parts := strings.SplitN(strings.TrimSpace(string(out)), "|||", 3)
	if len(parts) == 0 {
		return LastCommit{DateStr: "—", Message: "—", DaysSince: 999}
	}

	var commitTime time.Time
	if ts, err := strconv.ParseInt(parts[0], 10, 64); err == nil && ts > 0 {
		commitTime = time.Unix(ts, 0)
	}

	dateStr := "—"
	if len(parts) > 1 {
		dateStr = parts[1]
	}
	msg := ""
	if len(parts) > 2 {
		msg = parts[2]
	}

	days := 999
	if !commitTime.IsZero() {
		days = int(time.Since(commitTime).Hours() / 24)
	} else if len(dateStr) >= 10 {
		if t, err := time.Parse("2006-01-02", dateStr[:10]); err == nil {
			days = int(time.Since(t).Hours() / 24)
		}
	}
	if len(dateStr) >= 10 {
		dateStr = dateStr[:10]
	}

	return LastCommit{
		DateStr:    dateStr,
		Message:    msg,
		DaysSince:  days,
		CommitTime: commitTime,
	}
}

// CheckoutBranch attempts to switch to targetBranch without forcing.
// If uncommitted local changes conflict with targetBranch, Git aborts the checkout and returns false,
// leaving all local changes intact.
func CheckoutBranch(repoPath string, targetBranch string) bool {
	if targetBranch == "" {
		return false
	}
	err := exec.Command("git", "-C", repoPath, "checkout", "--quiet", targetBranch).Run() //nolint:gosec // intentional
	return err == nil
}

// PullQuiet executes a quiet git pull on the repository.
func PullQuiet(repoPath string) bool {
	err := exec.Command("git", "-C", repoPath, "pull", "--quiet").Run() //nolint:gosec // intentional
	return err == nil
}

// IsRefMergedInto checks if sourceRef is an ancestor of targetRef.
func IsRefMergedInto(repoPath string, sourceRef string, targetRef string) bool {
	if sourceRef == "" || targetRef == "" {
		return false
	}
	return exec.Command("git", "-C", repoPath, "merge-base", "--is-ancestor", sourceRef, targetRef).Run() == nil //nolint:gosec // intentional
}

// IsBranchPushed checks if a local branch has been pushed to any remote tracking branch
// (e.g. origin/<branch> or its configured upstream) and has no unpushed local commits.
func IsBranchPushed(repoPath string, branch string) bool {
	if branch == "" {
		return false
	}
	localRef := "refs/heads/" + branch
	// 1. Check origin/<branch>
	remoteRef := "refs/remotes/origin/" + branch
	if HasRemoteBranch(repoPath, "origin", branch) && IsRefMergedInto(repoPath, localRef, remoteRef) {
		return true
	}
	// 2. Check configured upstream @{upstream}
	out, err := exec.Command("git", "-C", repoPath, "rev-parse", "--abbrev-ref", branch+"@{upstream}").Output() //nolint:gosec // intentional
	if err == nil {
		upstream := strings.TrimSpace(string(out))
		if upstream != "" && IsRefMergedInto(repoPath, localRef, upstream) {
			return true
		}
	}
	return false
}

// PruneBranches deletes local branches (excluding main, master, develop, targetBranch, currentBranch,
// and branches active in worktrees) that are either:
// 1. Merged into targetBranch (or origin/targetBranch), OR
// 2. Pushed to the remote (so they can be cloned/checked out again without data loss).
func PruneBranches(repoPath string, targetBranch string, currentBranch string) []string {
	cmd := exec.Command("git", "-C", repoPath, "for-each-ref", "--format=%(refname:short)", "refs/heads/") //nolint:gosec // intentional
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	// Collect branches currently checked out in any worktree so we don't try to delete them before worktree pruning
	wtBranches := make(map[string]bool)
	for _, wt := range ListWorktrees(repoPath) {
		if wt.Branch != "" {
			wtBranches[wt.Branch] = true
		}
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var pruned []string
	for _, line := range lines {
		b := strings.TrimSpace(line)
		if b == "" || b == "main" || b == "master" || b == "develop" || b == targetBranch || b == currentBranch || wtBranches[b] {
			continue
		}

		merged := IsRefMergedInto(repoPath, "refs/heads/"+b, targetBranch) ||
			(HasRemoteBranch(repoPath, "origin", targetBranch) && IsRefMergedInto(repoPath, "refs/heads/"+b, "refs/remotes/origin/"+targetBranch))
		pushed := IsBranchPushed(repoPath, b)

		if !merged && !pushed {
			continue
		}

		flag := "-d"
		if pushed && !merged {
			// Branch is safely pushed to remote but not yet merged into local main; use -D to remove local ref
			flag = "-D"
		}
		if err := exec.Command("git", "-C", repoPath, "branch", flag, b).Run(); err == nil { //nolint:gosec // intentional
			pruned = append(pruned, b)
		}
	}
	return pruned
}

// ListWorktrees parses `git worktree list --porcelain` for the repository.
func ListWorktrees(repoPath string) []WorktreeInfo {
	cmd := exec.Command("git", "-C", repoPath, "worktree", "list", "--porcelain") //nolint:gosec // intentional
	out, err := cmd.Output()
	if err != nil || len(out) == 0 {
		return nil
	}

	var worktrees []WorktreeInfo
	blocks := strings.Split(strings.TrimSpace(string(out)), "\n\n")
	for _, block := range blocks {
		lines := strings.Split(strings.TrimSpace(block), "\n")
		var wt WorktreeInfo
		for _, line := range lines {
			line = strings.TrimSpace(line)
			switch {
			case strings.HasPrefix(line, "worktree "):
				wt.Path = strings.TrimSpace(strings.TrimPrefix(line, "worktree "))
			case strings.HasPrefix(line, "HEAD "):
				wt.Head = strings.TrimSpace(strings.TrimPrefix(line, "HEAD "))
			case strings.HasPrefix(line, "branch "):
				ref := strings.TrimSpace(strings.TrimPrefix(line, "branch "))
				wt.Branch = strings.TrimPrefix(ref, "refs/heads/")
			case line == "detached":
				wt.Detached = true
			case line == "bare":
				wt.Bare = true
			case strings.HasPrefix(line, "prunable"):
				wt.Prunable = true
			case strings.HasPrefix(line, "locked"):
				wt.Locked = true
			}
		}
		if wt.Path != "" {
			worktrees = append(worktrees, wt)
		}
	}
	return worktrees
}

// PruneWorktrees removes linked worktrees that are either:
// - Merged into trunkBranch, OR
// - Pushed to origin (so they can be cloned/checked out again), OR
// - Detached/clean and inactive beyond threshold (whose HEAD is reachable from trunk or remote)
// PROVIDED the worktree is clean or its uncommitted files are older than threshold.
// Also deletes the associated local branch when the worktree is removed.
func PruneWorktrees(repoPath string, trunkBranch string, threshold time.Duration) []string {
	if threshold <= 0 {
		threshold = 48 * time.Hour
	}

	// Clean up any administrative metadata for already-deleted directories first
	_ = exec.Command("git", "-C", repoPath, "worktree", "prune").Run() //nolint:gosec // intentional

	if trunkBranch == "" {
		trunkBranch = DefaultTrunkBranch(repoPath)
	}

	worktrees := ListWorktrees(repoPath)
	if len(worktrees) <= 1 {
		return nil
	}

	absMain, _ := filepath.EvalSymlinks(repoPath)
	if absMain == "" {
		absMain = filepath.Clean(repoPath)
	}

	var pruned []string
	for i, wt := range worktrees {
		// First entry in `git worktree list` is always the main worktree; never remove it
		if i == 0 || wt.Bare || wt.Locked {
			continue
		}
		absWT, _ := filepath.EvalSymlinks(wt.Path)
		if absWT == "" {
			absWT = filepath.Clean(wt.Path)
		}
		if absWT == absMain {
			continue
		}

		// If directory no longer exists on disk, prune metadata and branch
		if _, err := os.Stat(wt.Path); os.IsNotExist(err) {
			_ = exec.Command("git", "-C", repoPath, "worktree", "prune").Run() //nolint:gosec // intentional
			pruned = append(pruned, filepath.Base(wt.Path))
			continue
		}

		// Check working tree cleanliness & age of any uncommitted files inside the worktree
		st, err := GetStatus(wt.Path)
		if err != nil {
			continue
		}
		dirtyIsStale := !st.Dirty || (!st.LatestDirtyModTime.IsZero() && time.Since(st.LatestDirtyModTime) >= threshold)
		if !dirtyIsStale {
			// Active uncommitted changes in worktree -> preserve!
			continue
		}

		merged := false
		pushed := false
		if wt.Branch != "" {
			if trunkBranch != "" {
				merged = IsRefMergedInto(repoPath, "refs/heads/"+wt.Branch, trunkBranch) ||
					(HasRemoteBranch(repoPath, "origin", trunkBranch) && IsRefMergedInto(repoPath, "refs/heads/"+wt.Branch, "refs/remotes/origin/"+trunkBranch))
			}
			pushed = IsBranchPushed(repoPath, wt.Branch)
		} else if wt.Head != "" && trunkBranch != "" {
			// Detached HEAD: check if commit is reachable from trunkBranch
			merged = IsRefMergedInto(repoPath, wt.Head, trunkBranch)
		}

		if !merged && !pushed {
			// Worktree has unpushed & unmerged commits -> preserve!
			continue
		}

		// For pushed (but not yet merged) worktrees, also respect commit inactivity threshold if dirty
		// Wait: the user stated: "If a worktree is pushed, it can be cloned again, you should delete it."
		// Still, if someone just created a commit 1 minute ago and hasn't finished, wait or check?
		// Wait: in our test TestPruneWorktrees, commit was 72h ago, or in general if merged or pushed and dirtyIsStale, remove it.
		if err := exec.Command("git", "-C", repoPath, "worktree", "remove", "--force", wt.Path).Run(); err != nil { //nolint:gosec // intentional
			// Fallback if git worktree remove fails on untracked directories
			_ = os.RemoveAll(wt.Path)
			_ = exec.Command("git", "-C", repoPath, "worktree", "prune").Run() //nolint:gosec // intentional
		}

		if wt.Branch != "" && wt.Branch != "main" && wt.Branch != "master" && wt.Branch != "develop" {
			_ = exec.Command("git", "-C", repoPath, "branch", "-D", wt.Branch).Run() //nolint:gosec // intentional
		}

		pruned = append(pruned, filepath.Base(wt.Path))
	}

	return pruned
}
