# Native Agent Hooks & Ambient Telemetry

Daneel includes a native passive telemetry spooler and session compiler that captures AI agent activity (e.g. Claude Code, Antigravity, Cursor) and integrates it into the Second Brain graph, terminal dashboard, and nocturnal Dream Cycle.

---

## 1. Architecture Overview

```
                      +-------------------+
                      |   AI Agent Run    |
                      | (Claude Code/AGY) |
                      +---------+---------+
                                |
                                | stdio / hooks
                                v
                    daneel hook capture <event>
                                |
                 +--------------+--------------+
                 | (UDS Socket probe < 2ms)     |
                 v                             v
        [ Daemon Online ]             [ Daemon Offline ]
                 |                             |
      IPC via daneel.sock                      |
                 |                             |
     +-----------+-----------+                 |
     |                       |                 |
     v                       v                 v
Spool Writer          Hub Event Broadcast  Spool Writer
(session.jsonl)       (TUI & Web HUD)      (session.jsonl)
     |                                         |
     +-------------------+---------------------+
                         |
                         v (on SessionEnd or daneel hook compile)
              Raw-to-Inbox Compiler
                         |
                         v
          inbox/sessions/YYYY-MM-DD-HH-MM-<agent>.md
                         |
                         v (Overnight Consolidation)
               daneel dream (Phase 3 Triage)
```

### Key Technical Properties:
- **Zero Firewall / Port Overhead**: Communicates over Daneel's Unix Domain Socket (`~/.local/state/daneel/daneel.sock`).
- **Resilient Offline Fallback**: If the daemon is not running or does not reply promptly, events are appended directly to `~/.cache/daneel/spool/<session_id>.jsonl`.
- **RAG Trust Sandbox**: Transcripts and executed bash commands are compiled inside `<untrusted_content>` tags to prevent untrusted agent outputs from poisoning semantic search.

Native hook execution uses bounded work to fit within agent hook limits. Hook IPC gets a 200ms budget for connecting and requesting a daemon response. After a SessionEnd event is persisted, automatic session compilation gets a separate 500ms budget. These budgets bound normal local work; they cannot guarantee hard real-time behavior during arbitrary filesystem or kernel stalls.

If capture succeeds but automatic compilation is deferred or fails, the raw session remains available in the spool and the hook reports the retry command on stderr. Compile it later with `daneel hook compile --session <id>` (or `daneel hook compile --all`). The compiled summary is unavailable until that command succeeds. Do not hide hook errors with shell `|| true`; capture failures must remain visible to the agent.

---

## 2. Enabling & Installation

### Option A: Automatic CLI Setup (Recommended)

Daneel provides a one-shot automated installer that detects and safely patches your agent configuration files:

```bash
# Preview what will be modified
daneel hook install --dry-run

# Install hooks for all supported agents (Claude Code, Antigravity, Codex)
daneel hook install

# Or install for a specific agent
daneel hook install --agent claude-code
daneel hook install --agent antigravity
daneel hook install --agent codex
```

To revert changes at any time:
```bash
daneel hook uninstall
```

---

### Option B: Manual Configuration

#### 1. Claude Code
Add the hooks snippet to `~/.claude/settings.json`:

```json
{
  "hooks": {
    "SessionStart": [
      {
        "matcher": "",
        "command": "daneel hook handoff"
      }
    ],
    "PostToolUse": [
      {
        "matcher": "",
        "command": "daneel hook capture PostToolUse --agent claude-code"
      }
    ],
    "SessionEnd": [
      {
        "matcher": "",
        "command": "daneel hook capture SessionEnd --agent claude-code"
      }
    ]
  }
}
```

#### 2. Antigravity CLI
Add to `~/.gemini/antigravity-cli/hooks.json` or your project hooks configuration:

```json
{
  "hooks": {
    "SessionStart": "daneel hook handoff",
    "PostToolUse": "daneel hook capture ToolUse --agent antigravity",
    "SessionEnd": "daneel hook capture SessionEnd --agent antigravity"
  }
}
```

