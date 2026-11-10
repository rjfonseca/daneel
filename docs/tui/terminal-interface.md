# Terminal User Interface (TUI) Reference

Daneel provides a full-featured terminal workspace powered by [Bubble Tea](https://github.com/charmbracelet/bubbletea) and [Lip Gloss](https://github.com/charmbracelet/lipgloss).

Launch the TUI with:

```bash
daneel tui
```

---

## 1. Views and Tabs

| Key | Tab | Description |
|---|---|---|
| `1` | **🔍 Search** | Sub-millisecond FTS5 search with BM25 ranking, scope filters, and Glamour markdown reading |
| `2` | **📊 Dashboard** | Operational overview: time telemetry by archetype, active backlog tasks, and recent vault activity |
| `3` | **🗺️ Workspace** | Overview of local git repositories in `~/Work`, branch states, and quick-links |
| `4` | **🤝 People** | Network directory: profiles, interaction histories, and instant 1:1 briefing prep |
| `5` | **🌙 Routines** | Status of background routines: Dream Cycle, tracking reconciliation, strategic radar |
| `6` | **⚡ Skills** | Catalog of operational skills (repo cloner, backlog hygiene, dossier compiler) |
| `7` | **🤖 Agent Activity** | Real-time passive telemetry spooler and background agent lifecycle hooks |
| `8` / `T` | **📈 Telemetry** | FinOps metrics, model token consumption, and latency distributions |
| `9` / `i` | **📥 Ingest** | Interactive dual-pane file browser and universal note ingestion engine |

---

## 2. Global Keybindings

| Keybinding | Action |
|---|---|
| `Tab` / `Shift+Tab` | Cycle forwards or backwards through tabs |
| `1` - `9` | Jump directly to tab 1 through 9 |
| `i` | Jump directly to Tab 9 (**📥 Ingest**) |
| `T` | Jump directly to Tab 8 (**📈 Telemetry**) |
| `j` / `k` (or `↓` / `↑`) | Navigate items up and down in lists and viewports |
| `Enter` | Open selected note or trigger action |
| `/` | Focus search bar in the Search tab |
| `f` | Enter **Vimium Follow Mode** to jump to inline links |
| `Esc` | Clear search or exit current sub-view |
| `q` / `Ctrl+C` | Exit Daneel TUI |

---

## 3. Reading Notes in the TUI

Pressing `Enter` on any search result renders the note using [Glamour](https://github.com/charmbracelet/glamour) with full syntax highlighting, table formatting, and blockquote rendering. Press `Esc` to return to search results.

---

## 4. Universal Ingestion View (Tab 9: 📥 Ingest)

The Ingest tab provides an interactive terminal workspace for exploring the host filesystem and indexing markdown documents directly into your Second Brain.

### Dual-Pane Layout
- **Left Pane (File Browser)**: Interactive filesystem browser powered by Bubble Tea's `filepicker`. Highlights compatible `.md`, `.markdown`, and `.txt` files while dimming unsupported extensions.
- **Right Pane (Ingest Controls, Preview & History)**:
  - **Target Vault Destination**: Displays the active destination folder within the vault (defaults to `inbox/`).
  - **Status & Notification Banner**: Real-time feedback on ingestion operations or file selection.
  - **Ingested Note Preview**: Displays the top lines of the most recently ingested document.
  - **Session Ingestion History**: Chronological log of all files ingested during the active TUI session with timestamps and status indicators (`🟢 Ingested`, `🟡 Suffix collision`, `❌ Error`).

### Ingest Tab Keybindings
| Keybinding | Action |
|---|---|
| `Enter` | Ingest currently selected file, or enter highlighted directory |
| `d` / `D` | Recursively ingest all supported files in the current directory |
| `t` | Edit target vault destination directory (type path, press `Enter` to confirm, `Esc` to cancel) |
| `Tab` | Switch focus between File Browser and Details panel |
| `Esc` / `h` | Navigate up to parent directory in File Browser |
| `j` / `k` (or `↓` / `↑`) | Navigate items in the file browser |

