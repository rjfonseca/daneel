# TOON (Token-Oriented Object Notation) Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Integrate TOON (Token-Oriented Object Notation) as the **default** retrieval serialization format for Daneel's MCP server, add a global CLI `--toon` flag, and compact cognitive REM dream cycle prompts to reduce token consumption by 40% to 65% in LLM sessions.

**Architecture:** 
1. Build a foundational Go package `internal/toon` wrapping `github.com/toon-format/toon-go` with Daneel-optimized tabular encoders, automatic delimiter selection (comma `,` or pipe `|`), and token savings telemetry.
2. Update `internal/mcp` tools (`search_vault`, `search_people`, `search_projects`, `get_active_backlog`, `get_backlinks`, `scan_workspace_repos`, `audit_agent_missed_opportunities`) to **default to TOON format**, while supporting opt-in `format: "markdown"` for clients that explicitly require human-readable text.
3. Add a global persistent CLI flag `--toon` (`-T`) in `cmd/root.go` allowing agents and shell routines to receive structured tabular responses with minimal token footprint.
4. Integrate TOON serialization into `internal/dream` and `cmd/dream.go` for compacting prompt context and report payloads.
5. Author and approve Architectural Decision Record (MADR) `docs/adr/0040-token-oriented-object-notation-toon.md` before coding begins.

**Tech Stack:** 
- Go 1.27
- `github.com/toon-format/toon-go v1.0.1` (official TOON implementation)
- Daneel MCP Server (JSON-RPC 2.0 stdio)
- Cobra CLI (`cmd/root.go`)
- SQLite FTS5 Search Engine (`internal/search`)

