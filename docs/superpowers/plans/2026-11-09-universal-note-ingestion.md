# Universal Note Ingestion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a universal file and directory note ingestion engine in Daneel, with 100% parity across CLI, Web, TUI, and MCP interfaces.

**Architecture:** A dedicated, robust core engine in `internal/ingest` handles path validation, frontmatter synthesis/preservation, collision-free writing, symlink defense, and FTS5 indexing. The CLI (`daneel ingest`), ConnectRPC server (`IngestPath` / `CaptureNote`), MCP server (`ingest_note`, `ingest_path`), Web UI (`QuickCaptureDialog`), and TUI (`9: 📥 Ingest` tab) all leverage this unified engine.

**Tech Stack:** Go 1.24, Cobra, ConnectRPC / Protobuf, SQLite FTS5 (`mattn/go-sqlite3`), Bubble Tea (`charmbracelet/bubbletea`, `charmbracelet/bubbles/filepicker`), React 19, TypeScript, TailwindCSS, Vitest.

**Spec:** `docs/superpowers/specs/2026-11-09-universal-note-ingestion-design.md`

## Global Constraints

- **Language & Copy:** English-first UI strings, tooltips, dialogs, and messages (Invariant 2.2).
- **Security & Path Traversal:** Ingested files must be constrained strictly within the user's vault root; symlinks pointing outside the vault root or source directory must be rejected.
- **Data Neutrality:** Zero private corporate names, real colleague names, or local personal paths in tests and fixtures (Invariant 2.3).
- **WebFS Exact Sync:** Any changes to `web/` must be built via `npm` and copied byte-for-byte into `internal/webfs/dist/` (Invariant 2.4).
- **ADR Mandatory:** Architectural decisions must be documented in MADR format under `docs/adr/0039-universal-note-ingestion-engine.md` (Invariant 2.1).

## Review Focus

1. **Path Traversal / Escape:** An input path containing `../` or relative escaping outside vault root must be rejected with an explicit error.
2. **File Overwrite Protection:** Ingesting a file when a file of the same name already exists in target destination must not overwrite existing data; it must append a timestamp or collision suffix.
3. **Empty / Non-Markdown Files:** Ingesting non-existent files or directories with no supported files must return a clean, user-friendly error without panic or empty index corruption.
4. **Symlink Loops & Host Boundary:** Symlinks pointing to cyclic directories or outside source bounds must be safely skipped or rejected without infinite loops.
5. **Backwards Compatibility:** Invoking `daneel ingest` with zero positional arguments must preserve existing behavior for processing Google Meet / Gemini meeting drafts in `inbox/`.

---

### Task 1: Architectural Decision Record (ADR 0039)

**Files:**
- Create: `docs/adr/0039-universal-note-ingestion-engine.md`
- Modify: `docs/adr/README.md`

**Interfaces:**
- Produces: MADR documentation defining the Universal Note Ingestion Architecture.

- [ ] **Step 1: Draft MADR 0039**
Create `docs/adr/0039-universal-note-ingestion-engine.md` following standard MADR template, detailing Context, Decision Drivers, Considered Options (Option 1 Unified Engine vs Option 2 Ad-Hoc vs Option 3 Daemon-only), Decision Outcome, and Consequences.

- [ ] **Step 2: Update ADR Index**
Add entry for `0039-universal-note-ingestion-engine.md` in `docs/adr/README.md`.

- [ ] **Step 3: Commit**
```bash
git add docs/adr/0039-universal-note-ingestion-engine.md docs/adr/README.md
git commit -m "docs(adr): document universal note ingestion engine architecture in ADR 0039"
```

---

### Task 2: Core Ingestion Package (`internal/ingest`)

**Files:**
- Create: `internal/ingest/types.go`
- Create: `internal/ingest/engine.go`
- Create: `internal/ingest/engine_test.go`

