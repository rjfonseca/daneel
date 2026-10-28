# Operations Suite: Telemetry, Briefings & Engineering Copilots

Daneel includes specialized operational subsystems designed for engineering leaders and senior software practitioners to automate administrative overhead and strategic synthesis.

---

## 1. Executive Performance Review Dossier (`daneel dossier`)

Compiles an executive performance review dossier summarizing technical impact, time distribution, and leadership contributions over a semester or annual cycle:

```bash
# Compile dossier for the current year
daneel dossier

# Specify custom year and semester
daneel dossier --year 2026 --semester 2

# Output structured JSON for automation
daneel dossier --json
```

### Dossier Contents
- **Time Telemetry**: Consolidated hours across Strategic, Local Impact, Glue Work, and Overhead archetypes.
- **Architectural Deliverables**: High-impact RFCs, ADRs, and technical designs authored during the evaluation period.
- **Mentorship & People Impact**: 1:1 alignments conducted, mentee growth summaries, and feedback entries.
- **Operational Health**: Key incidents resolved, post-mortems authored, and tech debt mitigated.

---

## 2. 1:1 Strategic Briefing Generator (`daneel prep`)

Generates concise, context-rich briefings ahead of 1:1s or technical alignments:

```bash
# Generate briefing for a person by slug or name
daneel prep "jane-doe"
```

### Briefing Synthesis
- **Profile Summary**: Role, team, shared initiatives.
- **Interaction History**: Chronological log of past meetings and discussion points extracted from `sources/meetings/` and `daily/`.
- **Pending Action Items**: Open tasks and follow-ups involving the person.
- **Recent Mentions**: Synaptic backlinks where the person was referenced across notes.

---

## 3. Meeting Notes Ingestion Engine (`daneel ingest`)

Batch processes raw meeting notes and transcripts (such as Google Meet or Gemini outputs) dropped into `inbox/`:

```bash
# Ingest all candidate meeting notes in inbox/
daneel ingest

# Ingest from an archived zip file
daneel ingest --zip ~/Downloads/meeting-notes.zip

# Preview changes without modifying vault or removing originals
daneel ingest --dry-run
```

### Pipeline Actions
1. Identifies attendees and extracts email/name tokens.
2. Resolves attendee names to corresponding profiles in `notes/people/` or `meta/people/`.
3. Appends interaction dates and meeting titles to attendee profile timelines.
4. Moves sanitized notes to `sources/meetings/` with normalized metadata.

---

## 4. Local Workspace Scanner (`daneel workspace`)

Scans local development repositories in `~/Work` (or configured directories), performs automated branch and worktree hygiene, and generates a unified Map of Content:

```bash
daneel workspace
```

- Discovers Git remotes, primary branches, and active worktrees via the internal `internal/git` package.
- **Worktree Pruning**: Automatically removes linked Git worktrees (`git worktree`) whose branches have been merged into `main`/`master` or pushed to `origin` (provided any uncommitted files in the worktree are older than `--inactivity-threshold`, default `48h`).
- **Stale Dirty Branch Recovery**: When a repository is on an inactive feature branch ($\ge 48\text{h}$), attempts to switch back to `main`/`master` even if uncommitted files exist, provided those files are also older than the inactivity threshold and switching produces no conflicts. If a conflict occurs, local changes are left untouched.
- **Merged & Pushed Branch Pruning**: Removes local branches (excluding `main`, `master`, `develop`, and active worktree branches) that have already been merged into trunk or pushed to remote tracking branches.
- Detects programming languages and frameworks.
- Generates `notes/organizacao/moc-workspace-work.md` linking notes directly to local codebases.

---

## 5. Cognitive Repository Cloner (`daneel clone`)

Clones a remote repository, inspects its architecture, and scaffolds a technical system note:

```bash
# Clone by short slug (defaults to github.com)
daneel clone owner/repo

# Clone via SSH URL and immediately trigger a Dream Cycle pass
daneel clone git@github.com:org/platform-service.git --dream
```

- Clones the target into `~/Work/<host>/<org>/<repo>`.
- Detects stack components (`go.mod`, `package.json`, `Cargo.toml`, `Dockerfile`).
- Automatically creates `notes/systems/<repo>.md` and links it to `moc-systems.md`.