#### 3. OpenAI Codex CLI
Add to `~/.codex/hooks.json` (or install via `daneel hook install --agent codex`):

```json
{
  "hooks": {
    "SessionStart": [
      {
        "hooks": [
          { "type": "command", "command": "daneel hook handoff" }
        ]
      }
    ],
    "PostToolUse": [
      {
        "matcher": "*",
        "hooks": [
          { "type": "command", "command": "daneel hook capture PostToolUse --agent codex" }
        ]
      }
    ],
    "SessionEnd": [
      {
        "hooks": [
          { "type": "command", "command": "daneel hook capture SessionEnd --agent codex", "timeout": 3 }
        ]
      }
    ]
  }
}
```

Codex requires arrays of matcher groups and command handlers, as documented in
the [official hooks guide](https://developers.openai.com/codex/hooks). If an older
configuration uses command strings directly under event names or the `ToolUse`
event, rerun `daneel hook install --agent codex` to migrate it to this format.
The Daneel SessionEnd command has an explicit three-second timeout. SessionEnd
is synchronous; an async setting does not change that behavior.

---

## 3. Extending to Custom & Future Agents

Daneel was architected from day one to be **completely agent-agnostic**:

### 1. Arbitrary Agent Identifiers
The `daneel hook capture` command accepts any string in `--agent`:
```bash
echo '{"command": "pytest"}' | daneel hook capture ToolUse --agent custom-bot
```
The session compiler and the Dream Cycle automatically group, index, and triage sessions under `<vaultRoot>/inbox/sessions/YYYY-MM-DD-HH-MM-<agent>.md`.

### 2. Universal Shell Wrapper
For CLI agents or scripts that don't have native JSON hook configurations (e.g. Aider, Cursor CLI, Python scripts), wrap them with `assets/hooks/universal-agent-wrapper.sh`:

```bash
./assets/hooks/universal-agent-wrapper.sh codex codex "Refactor database engine"
./assets/hooks/universal-agent-wrapper.sh aider aider --model gpt-4o
./assets/hooks/universal-agent-wrapper.sh custom-agent python run_agent.py
```

### 3. Pluggable Go Adapter Registry (`AgentHookAdapter`)
In Go code, new agent configurations can be plugged into `daneel hook install` by implementing:

```go
type AgentHookAdapter interface {
    Name() string
    Aliases() []string
    Install(dryRun bool) (string, error)
    Uninstall(dryRun bool) (string, error)
}
```
And registering with `cmd.RegisterAgentHookAdapter(adapter)`.

---

## 4. CLI Command Suite

| Command | Description |
|---|---|
| `daneel hook capture <event>` | Consumes JSON payload via standard input and records to spool. |
| `daneel hook handoff` | Inspects recent sessions matching current working directory and outputs brief continuity context for the agent. |
| `daneel hook status` | Displays daemon connectivity, active spool files, and uncompiled session counts. |
| `daneel hook compile [--session <id> \| --all]` | Compiles active spool `.jsonl` files into `inbox/sessions/` markdown notes. |
| `daneel hook install` | Automatically registers hooks into agent config files. |
| `daneel hook uninstall` | Removes Daneel hooks from agent configurations. |

---

## 4. Observability & Ambient Telemetry

- **Terminal UI (`daneel tui`)**:
  - Press `Tab` or `H` to open the **Agent Activity** monitor.
  - Live timeline showing current sessions, tools executed, and spool volume.
- **Web Galaxy Visualizer (`daneel serve`)**:
  - Features the **Hooks** satellite on the Seldon Perimeter ($r \approx 6500$).
  - Displays the **Agent Presence HUD Card** in the upper-right corner whenever an active agent is executing tools.
- **Dream Cycle Ingestion (`daneel dream`)**:
  - Triages compiled session logs from `inbox/sessions/`.
  - Automatically feeds duration, modified files, and executed commands into the hypnagogic dream consolidation prompt.
