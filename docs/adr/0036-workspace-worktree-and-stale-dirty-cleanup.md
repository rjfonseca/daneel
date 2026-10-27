# ADR 0036: Workspace Worktree Pruning, Pushed Branch Cleanup, and Stale Dirty Checkout

* Status: accepted
* Date: 2026-10-27

## Context and Problem Statement

`daneel workspace` (initially specified in [ADR 0022](0022-workspace-branch-inactivity-and-merged-pruning.md)) previously bypassed any repository with uncommitted or untracked changes (`dirty == true`), never cleaned up linked Git worktrees (`git worktree`), and only pruned local branches when they were explicitly merged into the local `main`/`master` branch.

In day-to-day multi-agent and Staff Engineering workflows:
1. **Orphaned & Pushed Linked Worktrees**: AI coding agents and parallel feature workflows frequently create linked Git worktrees (`git worktree add`). Once a branch is pushed to `origin` or merged into trunk, the worktree directory remains on disk indefinitely, cluttering `~/Work` and preventing the local branch from being deleted.
2. **Pushed Unmerged Local Branches**: Feature branches pushed to `origin/<branch>` can always be fetched or checked out again on demand. Keeping stale local branch pointers after work has moved to remote PRs creates local branch sprawl.
3. **Stale Uncommitted Files Blocking Trunk Reset**: A repository left on an old feature branch with forgotten scratch files or non-conflicting local edits older than the inactivity threshold (`48h` default) remained stuck on that stale branch forever because `syncClean` skipped any dirty working tree.

Furthermore, Daneel requires minimal external dependencies in `go.mod` while centralizing Git operations in a dedicated internal Go package (`internal/git`).

## Decision Drivers

* **Minimal Dependency Footprint**: Avoid adding heavy third-party Git libraries (such as `go-git/v5`, which introduces ~25 transitive dependencies, lacks native `git worktree` lifecycle management, and does not perform Git's safe 3-way conflict check on `Keep: true` checkout).
* **Dedicated Internal Git Package (`internal/git`)**: Provide a unified, zero-dependency Go package (`internal/git`) combining fast direct filesystem inspection (`.git/HEAD`, `.git/worktrees`, dirty file `mtime`s) with safe Git CLI execution.
* **Aggressive but Safe Worktree & Branch Pruning**:
  * Delete linked worktrees and local branches when they are **merged** into trunk (`main`/`master`) **or** **pushed** to the remote (`origin/<branch>` or `@{upstream}`), provided any uncommitted files inside a worktree are older than the inactivity threshold.
* **Non-Destructive Stale Dirty Trunk Switch**:
  * When a repository on a feature branch has uncommitted files whose latest modification time (`mtime`) and last commit are both older than `inactivityThreshold` (`48h` default), attempt `git checkout <main|master>`.
  * Rely on Git's native 3-way index/working-tree check: if local modifications do not conflict with switching to trunk, switch to trunk while preserving the uncommitted files; if a conflict exists, abort cleanly and leave the repository untouched.

## Considered Options

* **Option 1: Adopt `github.com/go-git/go-git/v5`**
  * *Disadvantages*: Adds ~25 transitive dependencies to `go.mod`, lacks linked worktree pruning (`git worktree remove`/`prune`), and `CheckoutOptions{Keep: true}` can overwrite modified working-tree files instead of aborting on conflict.
* **Option 2: Zero-Dependency `internal/git` Package with Two-Phase Worktree & Stale-Dirty Cleanup**
  * *Advantages*: Zero external dependencies added to `go.mod`; full support for `git worktree list --porcelain` and `git worktree remove`; exact conflict-safe `git checkout` behavior; two-phase workspace discovery (primary repositories first, surviving linked worktrees second) prevents concurrent scan races.

## Decision Outcome

Chosen option: **Option 2: Zero-Dependency `internal/git` Package with Two-Phase Worktree & Stale-Dirty Cleanup**.

### Lifecycle & Algorithm

1. **Two-Phase Workspace Discovery (`discoverRepoDirs`)**:
   - Distinguishes primary repositories (`.git` is a directory) from linked worktrees (`.git` is a file).
   - Scans primary repositories first so they can prune their linked worktrees before any surviving linked worktrees are inspected.
2. **Linked Worktree Pruning (`git.PruneWorktrees`)**:
   - Runs `git worktree prune` to clear stale administrative entries.
   - Inspects each linked worktree via `git worktree list --porcelain`:
     - Skips the main worktree, bare worktrees, and locked worktrees.
     - Checks `GetStatus(wt.Path)`: if dirty, requires `time.Since(st.LatestDirtyModTime) >= threshold`.
     - Checks if the worktree's branch is **merged** into `main`/`master` (or `origin/<trunk>`) **or** **pushed** to remote (`IsBranchPushed`).
     - If eligible, removes the worktree (`git worktree remove --force`) and deletes its local branch (`git branch -D`).
3. **Stale Dirty Branch Reset (`Scanner.inspectRepo`)**:
   - Computes `LatestDirtyModTime` across all modified and untracked paths reported by `git status --porcelain`.
   - If a primary repository is on a non-trunk branch, `time.Since(lastCommitTime) >= threshold`, and (`!dirty` or `time.Since(LatestDirtyModTime) >= threshold`):
     - Attempts `git.CheckoutBranch(repoPath, "main")` (or `"master"`).
     - If non-conflicting, the repository switches to trunk (pulling updates if clean) and prunes merged/pushed branches.
     - If conflicting, Git aborts the checkout and the repository remains on its current branch with all uncommitted changes intact.
4. **Merged & Pushed Branch Pruning (`git.PruneBranches`)**:
   - Deletes local branches (excluding `main`, `master`, `develop`, `currentBranch`, and branches checked out in active worktrees) when `IsRefMergedInto` trunk **or** `IsBranchPushed` to remote.

## Consequences

### Positive
* **Zero Orphaned Worktrees**: Merged or pushed agent worktrees are automatically cleaned up on `daneel workspace` runs.
* **Incisive Stale Repo Recovery**: Repositories with old forgotten scratch files automatically return to `main`/`master` whenever there is no merge conflict.
* **Zero Dependency Bloat**: `go.mod` remains lean with a clean `internal/git` package boundary.
