# ADR 0022: Workspace Branch Inactivity Guardrail and Merged Branch Pruning Lifecycle

* Status: accepted
* Date: 2026-09-24

## Context and Problem Statement

Daneel provides continuous workspace scanning and maintenance routines (`daneel workspace`, `scan_workspace_repos` MCP tool, and systemd scheduled timers) that index local repositories under `~/Work`, extract language stacks, detect dirty working trees, and generate Maps of Content (MOCs).

Prior to this ADR, the workspace synchronization logic (`syncClean`) exhibited a disruptive flaw:
1. **Aggressive Branch Reset on Active Work**: Whenever a clean repository (`!dirty`) was checked out on a feature branch, the routine immediately executed `git checkout main` and attempted `git branch -d <branch>`. If a developer was actively working on a feature branch and had committed recently (with a clean working copy), the routine would switch the active branch to `main` in the background, interrupting ongoing work and creating developer friction.
2. **Accumulation of Stale Merged Branches**: Over time, completed and merged PR branches accumulated locally in `~/Work` repositories, requiring tedious manual cleanup (`git branch -d`).

## Decision Drivers

* **Non-Disruptive Maintenance**: Background routines must never switch branches or alter repositories while an engineer is actively working on them.
* **Configurable Temporal Guardrail**: A parameterized inactivity window (with a sensible default of 48 hours) based on committer commit date (`git log -1 --format=%ct`), after which unmerged or abandoned branches can safely be reset to trunk (`main` or `master`).
* **Safe Local Pruning**: Automatically detect and safely delete local branches that have already been merged into the default trunk branch, while strictly protecting trunk branches (`main`, `master`, `develop`) and the currently checked-out branch.
* **Unified Configuration & CLI Ergonomics**: Allow configuration via `.daneel.yaml` (`workspace.branch_inactivity`), CLI flag (`--inactivity-threshold`), and MCP tool arguments.
* **Transparent Observability**: Expose `synced_count` and `pruned_branches_count` in `WorkspaceReport`, HUD/CLI summaries, and SQLite metadata.

## Considered Options

* **Option 1: Disable Automatic Branch Switching Entirely**:
  * *Disadvantages*: Leaves repositories stranded indefinitely on abandoned or completed feature branches; fails to pull trunk updates on dormant projects.
* **Option 2: Branch Reset Dependent Exclusively on Working Tree Status (`!dirty`)**:
  * *Disadvantages*: The existing behavior that provoked this issue; a committed feature branch has `!dirty == true` while still actively under development.
* **Option 3: Temporal Inactivity Guardrail (48h Default) Combined with Trunk Merged Branch Pruning**:
  * Computes the duration since the last commit on the active branch.
  * If `time.Since(lastCommit) < threshold`: The branch is deemed active; Daneel skips switching/resetting.
  * If `time.Since(lastCommit) >= threshold`: Daneel assumes work has finished or is stale, checks out `main` (or `master`), pulls latest changes, and prunes merged branches.
  * When on `main` or `master`: Pulls latest changes and prunes all local branches merged into trunk using safe `git branch -d`.

## Decision Outcome

Chosen option: **Option 3: Temporal Inactivity Guardrail (48h Default) Combined with Trunk Merged Branch Pruning**.

### Lifecycle & Algorithm

```
                 inspectRepo(repoPath, syncClean)
                                │
                        syncClean && !dirty?
                                │
                ┌───────────────┴───────────────┐
               Yes                              No
                │                               │
        Branch is main/master?             Keep as-is
                │
        ┌───────┴───────┐
       Yes              No
        │                │
    git pull       time.Since(lastCommit) >= threshold?
        │                │
        │        ┌───────┴───────┐
        │       Yes              No
        │        │                │
        │   checkout trunk   Active branch:
        │   git pull         Keep current branch!
        │        │
        └────────┼──────────────────────────────┐
                 │                              │
                 ▼                              ▼
    pruneMergedBranches(trunk)              End Sync
    (safe git branch -d)
```

1. **Commit Timestamp Precision**:
   - `getGitLastCommit` retrieves the UNIX timestamp seconds (`%ct`) of HEAD in a single subprocess call:
     `git -C repoPath log -1 --format=%ct|||%ci|||%s`
   - Exact elapsed duration is calculated via `time.Since(commitTime)`.
2. **Inactivity Guardrail**:
   - If `branch != "main" && branch != "master"` and `time.Since(commitTime) < inactivityThreshold` (default `48h`), no branch switching occurs.
   - If `time.Since(commitTime) >= inactivityThreshold`, Daneel checks out `main` (fallback `master`), executes `git pull --quiet`, and invokes branch pruning.
3. **Safe Pruning of Merged Branches**:
   - Queries `git -C repoPath branch --format=%(refname:short) --merged <trunk>`.
   - Explicitly excludes `main`, `master`, `develop`, the target trunk branch, and the current branch.
   - Issues safe deletion (`git branch -d <branch>`). Branches with unmerged commits are protected by git and will never be deleted.
4. **Configuration & Defaults**:
   - `.daneel.yaml`:
     ```yaml
     workspace:
       work_dir: "~/Work"
       branch_inactivity: "48h" # e.g. "48h", "24h", "2d"
     ```
   - CLI flag: `daneel workspace --inactivity-threshold=48h`
   - Default: `48 * time.Hour`.

## Consequences

### Positive
* **Zero Disruption for Active Developers**: Committing work and leaving the working directory clean no longer causes Daneel's background routines to reset the branch.
* **Self-Cleaning Local Repositories**: Repositories on `main`/`master` or reset after inactivity automatically clean up stale merged branches without user intervention.
* **Transparent Metrics**: The CLI outputs synced repositories count and the number of pruned branches (`🌿 Pruned Branches: X merged local branches removed`).
