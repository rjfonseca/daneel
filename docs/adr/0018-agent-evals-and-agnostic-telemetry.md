# ADR 0018: Agent Evals and Agnostic Telemetry for Missed Opportunities and Knowledge Gaps

* Status: accepted
* Date: 2026-09-18

## Context and Problem Statement

Autonomous AI coding agents and IDEs (such as Antigravity/AGY, Claude Code, and Cursor) operate extensively over local codebases and the Daneel Second Brain. However, they frequently suffer from cognitive laziness or context starvation, resulting in unnecessary human interruptions:
1. **Interrogating the user for technical details**: The agent halts its execution loop to ask the human engineer questions like *"Where is the payment gateway configured?"*, *"What is the authentication flow for the identity service?"*, or *"Which port does Redis use in staging?"*.
2. **Failure to exploit search tooling**: Even when equipped with Daneel Model Context Protocol (MCP) tools (`search_vault`, `resolve_note`, `get_note_content`), agents frequently fail to invoke them proactively before interrupting the engineer.

To optimize human-agent synergy and keep the Second Brain authoritative, Daneel requires a telemetry evaluation system to analyze recent agent sessions and categorize false negatives into two actionable problem archetypes:
1. **Agent Failure (Missed Opportunity)**: The required technical knowledge is already documented within the Vault, but the agent chose to interrupt the user instead of querying `search_vault` or `resolve_note`.
2. **Vault Failure (Knowledge Gap)**: The agent asked a technical question about a subject that should reside in the Vault, but no matching documentation exists. This signals a need for a manual note or an automated repository Harvester.

Feeding hundreds of raw megabytes of daily session transcripts directly to a large language model (LLM) is economically non-viable and slow. A lightweight, privacy-preserving, local-first evaluation architecture is required.

## Decision Drivers

* **Zero-Token Native Go Filtering**: Filter out non-suspicious interactions deterministically on local CPU in sub-second time without incurring LLM API token costs.
* **Agnostic Multi-Agent Log Parsing**: Seamlessly parse heterogeneous local transcript formats across Antigravity (`~/.gemini/antigravity-cli/brain/`), Claude Code (`~/.claude/`), and Cursor (`~/.config/Cursor/User/workspaceStorage/` SQLite `state.vscdb` or JSON logs).
* **Deterministic Tool Negation Check**: Automatically discard interactions where the agent already exercised vault discovery tools (`search_vault`, `resolve_note`, `search_projects`, etc.).
* **FTS5 Knowledge Validation**: Validate extracted technical doubts against the local SQLite FTS5 search index and note resolution engine in sub-5ms to distinguish between Missed Opportunities and Knowledge Gaps.
* **Dual Interface Parity**: Expose the evaluator symmetrically through both the pure Go CLI (`daneel audit agents`) and the MCP stdio JSON-RPC server (`audit_agent_missed_opportunities`).
* **Continuous Feedback Loop**: Persist eval findings into `meta/guias/aprendizados-agente.md` and synchronize metrics in `meta/dashboard-metricas.md` when `--apply` is invoked.

## Considered Options

* **Option 1: Full Transcript Cloud LLM Pipeline**: Stream entire agent transcripts directly to a remote LLM for judgment.
  * *Disadvantages*: Extreme token costs (hundreds of thousands of tokens per day); high latency; leaks entire conversational transcripts to external network endpoints.
* **Option 2: Pure Regex Heuristics Without Vault Cross-Referencing**: Flag all agent questions as failures without checking if the Vault contains the answer.
  * *Disadvantages*: Cannot differentiate between an Agent Autonomy Failure (Missed Opportunity) and a Vault Coverage Deficit (Knowledge Gap); produces uncalibrated metrics.
* **Option 3: Hybrid Two-Phase Architecture (Native Go Deterministic Filtering + SQLite FTS5 Engine Validation)**:
  * Phase 1 executes native Go regexes and tool negation checks to isolate the "Suspicious Delta".
  * Phase 2 extracts substantive technical keywords from the delta and executes silent full-text queries against SQLite FTS5 and entity resolution.
  * Categorizes results into `MissedOpportunity` (note exists) vs. `KnowledgeGap` (no note exists) with targeted remediation actions.