**Interfaces:**
- Produces:
  - `type IngestOptions struct { Destination string; Recursive bool; Move bool; SourceURL string; Tags []string }`
  - `type IngestedNote struct { SourcePath string; RelPath string; Title string; SizeBytes int64; Warning string }`
  - `type IngestReport struct { Items []IngestedNote; TotalProcessed int; Errors []string }`
  - `type Engine struct { ... }`
  - `func NewEngine(vaultRoot string, searchEngine *search.Engine, eventHub daemon.Hub) *Engine`
  - `func (e *Engine) IngestPath(ctx context.Context, hostPath string, opts IngestOptions) (*IngestReport, error)`
  - `func (e *Engine) IngestContent(ctx context.Context, title string, content []byte, opts IngestOptions) (*IngestedNote, error)`

- [ ] **Step 1: Write failing tests in `internal/ingest/engine_test.go`**
Test single markdown file ingestion, existing frontmatter preservation, frontmatter synthesis when missing, collision-free renaming, directory recursive ingestion, symlink escape rejection, and `--move` flag behavior.

- [ ] **Step 2: Run tests to verify failure**
Run: `go test -v ./internal/ingest/...`
Expected: FAIL (types and package do not exist).

- [ ] **Step 3: Implement `types.go` and `engine.go`**
Implement `IngestOptions`, `IngestedNote`, `IngestReport`, path validation against vault root, safe file copying (`safefile` / `os.OpenRoot`), collision resolution using timestamp suffixes, and frontmatter parsing/synthesis.

- [ ] **Step 4: Run tests to verify they pass**
Run: `go test -v -race ./internal/ingest/...`
Expected: PASS.

- [ ] **Step 5: Commit**
```bash
git add internal/ingest/
git commit -m "feat(ingest): implement unified note ingestion engine core with collision safety"
```

---

### Task 3: Ingest Engine Indexing & Live UI Events

**Files:**
- Modify: `internal/ingest/engine.go`
- Modify: `internal/ingest/engine_test.go`

**Interfaces:**
- Consumes: `*search.Engine` from `internal/search`, `daemon.Hub` from `internal/daemon`.
- Produces: Real-time FTS5 indexing of ingested notes and dispatch of `UIActionType_ACTION_GALAXY_RELOAD` to event hub.

- [ ] **Step 1: Write failing tests for search indexing and event publishing**
In `internal/ingest/engine_test.go`, test that `engine.IndexNote` is invoked for each ingested note, and when `eventHub` is provided, a reload event is dispatched.

- [ ] **Step 2: Run tests to verify failure**
Run: `go test -v ./internal/ingest/... -run TestIndexing`
Expected: FAIL.

- [ ] **Step 3: Implement indexing and event dispatching in `internal/ingest/engine.go`**
Connect `searchEngine.IndexNote(ctx, item.RelPath)` and `eventHub.PublishUIEvent(...)`.

- [ ] **Step 4: Run tests to verify pass**
Run: `go test -v ./internal/ingest/...`
Expected: PASS.

- [ ] **Step 5: Commit**
```bash
git add internal/ingest/
git commit -m "feat(ingest): add automated FTS5 indexing and live galaxy event dispatching"
```

---

### Task 4: ConnectRPC Protocol & Server Implementation

**Files:**
- Modify: `api/v1/graph.proto`
- Generate: `api/v1/graph.pb.go`, `api/v1/graphv1connect/graph.connect.go`, `web/src/gen/*`
- Create: `cmd/serve_ingest.go`
- Modify: `cmd/serve_capture.go`
- Create: `cmd/serve_ingest_test.go`

**Interfaces:**
- Produces:
  - `rpc IngestPath(IngestPathRequest) returns (IngestPathResponse)`
  - Updated `CaptureNoteRequest` with `destination` field.
  - Connect handler `(s *GraphServer) IngestPath(ctx, req)`

- [ ] **Step 1: Update `api/v1/graph.proto`**
Add `IngestPath` RPC, `IngestPathRequest`, `IngestedItem`, and `IngestPathResponse`. Add `destination` string to `CaptureNoteRequest`.

