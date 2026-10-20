# Daneel Native Agent Hooks Integration

This directory contains integration templates for wiring AI agents (Claude Code, Antigravity CLI, Cursor) into Daneel's passive capture engine and ambient telemetry.

## Architecture

Hooks invoke `daneel hook capture <event>` directly:
- **Daemon Online**: Sends event over Daneel's Unix socket (`~/.local/state/daneel/daneel.sock`), publishing ambient events to the Web Galaxy and TUI.
- **Daemon Offline**: Automatically falls back to appending directly into `~/.cache/daneel/spool/<session_id>.jsonl` with zero dropped telemetry and zero agent lag (< 3ms).

## 1. Claude Code Setup

Merge the snippet from [`claude-settings-snippet.json`](claude-settings-snippet.json) into `~/.claude/settings.json`:

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

## 2. Antigravity CLI Setup

In Antigravity hook configurations (`~/.gemini/antigravity-cli/hooks.json` or project hooks):

```json
{
  "hooks": {
    "SessionStart": "daneel hook handoff",
    "PostToolUse": "daneel hook capture ToolUse --agent antigravity",
    "SessionEnd": "daneel hook capture SessionEnd --agent antigravity"
  }
}
```

## 3. OpenAI Codex CLI Setup

Install automatically via:
```bash
daneel hook install --agent codex
```

Or manually configure `~/.codex/hooks.json`:

```json
{
  "hooks": {
    "SessionStart": "daneel hook handoff",
    "ToolUse": "daneel hook capture ToolUse --agent codex",
    "SessionEnd": "daneel hook capture SessionEnd --agent codex"
  }
}
```

## 4. Universal Wrapper for Any Arbitrary Agent

For agents without native hook configurations (e.g. Aider, Cursor CLI, custom scripts), use [`universal-agent-wrapper.sh`](universal-agent-wrapper.sh):

```bash
# Wrap any CLI execution:
./universal-agent-wrapper.sh codex codex "Implement feature X"
./universal-agent-wrapper.sh aider aider --model gpt-4o
./universal-agent-wrapper.sh my-agent python run.py
```

## 5. Inspection & Status

Run `daneel hook status` at any time to inspect active spool files, daemon connectivity, and uncompiled event counts.
In the TUI (`daneel tui`), press tab to navigate to the **Agent Activity** monitor.
