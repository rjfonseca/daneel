# CLI Command Reference

Daneel provides a comprehensive command-line suite for second-brain management, search, and autonomous workflows.

---

## 1. Global Flags

Available across all commands:
- `--vault-path <path>`: Explicit path to the Vault root directory.
- `--config <path>`: Custom path to `.daneel.yaml`.
- `-g, --global`: Forces global configuration (`~/.config/daneel/config.yaml`), ignoring local directory files.
- `--json`: Formats output as machine-readable JSON.
- `-v, --verbose`: Enables verbose debug logging.

---

## 2. Command Catalog

### Visualizer, Daemon & MCP
| Command | Summary | Key Flags / Subcommands |
|---|---|---|
| `daneel serve` | Starts the Web Galaxy visualizer & ConnectRPC server | `-p, --port` (default: 4321, with auto-fallback up to 4340) |
| `daneel daemon` | Controls the background Unix socket daemon & unified multi-listener | `run [--serve --port]`, `start`, `stop`, `status`, `service install [--with-serve --with-routines]`, `service uninstall [--with-routines]`, `service status` |
| `daneel hook` | Silent agent lifecycle telemetry, session compiler & hook installer | `capture <event>`, `handoff`, `status`, `compile [--session <id> \| --all]`, `install [--agent <name>]`, `uninstall` |
| `daneel routine` | Manages scheduled maintenance routines (systemd timers) | `list [--json]`, `install [--all]`, `uninstall [--all]`, `enable`, `disable`, `run`, `logs [-n -f]`, `status` |
| `daneel mcp` | Starts the stdio JSON-RPC Model Context Protocol server | None |
| `daneel tui` | Launches the interactive terminal user interface | None |

### Search & Knowledge Navigation
| Command | Summary | Key Flags |
|---|---|---|
| `daneel search` | Sub-3ms full-text search with BM25 ranking and faceted metadata filters | `[query]`, `-s, --scope`, `-t, --type` (e.g. `rfc`, `adr`), `--tag`, `-l, --limit`, `-r, --reindex` |
| `daneel resolve` | Fuzzy resolves any note title, slug, or alias to canonical path in < 2ms | `<query>`, `--json` |
| `daneel people` | Discovers colleagues and profiles with compound name and alias matching | `[query]`, `--squad`, `--role`, `--email`, `--json` |
| `daneel projects` | Sub-3ms search across local Git repositories in `~/Work` via SQLite cache | `[query]`, `--dirty`, `--clean`, `--stack`, `-l, --limit`, `--json` |
| `daneel read` | Renders a formatted technical note with syntax highlighting | Path argument, e.g. `notes/ai/agents.md` |
| `daneel backlinks` | Lists all notes linking to a specific document | Path argument, e.g. `notes/systems/gateway.md` |
| `daneel index` | Synchronizes notes into SQLite FTS5 database | `-f, --force` |

### Hygiene, Security & Cognitive Consolidation
| Command | Summary | Key Flags |
|---|---|---|
| `daneel tidy` | Deterministic vault hygiene: link fixes, MOC updates, live reload | `--dry-run`, `--no-agent` |
| `daneel guard` | Audits public notes against Asimov's Four Laws of Robotics | `--check`, `--file <path>`, `--json` |
| `daneel dream` | Runs the cognitive REM cycle (hygiene, triage, telemetry, radar, metacognition) | `--date`, `--no-agent`, `--stats`, `--metacognition`, `--synthesize`, `--prompt`, `--interactive`, `--profile`, `--json`, `--learn` |
| `daneel audit sessions` | Audits agent session transcripts for corrections, skills, and evals | `--since`, `--mine-skills`, `--evals`, `--apply`, `--dry-run` |
| `daneel audit agents` | Audits agnostic agent sessions for missed opportunities and knowledge gaps | `--since`, `--agent <all\|antigravity\|claude-code\|cursor>`, `--apply`, `--dry-run`, `--json` |
| `daneel harvest` | Scrapes documentation from GitHub, GitLab, Jira, GDrive | `[catalog]`, `[prune]`, `--prune`, `--dry-run`, `-l, --limit`, `-r, --reindex`, `-t, --token`, `--type <org\|user>` |
| `daneel doctor` | Diagnostics for vault health, broken links, and metadata | None |

### Operations, Telemetry & Copilots
| Command | Summary | Key Flags |
|---|---|---|
| `daneel dossier` | Compiles executive performance review dossier and impact report | `--year`, `--semester`, `-o, --output` |
| `daneel prep` | Generates an executive briefing for 1:1s and alignments | `<person-slug>` |
| `daneel log` | Displays activity logs, time tracking, rolling weekly matrix, and monthly trends | `--date <today\|YYYY-MM-DD>`, `--recent <days>`, `--weeks [N]`, `--monthly`, `--json` |
| `daneel inbox` | Lists and inspects pending items in `inbox/` | None |
| `daneel ingest` | Batch ingests Google Meet / Gemini meeting transcripts | `--zip <path>`, `--dry-run` |
| `daneel workspace` | Scans local development directory (`~/Work`), prunes merged/pushed branches and worktrees, resets stale feature branches, and updates MOC | `--sync-clean`, `--inactivity-threshold <48h>`, `--write`, `-o` |
| `daneel clone` | Clones a repository, inspects stack, and creates a system note | `--dream`, `--fetch` |
| `daneel chat` | Silent pre-indexing (<5ms) and exec passthrough to AI agents | `-p, --profile` (`agy`, `claude`), prompt string |

### Setup & Utility
| Command | Summary | Key Flags |
|---|---|---|
| `daneel init` | Scaffolds a new vault with folders, skills, and templates | `--local <path>`, `--global <path>`, `--non-interactive` |
| `daneel config` | Displays active config and sets the global vault path | `set-vault <path>` |
| `daneel version` | Displays version, build metadata, and positronic core lore | None |
| `daneel completion` | Generates shell autocomplete scripts (bash, zsh, fish) | Shell argument |