- [ ] **Step 2: Run code generation**
Run: `PATH="$PWD/web/node_modules/.bin:$PATH" buf generate`
Verify generated Go and TS files.

- [ ] **Step 3: Write failing tests in `cmd/serve_ingest_test.go`**
Test `IngestPath` with valid file, directory, and invalid path; test `CaptureNote` with custom `destination`.

- [ ] **Step 4: Implement `cmd/serve_ingest.go` and update `cmd/serve_capture.go`**
Wire `IngestPath` to `internal/ingest.Engine`. Update `CaptureNote` to support target destination folder within vault.

- [ ] **Step 5: Run tests to verify pass**
Run: `go test -v ./cmd/... -run TestIngest`
Expected: PASS.

- [ ] **Step 6: Commit**
```bash
git add api/v1/ cmd/serve_ingest* cmd/serve_capture* web/src/gen/
git commit -m "feat(api): add IngestPath ConnectRPC endpoint and target destination support"
```

---

### Task 5: Generalize CLI Command (`cmd/ingest.go`)

**Files:**
- Modify: `cmd/ingest.go`
- Create: `cmd/ingest_test.go`

**Interfaces:**
- Produces:
  - `daneel ingest <paths...> [flags]`
  - Flags: `-d, --dest`, `-r, --recursive`, `-m, --move`, `--reindex`
  - Backwards-compatible fallback to `meetings.Processor` when no positional arguments are provided.

- [ ] **Step 1: Write failing CLI tests in `cmd/ingest_test.go`**
Test:
1. `daneel ingest <file>` ingests single file into `inbox/`.
2. `daneel ingest <dir> --dest notes/tech` ingests directory into `notes/tech/`.
3. `daneel ingest` with no args executes meeting processing as before.

- [ ] **Step 2: Run CLI tests to verify failure**
Run: `go test -v ./cmd/... -run TestIngestCmd`
Expected: FAIL.

- [ ] **Step 3: Update `cmd/ingest.go`**
Refactor `ingestCmd` to inspect `args []string`:
- If `len(args) > 0`, invoke `internal/ingest.Engine.IngestPath` for each argument with flags.
- If `len(args) == 0`, retain existing `meetings.Processor` workflow.

- [ ] **Step 4: Run CLI tests to verify pass**
Run: `go test -v ./cmd/... -run TestIngestCmd`
Expected: PASS.

- [ ] **Step 5: Commit**
```bash
git add cmd/ingest.go cmd/ingest_test.go
git commit -m "feat(cli): generalize daneel ingest to accept arbitrary files and directories"
```

---

### Task 6: MCP Tools (`internal/mcp/server.go`)

**Files:**
- Modify: `internal/mcp/server.go`
- Modify: `internal/mcp/server_test.go`

**Interfaces:**
- Produces:
  - Tool `ingest_note`: writes raw note with title, content, destination, tags.
  - Tool `ingest_path`: ingests host file or directory path into vault.

- [ ] **Step 1: Write failing tests in `internal/mcp/server_test.go`**
Test tool schemas and execution for `ingest_note` and `ingest_path`.

- [ ] **Step 2: Run MCP tests to verify failure**
Run: `go test -v ./internal/mcp/... -run TestIngestTools`
Expected: FAIL.

- [ ] **Step 3: Implement tools in `internal/mcp/server.go`**
Add tool definitions to `ListTools()` and cases in `CallTool()` calling `internal/ingest.Engine`.

- [ ] **Step 4: Run MCP tests to verify pass**
Run: `go test -v ./internal/mcp/...`
Expected: PASS.

- [ ] **Step 5: Commit**
```bash
git add internal/mcp/
git commit -m "feat(mcp): expose ingest_note and ingest_path tools for AI agents"
```

---

### Task 7: Web UI File Ingestion (`web/src/components/QuickCaptureDialog.tsx`)

**Files:**
- Modify: `web/src/components/QuickCaptureDialog.tsx`
- Modify: `web/src/lib/apiClient.ts`
- Modify: `web/tests/QuickCaptureDialog.test.tsx`