## Decision Outcome

Chosen option: **Option 3: Hybrid Two-Phase Architecture**.

### Architectural Details

1. **Agnostic Log Scanners (`internal/auditor/scanners.go`)**:
   - `AgentLogScanner` interface defines `Name() string` and `ScanRecent(since time.Duration) ([]Interaction, error)`.
   - `AntigravityScanner`: Parses JSONL transcripts from `.system_generated/logs/transcript.jsonl`, pairing `USER_INPUT` (from `USER_EXPLICIT`) with `PLANNER_RESPONSE` and extracted `tool_calls`.
   - `ClaudeCodeScanner`: Traverses `~/.claude/` for `.jsonl` session files, extracting user prompts, assistant replies, and `tool_use` blocks.
   - `CursorScanner`: Inspects `~/.config/Cursor/User/workspaceStorage/` SQLite `state.vscdb` (`ItemTable` keys matching `%chatdata%` or `%composerData%`) and `.json`/`.jsonl` transcripts.
   - `MultiScanner`: Aggregates all registered scanners and filters by `--agent` flag (`all`, `antigravity`, `claude-code`, `cursor`).

2. **Phase 1: Deterministic Filtering (Suspicious Delta)**:
   - Evaluates interactions across three filters:
     1. **Tool Negation Check**: Discards any interaction where `HasVaultSearchTool(turn.ToolsUsed)` is true.
     2. **Interrogation Pattern Check**: Filters for agent replies containing question marks (`?`) or interrogation starters (`qual`, `como`, `onde`, `what`, `how`, `where`, `can you provide`).
     3. **Substantive User Prompt Check**: Requires prompt length $\ge 5$ characters.
   - Unmatched interactions are discarded immediately without computational overhead.

3. **Phase 2: Classification Engine (`internal/auditor/evals.go`)**:
   - Strips conversational preambles and stopwords (Portuguese & English) to extract key substantive query terms (`ExtractSubstantiveTerms`).
   - Executes silent faceted search via `engine.SearchFaceted(ctx, search.SearchOptions{Query: substantiveQuery, Limit: 3})` and `engine.ResolveNote(ctx, query)`.
   - Verifies genuine token overlap in matching note title, rel_path, or snippet:
     - **Match Found** $\rightarrow$ **`ClassificationMissedOpportunity`**: Information exists in Vault note(s). Action: Refine agent system prompt to mandate proactive search.
     - **No Match Found** $\rightarrow$ **`ClassificationKnowledgeGap`**: Information is absent from Vault. Action: Document missing domain knowledge or create a new Harvester.

4. **CLI & MCP Server Exposure**:
   - Pure Go CLI: `daneel audit agents [--since 48h] [--agent all|antigravity|claude-code|cursor] [--apply] [--dry-run] [--json]`.
   - MCP Tool: `audit_agent_missed_opportunities` with parameters `since`, `agent`, `apply`.

5. **Closed Cognitive Loop Integration**:
   - When `--apply` is specified, discoveries are persisted via `dream.RecordFeedback` to `meta/guias/aprendizados-agente.md` and updated in `meta/dashboard-metricas.md` under section `## 🤖 3. Telemetria de Agentes & Evals (Missed Opportunities)`.

## Consequences

### Positive Consequences
* Zero LLM token consumption for Phase 1 filtering and local SQLite FTS5 Phase 2 verification.
* Instant (< 50ms) evaluation of dozens of session transcripts across multiple agent frameworks.
* Clear operational distinction between agent instruction deficits (System Prompt) and Second Brain knowledge gaps (Documentation / Harvesters).
* Automated closed-loop telemetry updates to the central dashboard.

### Negative Consequences / Trade-offs
* Format shifts in third-party agent log files (e.g. Cursor internal SQLite schema updates) may require periodic parser maintenance in `internal/auditor/scanners.go`.
