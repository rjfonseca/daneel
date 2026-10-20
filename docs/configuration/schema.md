# Configuration Specification & Schema Reference

Daneel operates on a flexible, layered configuration system adhering to XDG Base Directory specifications and supporting both multi-vault setups and global developer environments.

---

## 1. Resolution Hierarchy

Configuration is resolved in order of precedence:

1. **CLI Flags**: `--vault-path`, `--config`, `-g / --global`.
2. **Environment Variables**: `DANEEL_VAULT_PATH`, `DANEEL_CONFIG`.
3. **Local Vault Config**: `.daneel.yaml` at the root of the active vault or repository.
4. **Global System Config**: `~/.config/daneel/config.yaml` (or `$XDG_CONFIG_HOME/daneel/config.yaml`).

---

## 2. Complete `.daneel.yaml` Schema

```yaml
# 1. Vault Identity & Knowledge Hierarchy
vault:
  title: "Engineering Second Brain"
  owner: "Jane Doe"
  role: "Staff Software Engineer"
  squad: "Core Platform"
  indexed_dirs:
    - notes
    - sources
    - meta
    - daily
    - inbox
    - public/content

# 2. Local Code Workspace & Repository Cloner
workspace:
  work_dir: "~/Work"
  default_git_host: "github.com"
  branch_inactivity: "48h" # Duration before resetting clean feature branches to main/master (e.g. 48h, 2d)


# 3. Public Export & Security Guardrails (daneel guard)
publishing:
  content_dir: "public/content"
  block:
    domains:
      - "internal.corp"
      - "jira.internal"
    keywords:
      - "confidential"
      - "secret-project"

# 4. Daily Time Telemetry & Leadership Archetypes
telemetry:
  taxonomy:
    - id: "A_Strategic"
      name: "Strategic & Architecture"
      description: "Vision, RFCs, long-term technical roadmap"
      keywords: ["rfc", "architecture", "vision", "strategy", "platform"]
      target_min: 30.0
    - id: "B_LocalImpact"
      name: "Local Impact & Execution"
      description: "Bug fixes, PRs, refactoring, SDKs"
      keywords: ["fix", "bug", "sdk", "pr", "refactor"]
      target_min: 30.0
    - id: "C_GlueWork"
      name: "Glue Work & Alignment"
      description: "1:1s, mentorship, code reviews, hiring"
      keywords: ["1:1", "mentorship", "alignment", "interview"]
      target_max: 30.0
    - id: "E_Overhead"
      name: "Overhead & Ceremonies"
      description: "All-hands, administrative meetings, email"
      keywords: ["all-hands", "administrative", "slack", "email"]
      target_max: 10.0

# 5. Strategic Radar & Friction Signals (Dream Cycle)
radar:
  - name: "Cloud & Infrastructure"
    type: "business" # business | tech | friction | people
    description: "Cloud migration and Gateway API architecture"
    keywords: ["kubernetes", "cloud", "gateway", "service-mesh"]
  - name: "AI Platform"
    type: "business"
    description: "Autonomous agents and MCP copilots"
    keywords: ["agents", "mcp", "daneel", "llm"]
  - name: "Technical Debt & Friction"
    type: "friction"
    description: "Early detection of operational blockers and CVEs"
    keywords: ["incident", "bug", "cve", "blocker", "slowdown"]

# 6. Terminal User Interface (TUI) Layout
tui:
  tabs:
    search: "1: 🔍 Search"
    dashboard: "2: 📊 Dashboard"
    workspace: "3: 🗺️ Workspace"
    people: "4: 🤝 People"
    routines: "5: 🌙 Routines"
    skills: "6: ⚡ Skills"

# 7. Semantic Inbox Routing Rules
dream:
  auto_fix: true
  routing:
    "notes/ai": ["agent", "mcp", "rag", "prompt", "llm"]
    "notes/systems": ["api", "service", "gateway", "worker", "sdk"]
    "notes/processes": ["rfc", "prd", "sla", "postmortem"]
    "notes/career": ["goals", "feedback", "1:1", "growth"]

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

# 9. Technical Documentation Harvesting
harvester:
  skip_archived: true
  ignored_directories:
    - "node_modules"
    - "vendor"
    - "wp-content"
    - "dist"
    - "build"
    - "coverage"
    - ".git"
    - ".next"
  exclude_groups:
    - "marketing"
    - "github-images"
  exclude_projects:
    - "*poc*"
    - "test-*"
  match_directories: ["docs", "doc", "adrs"]
  match_filenames: ["README.md", "catalog-info.yaml"]
  match_extensions: [".md", ".org"]
  targets:
    - platform: "github"
      org: "my-organization"
    - platform: "github"
      user: "octocat"
      type: "user"
    - platform: "gitlab"
      group: "my-group"
    - platform: "gitlab"
      user: "my-user"
      type: "user"

# 10. Seldon Vault Anchors & Temporal Hotbar
hotbar:
  weekly_pattern: "weekly/{{year}}-W{{week}}.md" # Custom pattern supporting {{year}}, {{week}} (01-53), {{weekNum}}
  pinned_anchors:
    - path: "notes/projects/core-platform.md"
      label: "Core Platform"
      icon: "⚡"
    - path: "notes/career/staff-charter.md"
      label: "Staff Charter"
      icon: "🎯"
```

---

## 3. Managing Global Vaults

Inspect the active configuration and XDG directories:

```bash
daneel config
```

Set the global vault path:

```bash
daneel config set-vault ~/Documents/second-brain
```