**Interfaces:**
- Produces:
  - Two-tab dialog: `"Write Note"` and `"Import Files / Host Path"`.
  - File picker `<input type="file" multiple>` + drag-and-drop zone.
  - Host path text input with `"Ingest Path"` button.
  - Feedback listing ingested files with links.

- [ ] **Step 1: Write failing test in `web/tests/QuickCaptureDialog.test.tsx`**
Test tab switching, file selection handling, host path submission, and error display.

- [ ] **Step 2: Run test to verify failure**
Run: `cd web && npm test tests/QuickCaptureDialog.test.tsx -- --run`
Expected: FAIL.

- [ ] **Step 3: Implement tabs and file/path ingestion in `QuickCaptureDialog.tsx`**
Add tab state, drag-and-drop listener, file reading via `client.captureNote` or `client.ingestPath`, and success feedback.

- [ ] **Step 4: Run test to verify pass**
Run: `cd web && npm test tests/QuickCaptureDialog.test.tsx -- --run`
Expected: PASS.

- [ ] **Step 5: Commit**
```bash
git add web/src/components/QuickCaptureDialog.tsx web/tests/QuickCaptureDialog.test.tsx web/src/lib/apiClient.ts
git commit -m "feat(web): add file picker, drag-and-drop and host path ingestion to QuickCapture"
```

---

### Task 8: WebFS Synchronization & Build

**Files:**
- Synchronize: `internal/webfs/dist/`

- [ ] **Step 1: Build web bundle and sync to `internal/webfs/dist/`**
Run:
```bash
cd web && npm ci && npm run lint && npm run typecheck && npm test -- --run && npm run build && cd ..
rm -rf internal/webfs/dist && cp -r web/dist internal/webfs/dist
```

- [ ] **Step 2: Commit**
```bash
git add internal/webfs/dist/
git commit -m "build(web): synchronize compiled webfs bundle with universal ingestion UI"
```

---

### Task 9: Terminal UI (`internal/tui/ingest_view.go`)

**Files:**
- Create: `internal/tui/ingest_view.go`
- Create: `internal/tui/ingest_view_test.go`
- Modify: `internal/tui/app.go`
- Modify: `internal/tui/keys.go`

**Interfaces:**
- Produces:
  - Tab `"9: 📥 Ingest"` (`ViewIngest`) in TUI.
  - File browser (`bubbles/filepicker`), target path prompt, and import history list.

- [ ] **Step 1: Write failing tests in `internal/tui/ingest_view_test.go`**
Test view creation, key navigation, directory selection, and execution trigger.

- [ ] **Step 2: Run tests to verify failure**
Run: `go test -v ./internal/tui/... -run TestIngestView`
Expected: FAIL.

- [ ] **Step 3: Implement `IngestView` and integrate in `AppModel`**
Implement `IngestView` using `filepicker.New()`, integrate into `ViewType` (`ViewIngest`), register tab in `AppModel.tabs` and `KeyMap.Tab9` (`9` or `i`).

- [ ] **Step 4: Run tests to verify pass**
Run: `go test -v ./internal/tui/...`
Expected: PASS.

- [ ] **Step 5: Commit**
```bash
git add internal/tui/
git commit -m "feat(tui): introduce interactive filepicker ingest view in tab 9"
```

---

### Task 10: End-to-End Verification & Quality Gates

**Files:**
- Documentation updates: `docs/cli/reference.md`, `docs/ambient-intelligence/mcp-tools.md`, `docs/tui/terminal-interface.md`.

- [ ] **Step 1: Update documentation**
Reflect generalized `daneel ingest [paths...]`, new MCP tools, and TUI tab 9.

- [ ] **Step 2: Run full verification suite**
Run:
```bash
go test -v -race ./...
cd web && npm test -- --run && cd ..
```

- [ ] **Step 3: Commit**
```bash
git add docs/
git commit -m "docs: update CLI reference, MCP tools and TUI docs for universal ingestion"
```
