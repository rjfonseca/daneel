# Architecture Overview

Daneel is architected as a modular, local-first system designed around a single source of truth: a Markdown Knowledge Vault structured with Obsidian-compatible Wikilinks and metadata frontmatter.

```mermaid
graph TD
    subgraph Data & Storage
        Vault[Markdown Vault: notes/, sources/, meta/]
        DB[(SQLite FTS5 + note_links Cache)]
    end

    subgraph Core Runtime
        Daemon[Unix Socket Daemon: ~/.local/state/daneel/daneel.sock]
        Hub[Thread-Safe Hub & Event Bus]
        Daemon --> Hub
        Hub --> DB
    end

    subgraph Interfaces
        Web[Web Galaxy: React + Sigma.js + WebGL]
        TUI[Terminal UI: BubbleTea]
        CLI[CLI Commands: harvest, tidy, index, daemon]
        MCP[MCP Server: Stdio JSON-RPC]
    end

    subgraph External
        Assistants[AI Assistants: Antigravity, Claude, Cursor]
        Sources[External Sources: GitHub, GitLab, GDrive, Jira]
    end

    Vault --> DB
    Sources -->|Harvesters| Vault
    Assistants <-->|MCP Protocol| MCP
    MCP <-->|Unix Socket| Daemon
    CLI <-->|RunOrDelegate| Daemon
    TUI <-->|Active Context| Daemon
    Web <-->|ConnectRPC Stream & Context| Daemon

## Open Knowledge Framework (OKF) & Repository Anchors

External documentation ingested into `sources/` (from GitLab, GitHub, Jira, etc.) is anchored via standard OKF Maps of Content (`index.md`):
- **Repository MOC (`type: repository`)**: Resides at the root of each repository (e.g. `sources/gitlab/acme/edge/gateway/index.md`). Catalogs internal specifications, ADRs, and guides, and acts as the gravitational Alpha Star for that project's Star Cluster.
- **Group MOC (`type: group`)**: Resides at intermediate organizational namespaces (e.g. `sources/gitlab/acme/edge/index.md`), linking sub-groups and repositories.
- **Local Reconciliation**: Run `daneel harvest catalog` to automatically generate or repair MOCs across all existing harvested repositories without network downloads.

