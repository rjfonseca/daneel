# Harvester Pipeline

Daneel includes automated harvesting engines to ingest, sanitize, filter, prune, and index external technical documentation into the local Vault.

## 1. Supported Platforms

| Platform | Flag / Command | Target Output Directory | Auth Source |
|---|---|---|---|
| **GitLab** | `daneel harvest gl <group\|user>` | `sources/gitlab/<group|user>/` | `GITLAB_TOKEN` or `--token` |
| **GitHub** | `daneel harvest gh <org\|user>` | `sources/github/<org|user>/` | `GITHUB_TOKEN` or `--token` |
| **Google Drive** | `daneel harvest gd` | `sources/gdrive/` | `credentials.json` (OAuth) |
| **Jira** | `daneel harvest jr` | `sources/jira/` | `JIRA_API_TOKEN` |

Target types default to organizations/groups. To harvest personal user repositories, use the `--type user` flag:
```bash
daneel harvest gh <username> --type user
daneel harvest gl <username> --type user
```

Targets and filtering rules can also be pre-configured in `.daneel.yaml`:
```yaml
harvester:
  skip_archived: true # Default: true. Omits archived repositories on GitLab & GitHub
  ignored_directories: # Default toxic directories ignored immediately during scan
    - "node_modules"
    - "vendor"
    - "wp-content"
    - "dist"
    - "build"
    - "coverage"
    - ".git"
    - ".next"
  exclude_groups: # Glob patterns for groups/namespaces to skip completely
    - "marketing"
    - "github-images"
    - "*sandbox*"
  exclude_projects: # Glob patterns for project names or paths to skip completely
    - "*poc*"
    - "test-*"
    - "*-sandbox"
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
```

## 2. Ingestion & Sanitization Rules

1. **Early Skip on Remote APIs**: Archived or excluded groups/projects are skipped at the API listing level, eliminating tree traversal and network I/O.
2. **Toxic Directory Elimination**: Folders matching `ignored_directories` (such as `node_modules/` or `vendor/`) are blocked immediately by `ShouldHarvest`.
3. **Format Conversion**: Non-markdown sources (e.g. Google Docs) are converted to clean Markdown.
4. **Deterministic Frontmatter**: Preserves or generates metadata (`title`, `source_url`, `author`, `updated_at`, `scope: sources`).
5. **Link Normalization**: Transforms relative web URLs into Obsidian-compatible internal links where target documents exist locally.
6. **Automated Reindexing**: When `--reindex` is enabled (default `true`), any newly downloaded or updated documents trigger immediate SQLite FTS5 reindexing.

## 3. Pruning & Orphan Reconciliation

To prevent on-disk bloat and keep `sources/` aligned with remote reality, Daneel provides pruning via set difference ($\text{Orphans} = \text{Disk} - \text{Active}$):

```bash
# Preview what would be removed without moving files
daneel harvest prune --dry-run

# Prune orphaned, excluded, and toxic repositories across sources/
daneel harvest prune

# Prune without calling remote APIs (local exclusions and toxic directories only)
daneel harvest prune --remote=false

# Harvest configured targets and immediately prune orphaned/toxic files
daneel harvest --prune
```

When pruning executes:
- Toxic directories (e.g. `node_modules`, `vendor`, `wp-content`) inside any repository under `sources/` are moved to vault-local quarantine.
- Repositories that no longer exist remotely or match `exclude_groups` / `exclude_projects` are moved to vault-local quarantine.
- Recovery locations are printed and included as `restore_path` in JSON output. Empty parent directories may remain.
- Retained repository content is not modified by an implicit catalog reconciliation. Catalog maintenance is a separate explicit command.
- The SQLite FTS5 index is updated to purge deleted documents.

## 4. OKF Catalog MOC Reconciliation & Strict 3-Tier Topology

Rebuild group and repository Map of Content (MOC) notes across `sources/` at any time:
```bash
daneel harvest catalog
# Or:
daneel harvest --catalog
```

### Strict 3-Tier MOC Hierarchy
Daneel enforces a deterministic 3-tier MOC structure to prevent phantom stars and clutter in the Galaxy visualizer:
1. **Tier 1 (Organization Root):** `sources/gitlab/<org>/index.md` and `sources/github/<org>/index.md`.
2. **Tier 2 (Intermediate Groups & Subgroups):** `sources/gitlab/<org>/<group>/index.md`.
3. **Tier 3 (Repository Root):** `sources/gitlab/<org>/<group>/.../<repo>/index.md` is the **single canonical MOC** for the entire project.

> [!IMPORTANT]
> **Single MOC Per Repository Invariant**: A repository can have **only one** `index.md` located strictly at its root. The catalog reconciler never generates `index.md` files inside repository subdirectories (`docs/`, `plans/`, `specs/`, `superpowers/`, etc.) and automatically purges any spurious `index.md` or `_index.md` files found in repository subfolders.

Pruning retains local edits in `.daneel/quarantine/`. See [recovery instructions](../operations/recovery.md) before restoring or removing retained data.
