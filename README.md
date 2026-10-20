# 🧠 Daneel (`daneel`)

<div align="center">

[![CI](https://github.com/rjfonseca/daneel/actions/workflows/ci.yml/badge.svg)](https://github.com/rjfonseca/daneel/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/github/go-mod/go-version/rjfonseca/daneel)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![MCP Compatible](https://img.shields.io/badge/MCP-Compatible-blue)](https://modelcontextprotocol.io/)

**The AI-First Engineering Second Brain CLI & Model Context Protocol (MCP) Server in Pure Go.**

*Inspired by R. Daneel Olivaw, the enduring cognitive guardian created by Isaac Asimov.*

</div>

---

## 💡 Overview

**Daneel** is a high-performance CLI tool and native Model Context Protocol (MCP) server for software engineers, engineering leaders, and autonomous AI coding agents (Claude Code, Cursor, Antigravity, Gemini, Windsurf, Neovim).

It combines **local full-text retrieval with SQLite FTS5/BM25**, **autonomous memory consolidation (Dream Cycle)**, **team & career copilots (1:1 prep, AVD dossier, time tracking)**, and **security guardrails for public publishing**.

---

## ⚡ Highlights

* **Zero CGO / Pure Go:** 100% static compilation without external C runtime dependencies (`modernc.org/sqlite`).
* **Ranked Full-Text Search:** SQLite FTS5 with BM25 relevance ranking, context snippets, and scoped filtering (`tech`, `daily`, `meta`, `inbox`, `sources`, `public`).
* **Native MCP Server (`daneel mcp`):** Standard `stdio` JSON-RPC protocol ready to plug into any AI coding assistant or IDE.
* **XDG & Multi-Vault Compliant:** Seamlessly operates across local isolated repositories (`.daneel.yaml`) and system-wide global vaults (`~/.config/daneel/config.yaml`).
* **Open Source & Language Agnostic:** Fully English-compliant codebase, parameterized configurations, and zero hardcoded proprietary context.
* **Agentic OS & ARMS Framework:** Fully aligned with the Agentic Operating System vision, offering integrated visual dashboards, *Routines*, and *Skills* catalog directly in the TUI.
* **Vimium-Style Link Hopping:** Seamlessly navigate between markdown notes inside the TUI by pressing `f` to trigger follow mode.
* **Multi-Target Harvester:** Ingest and synchronize tickets, issues, and MRs from Jira, GitLab, and GitHub natively into your Vault.
* **Cognitive Dream Cycle:** Autonomous nightly REM-like consolidation that repairs broken links, synchronizes Maps of Content (MOCs), and tracks staff engineering telemetry.
* **Privacy & Secret Guardrails:** Audits exported markdown notes before publishing to ensure zero PII, internal tokens, or broken links.

---

## 🚀 Quickstart & Installation

### Option 1: Using `go install`

Requires Go 1.27 or newer. Pre-built binaries do not require Go or Node.js.

```bash
go install github.com/rjfonseca/daneel@latest
```

### Option 2: Pre-built Binaries

Download the latest release tarball for your OS/Architecture from [GitHub Releases](https://github.com/rjfonseca/daneel/releases).

```bash
# Example for Linux (x86_64)
tar -xzf daneel_*_linux_amd64.tar.gz
sudo mv daneel /usr/local/bin/
```

### Option 3: From Source (with Task & mise)

```bash
git clone https://github.com/rjfonseca/daneel.git
cd daneel
task build
task install # installs to ~/.local/bin/daneel
```

---

### First vault

```bash
daneel init --local --yes ./my-vault
cd my-vault
# Add a Markdown note under notes/, then search it:
daneel search "system design"
daneel serve
```

The Web interface is local-only. Open the URL printed by `serve`. For a
reproducible example, follow the [demonstration vault guide](docs/development/demo-vault.md).
See [compatibility guarantees](docs/releases/compatibility.md),
[recovery](docs/operations/recovery.md), and the [release checklist](docs/releases/checklist.md).

Search uses lexical matching and BM25 ranking, without embeddings. Query latency
depends on the vault, hardware and cache state; run `task bench` to measure it.

## 🤖 AI Agent & MCP Integration

Daneel exposes a native MCP server with discovery through `tools/list` for AI assistants.

### 1. Claude Desktop

Add Daneel to your `claude_desktop_config.json`:

* **macOS:** `~/Library/Application Support/Claude/claude_desktop_config.json`
* **Linux:** `~/.config/Claude/claude_desktop_config.json`
* **Windows:** `%APPDATA%\Claude\claude_desktop_config.json`

```json
{
  "mcpServers": {
    "daneel": {
      "command": "daneel",
      "args": ["mcp"]
    }
  }
}
```

### 2. Cursor IDE

In Cursor, open **Settings > Features > MCP Servers** and add:

* **Name:** `daneel`
* **Type:** `command`
* **Command:** `daneel mcp`

### 3. Antigravity / Claude Code / Gemini CLI

In your MCP configuration file (`~/.gemini/antigravity-cli/mcp.json` or `.mcp.json`):

```json
{
  "mcpServers": {
    "daneel": {
      "command": "daneel",
      "args": ["mcp"]
    }
  }
}
```

---

## 📖 CLI Commands & Usage

```bash
# Initialize a new vault (Local or Global)
daneel init --local ./my-vault
daneel init --global ~/Documents/second-brain

# Ultra-fast FTS5 / BM25 Search
daneel search "microservices architecture"
daneel search "database replication" --scope tech --limit 5

# Interactive Terminal UI (Charm Bubble Tea & Glamour)
daneel tui

# Web Galaxy 3D Visualizer & ConnectRPC Server
daneel serve
daneel serve --port 4321

# Unified Background Daemon & IPC Socket
daneel daemon start
daneel daemon status

# Silent Native Agent Hooks & Cross-Agent Continuity
daneel hook install                     # Auto-configure hooks for Claude, Antigravity, Codex
daneel hook handoff                     # Emit active context for seamless session handoff
daneel hook status                      # Inspect spool buffer metrics and active sessions
daneel hook compile --all               # Compile raw sessions into vault inbox

# Scheduled Maintenance Routines (systemd timers & execution ledger)
daneel routine list
daneel routine run dream

# Render markdown notes with syntax highlighting
daneel read "notes/arquitetura/hexagonal.md"

# 1:1 Preparation & Meeting Briefing
daneel prep "jane-doe"

# Staff Engineering Performance Review Executive Dossier
daneel dossier --year 2026

# Staff Engineering Time Tracking, Weekly Matrix & Historical Trends
daneel log --date today
daneel log --recent 7
daneel log --weeks 4 --monthly

# Local Workspace Scanner & Branch Inactivity Pruning
daneel workspace
daneel workspace --sync-clean --inactivity-threshold 48h

# Deterministic Vault Hygiene & Synchronization (< 50ms)
daneel tidy             # Converts WikiLinks, syncs MOCs, people & workspace
daneel tidy --no-fix    # Dry-run audit

# Synaptic Graph & OKF Diagnostics (Linting)
daneel doctor           # Audits frontmatter OKF compliance and broken links

# Autonomous End-to-End Cognitive REM Dream Cycle (AI Agent Driven)
daneel dream            # Runs pre-flight hygiene, invokes AI agent, and syncs Git
daneel dream && shutdown now  # Overnight consolidation and automatic poweroff
daneel dream --no-agent # Runs pre-flight hygiene without invoking AI agent

# Cognitive Repository Clone & Vault Onboarding
daneel clone git@github.com:charmbracelet/lipgloss.git
daneel clone acme/knowledge-base --dream

# Public Export Security & PII Audit
daneel guard --check
daneel guard --file public/content/my-article.md

# Start MCP Server for AI Agents
daneel mcp

# Launch AI Agent Chat in Vault Directory (with fast pre-indexing)
daneel chat
daneel chat -p claude
daneel chat "what is the status of the Gateway API migration?"
```

---

## ⚙️ Semantic Configuration (`.daneel.yaml`)

Daneel features a rich, self-documenting configuration schema organized in 7 lifecycle domains:

```yaml
# 1. Vault Identity & Knowledge Categories
vault:
  title: "Engineering Second Brain"
  owner: "Jane Doe"
  role: "Staff Software Engineer"
  squad: "Platform Core"
  indexed_dirs:
    - notes
    - sources
    - meta
    - daily
    - inbox
    - public/content

# 2. Local Code Workspace & Git Defaults
workspace:
  work_dir: "~/Work"
  default_git_host: "github.com"

# 3. Public Exporting & Privacy Firewall (daneel guard)
publishing:
  content_dir: "public/content"
  block:
    domains:
      - "internal.corp"
      - "jira.corp"
    keywords:
      - "secret-project"
      - "confidential"

# 4. Daily Log Tracking & Staff Eng Taxonomy
telemetry:
  taxonomy:
    - id: "A_Estrategico"
      name: "Strategic & Architecture"
      description: "Vision, RFCs, long-term roadmap"
      keywords: ["rfc", "architecture", "vision", "strategy"]
    - id: "B_ImpactoLocal"
      name: "Local Impact & Execution"
      description: "Bug fixes, PRs, refactoring, SDKs"
      keywords: ["fix", "bug", "sdk", "pr", "refactor"]

# 5. Strategic Radar & Tech Debt Tracking (Dream Cycle)
radar:
  - name: "AI Platform"
    type: "business" # business | tech | friction | people
    description: "Autonomous agents and MCP copilots"
    keywords: ["agent", "mcp", "daneel", "llm platform"]
  - name: "Technical Debt & Friction"
    type: "friction"
    description: "Early detection of operational blockers"
    keywords: ["incident", "bug", "cve", "blocker", "slowdown"]

# 6. TUI Layout & Agnostic Parameterization
tui:
  tabs:
    search: "1: 🔍 Search"
    dashboard: "2: 📊 Dashboard"
    workspace: "3: 🗺️ Workspace"
    people: "4: 🤝 People"
    routines: "5: 🌙 Routines"
    skills: "6: ⚡ Skills"

# 7. Zettelkasten Routing & Auto-Triaging
dream:
  auto_fix: true
  routing:
    "notes/ia": ["agent", "mcp", "rag", "prompt", "llm"]
    "notes/sistemas": ["api", "service", "gateway", "worker", "sdk"]

# 8. AI Agent Profiles & Terminal Passthrough (daneel chat)
chat:
  default: "agy"
  profiles:
    agy:
      command: "agy"
      env:
        TERM: "xterm-256color"
    claude:
      command: "claude"
    codex:
      command: "codex"
```

---

## 🛠️ Contributing

Contributions are welcome! Please read our [CONTRIBUTING.md](CONTRIBUTING.md) guide and [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) before submitting Pull Requests.

```bash
# Run tests & linters
task fmt
task lint
task test
task bench
```

---

## 📄 License

This project is licensed under the [MIT License](LICENSE).