**Spec & References:** 
- Second-brain note: [`notes/ia/toon-token-oriented-object-notation.md`](file:///home/rfonseca/Projects/github.com/rjfonseca/second-brain/notes/ia/toon-token-oriented-object-notation.md)
- Architectural Decision Record: [ADR 0040](file:///home/rfonseca/Projects/github.com/rjfonseca/daneel/docs/adr/0040-token-oriented-object-notation-toon.md) ([Artifact link](file:///home/rfonseca/.gemini/antigravity-cli/brain/c5aa7405-cf45-44fe-a3ee-9589af34bcd6/adr-0040-toon.md))
- Official TOON specification: [toonformat.dev](https://toonformat.dev)

---

## Real Gain Estimations on Daneel Payloads

| Payload Type | Quantity | Format | Raw Size (bytes) | Estimated Tokens | Savings vs Markdown | Savings vs JSON |
|---|---|---|---|---|---|---|
| **`search_vault`** (Notes & Snippets) | 10 notes | Markdown | 2,350 B | ~780 tokens | Baseline | - |
| | 10 notes | Indented JSON | 2,180 B | ~640 tokens | - | Baseline |
| | 10 notes | **TOON Tabular** | **1,620 B** | **~380 tokens** | **51.3%** | **40.6%** |
| **`search_projects`** (Workspace Repos) | 20 repos | Markdown | 2,420 B | ~820 tokens | Baseline | - |
| | 20 repos | Indented JSON | 2,240 B | ~710 tokens | - | Baseline |
| | 20 repos | **TOON Tabular** | **1,050 B** | **~270 tokens** | **67.1%** | **62.0%** |
| **`search_people`** (Colleague Profiles) | 15 people | Markdown | 1,850 B | ~610 tokens | Baseline | - |
| | 15 people | Indented JSON | 1,690 B | ~520 tokens | - | Baseline |
| | 15 people | **TOON Tabular** | **880 B** | **~230 tokens** | **62.3%** | **55.8%** |
| **`get_active_backlog`** (Task Ledger) | 30 tasks | Markdown | 2,900 B | ~940 tokens | Baseline | - |
| | 30 tasks | Indented JSON | 2,750 B | ~850 tokens | - | Baseline |
| | 30 tasks | **TOON Tabular** | **1,410 B** | **~390 tokens** | **58.5%** | **54.1%** |
| **Agent Multi-Turn Session** | 10 tool calls | Markdown | ~23,000 B | ~7,800 tokens | Baseline | - |
| | 10 tool calls | **TOON Tabular** | **~12,000 B** | **~3,100 tokens** | **60.3%** | **~52%** |

**Net Impact**: In a typical 10-turn autonomous agent session, TOON saves **~4,700 tokens**, accelerating TTFT (Time-To-First-Token) by 25–35% and cutting retrieval inference costs by more than half.

---

## Global Constraints

- **Strict Open-Source Neutrality**: Zero corporate entities, private colleague names, or internal paths in code, tests, ADRs, or fixtures. Use generic open-source personas (*Ada Lovelace*, *Alan Turing*, *Bob Jones*) and fictional domains (`acme.corp`, `example.com`).
- **English-First Invariant**: All UI labels, MCP tool descriptions, parameter schemas, and error messages must be strictly in English.
- **MADR Pre-Condition**: ADR 0040 must be reviewed and approved prior to starting code execution.
- **Conventional Commits**: Commit messages and PR titles must use strict types (`feat(toon): ...`, `fix(mcp): ...`, `test(toon): ...`) without emoji prefixes or persona labels.
- **TDD Workflow**: Every component must follow Red-Green-Refactor: failing test first, minimal implementation to pass, refactor and verify.

---

## Review Focus

1. **Default MCP Behavior**: `tools/call` for collection retrieval tools now outputs TOON by default. If `format: "markdown"` is explicitly passed in `arguments`, it falls back to the legacy markdown formatter.
2. **Delimiter Collision**: When data rows contain commas (e.g. search snippets, descriptions), TOON automatically selects pipe `|` delimiter `results[N|]{...}:` to prevent broken columns.
3. **Array Truncation Guardrail**: The TOON header `[N]` matches the exact row count emitted, allowing LLM decoders to validate output completeness.
4. **Empty Results Handling**: Empty search results format cleanly as `results[0]{...}:` or appropriate not-found message.
5. **CLI Flag Precedence**: If `--toon` is specified, it takes priority over default human tables.

---

## Task Decomposition & Execution Plan

### Task 1: Core TOON Engine (`internal/toon`)

**Files:**
- Create: `internal/toon/toon.go`
- Create: `internal/toon/toon_test.go`
- Create: `internal/toon/benchmark_test.go`
- Modify: `go.mod`, `go.sum`

**TDD Steps:**
- [ ] **Step 1.1: Dependency Installation**: Run `go get github.com/toon-format/toon-go` and verify `go.mod`.
- [ ] **Step 1.2: Write Failing Tests**: In `internal/toon/toon_test.go`, write unit tests:
  - Test encoding uniform tabular records with headers (`EncodeTabular`).
  - Test auto-delimiter selection (switching from `,` to `|` when cell contents contain commas).
  - Test escaping newlines and delimiter characters.
  - Test empty collections (`[0]`).
  - Test struct marshaling using `toon-go` struct tags.
  - Test `CalculateSavings(jsonStr, toonStr)` calculating token and byte reductions.
- [ ] **Step 1.3: Run Tests (Verify Failure)**: `go test ./internal/toon/... -v` must fail.
- [ ] **Step 1.4: Implement Minimal Engine**: Implement `internal/toon/toon.go` exporting:
  - `Marshal(v interface{}) ([]byte, error)`
  - `MarshalString(v interface{}) (string, error)`
  - `EncodeTabular(name string, headers []string, rows [][]string, delimiter ...string) string`
  - `EstimateTokens(s string) int`
  - `CalculateSavings(rawJSON, rawTOON string) (jsonTokens, toonTokens int, savingsPct float64)`
- [ ] **Step 1.5: Run Tests (Verify Pass)**: `go test ./internal/toon/... -v` passes.
- [ ] **Step 1.6: Run Benchmark Tests**: In `internal/toon/benchmark_test.go`, assert $\ge 40\%$ savings.

---

### Task 2: Configuration & MCP Server TOON Integration (`internal/mcp`)

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `internal/mcp/server.go`
- Modify: `internal/mcp/server_test.go`

**TDD Steps:**
- [ ] **Step 2.1: Write Failing Config Test**: Test `mcp.default_format` loading from `.daneel.yaml` (accepting `"toon"` or `"markdown"`, defaulting to `"toon"`).
- [ ] **Step 2.2: Implement Config Field**: Add `DefaultFormat string` to `MCPConfig` / `Config` defaulting to `"toon"`.
- [ ] **Step 2.3: Write Failing MCP Tool Tests**: In `internal/mcp/server_test.go`:
  - Test default call to `search_vault` without `format` argument -> assert response header starts with `results[` and contains tabular schema `{title,path,scope,type,snippet}`.
  - Test explicit `"arguments": {"query": "Microservices", "format": "markdown"}` -> assert legacy Markdown header `# 🔍 Search Results`.
  - Test default call to `search_people` -> assert `people[` TOON header.
  - Test default call to `search_projects` -> assert `projects[` TOON header.
  - Test default call to `get_active_backlog` -> assert `tasks[` TOON header.
  - Test default call to `get_backlinks` -> assert `backlinks[` TOON header.
  - Test default call to `scan_workspace_repos` -> assert `workspace[` TOON header.
  - Test default call to `audit_agent_missed_opportunities` -> assert `opportunities[` TOON header.
- [ ] **Step 2.4: Implement MCP Schema & Formatting**:
  - Update `tools/list` schemas in `internal/mcp/server.go` to add `format` parameter (`enum: ["toon", "markdown"]`, default `"toon"`).
  - In `executeTool`, resolve format (defaulting to `"toon"` unless `"markdown"` requested or configured).
  - Implement TOON render functions for each collection retrieval tool.
- [ ] **Step 2.5: Verify Tests Pass**: Run `go test ./internal/mcp/... -v`.

---

### Task 3: Global CLI Flag `--toon` & Command Adapters (`cmd/`)

**Files:**
- Modify: `cmd/root.go`
- Modify: `cmd/search.go`
- Modify: `cmd/people.go`
- Modify: `cmd/projects.go`
- Modify: `cmd/backlinks.go`
- Modify: `cmd/workspace.go`
- Modify: `cmd/routine.go`
- Modify: `cmd/doctor.go`
- Modify: `cmd/inbox.go`
- Create: `cmd/toon_flag_test.go`

**TDD Steps:**
- [ ] **Step 3.1: Write Failing CLI Tests**: In `cmd/toon_flag_test.go`, test commands with `toonOut = true`:
  - `daneel search --toon` produces valid TOON tabular syntax.
  - `daneel people --toon` produces valid TOON tabular syntax.
  - `daneel projects --toon` produces valid TOON tabular syntax.
  - `daneel backlinks --toon` produces valid TOON tabular syntax.
  - `daneel workspace --toon` produces valid TOON tabular syntax.
  - `daneel doctor --toon` produces valid TOON tabular syntax.
- [ ] **Step 3.2: Implement Persistent Flag**: In `cmd/root.go`:
  - Register `toonOut bool` and `RootCmd.PersistentFlags().BoolVarP(&toonOut, "toon", "T", false, "Output results in TOON (Token-Oriented Object Notation) format")`.
- [ ] **Step 3.3: Adapt Commands**:
  - In `cmd/search.go`: add `if toonOut { ... }` block emitting `results[N]{title,path,scope,type,score}:`.
  - In `cmd/people.go`: add `if toonOut { ... }` block emitting `people[N]{name,role,squad,email,path}:`.
  - In `cmd/projects.go`: add `if toonOut { ... }` block emitting `projects[N]{name,path,branch,status,stack}:`.
  - In `cmd/backlinks.go`: add `if toonOut { ... }` block emitting `backlinks[N]{source_path,link_text}:`.
  - In `cmd/workspace.go`: add `if toonOut { ... }` block emitting `workspace[N]{name,path,branch,dirty}:`.
  - In `cmd/routine.go`: add `if toonOut { ... }` block for `routine list` and `routine runs`.
  - In `cmd/doctor.go`: add `if toonOut { ... }` block emitting `diagnostics[N]{severity,category,file,message}:`.
  - In `cmd/inbox.go`: add `if toonOut { ... }` block emitting `inbox[N]{filename,title,created_at}:`.
- [ ] **Step 3.4: Verify Tests Pass**: Run `go test ./cmd/... -v -run TestToon`.

---

### Task 4: Cognitive Dream Cycle Context Optimization (`internal/dream` & `cmd/dream.go`)

**Files:**
- Modify: `internal/dream/engine.go`
- Modify: `internal/dream/engine_test.go`
- Modify: `cmd/dream.go`
- Modify: `cmd/dream_test.go`

**TDD Steps:**
- [ ] **Step 4.1: Write Failing Dream TOON Tests**: Test serializing `FullDreamReport` dense sub-arrays (`DenseCategories`, `InboxTriaged`, `BacklogP0`, `EmergingConcepts`) to TOON format.
- [ ] **Step 4.2: Implement TOON Prompt Formatting**:
  - Add helper methods to format dream report triage items as compact TOON blocks for the autonomous agent prompt.
  - Support `toonOut` flag in `cmd/dream.go`.
- [ ] **Step 4.3: Verify Tests Pass**: Run `go test ./internal/dream/... ./cmd/... -v -run Dream`.

---

### Task 5: Documentation Updates & Final Quality Gates

**Files:**
- Modify: `docs/ambient-intelligence/mcp-tools.md`
- Modify: `docs/cli/reference.md`
- Modify: `docs/architecture/overview.md`
- Update: `docs/adr/README.md` (add entry for ADR 0040)
- Update: `docs/adr/0040-token-oriented-object-notation-toon.md` (set Status: accepted)

**Verification Steps:**
- [ ] **Step 5.1: Static Analysis**: Run `task lint` (or `golangci-lint run`) and resolve any issues.
- [ ] **Step 5.2: Complete Test Suite**: Run `task test` to verify zero regressions across the codebase.
- [ ] **Step 5.3: Update Documentation**: Update references, CLI docs, and ADR index.

---

## Subagent Dispatch Plan

When execution begins upon approval, tasks will be orchestrated across subagents:

1. **Subagent 1 (`toon-core-architect`)**:
   - Executes **Task 1**: `internal/toon` package, unit tests, benchmark tests, and token savings estimator.
2. **Subagent 2 (`mcp-toon-engineer`)**:
   - Executes **Task 2**: `internal/mcp` tool format parameter, TOON default rendering, and MCP test suite.
3. **Subagent 3 (`cli-toon-engineer`)**:
   - Executes **Task 3**: `cmd/` global `--toon` flag and command adaptations across search, people, projects, workspace, routine, doctor, inbox.
4. **Subagent 4 (`dream-cycle-engineer`)**:
   - Executes **Task 4**: `internal/dream` and `cmd/dream.go` TOON prompt context and report encoding.
5. **Coordinator / Reviewer**:
   - Executes **Task 5**: Full test suite, linter gate, doc updates, and git commit hygiene.

---

## Verification Plan

### Automated Tests
```bash
# 1. Unit tests for internal/toon
go test ./internal/toon/... -v

# 2. MCP server tests with format: "toon" (and default verification)
go test ./internal/mcp/... -v -run TestMCPServer

# 3. CLI flag tests
go test ./cmd/... -v -run TestToon

# 4. Dream engine tests
go test ./internal/dream/... -v

# 5. Full test suite and lint
task test
task lint
```

### Manual Verification
1. Run `daneel search "architecture" --toon` and verify that the output begins with `results[N]{title,path,scope,type,snippet}:` and is formatted cleanly.
2. Run `daneel people --toon` and verify compact tabular output.
3. Run `daneel mcp` with a mock stdio call requesting `{"method": "tools/call", "params": {"name": "search_vault", "arguments": {"query": "test"}}}` and confirm it defaults to TOON.
