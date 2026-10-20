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

---

## 2. Global Keybindings

| Keybinding | Action |
|---|---|
| `Tab` / `Shift+Tab` | Cycle forwards or backwards through tabs |
| `1` - `6` | Jump directly to tab 1 through 6 |
| `j` / `k` (or `↓` / `↑`) | Navigate items up and down in lists and viewports |
| `Enter` | Open selected note or trigger action |
| `/` | Focus search bar in the Search tab |
| `f` | Enter **Vimium Follow Mode** to jump to inline links |
| `Esc` | Clear search or exit current sub-view |
| `q` / `Ctrl+C` | Exit Daneel TUI |

---

## 3. Reading Notes in the TUI

Pressing `Enter` on any search result renders the note using [Glamour](https://github.com/charmbracelet/glamour) with full syntax highlighting, table formatting, and blockquote rendering. Press `Esc` to return to search results.
