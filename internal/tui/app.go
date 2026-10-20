//nolint:revive // package comment not required
package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/rjfonseca/daneel/internal/config"
	"github.com/rjfonseca/daneel/internal/search"
)

// ViewType identifies which tab is currently active.
type ViewType int

const (
	ViewSearch ViewType = iota //nolint:revive
	ViewDashboard
	ViewWorkspace
	ViewPeople
	ViewRoutines
	ViewSkills
	ViewHooks
)

// AppModel is the root BubbleTea application model.
type AppModel struct {
	vaultRoot string
	engine    *search.Engine
	styles    Styles
	keyMap    KeyMap
	activeTab ViewType
	tabs      []string
	width     int
	height    int
	showHelp  bool
	quitting  bool
	statusMsg string

	// Child Views
	searchView    *SearchView
	dashboardView *DashboardView
	workspaceView *WorkspaceView
	peopleView    *PeopleView
	routinesView  *RoutinesView
	skillsView    *SkillsView
	hooksView     *HooksView
}

// NewAppModel constructs the root TUI application with all sub-views.
func NewAppModel(vaultRoot string, engine *search.Engine, tuiCfg config.TuiConfig) *AppModel {
	styles := NewStyles()
	keyMap := DefaultKeyMap()

	sv := NewSearchView(vaultRoot, engine, styles)
	dv := NewDashboardView(vaultRoot, styles)
	wv := NewWorkspaceView(vaultRoot, styles)
	pv := NewPeopleView(vaultRoot, styles)
	rv := NewRoutinesView(vaultRoot, styles)
	skv := NewSkillsView(vaultRoot, styles)
	hv := NewHooksView(vaultRoot, styles)

	return &AppModel{
		vaultRoot:     vaultRoot,
		engine:        engine,
		styles:        styles,
		keyMap:        keyMap,
		activeTab:     ViewSearch,
		tabs:          []string{tuiCfg.Tabs.Search, tuiCfg.Tabs.Dashboard, tuiCfg.Tabs.Workspace, tuiCfg.Tabs.People, tuiCfg.Tabs.Routines, tuiCfg.Tabs.Skills, "Agent Activity"},
		searchView:    sv,
		dashboardView: dv,
		workspaceView: wv,
		peopleView:    pv,
		routinesView:  rv,
		skillsView:    skv,
		hooksView:     hv,
	}
}

// Init initializes the application and all child views.
func (m *AppModel) Init() tea.Cmd {
	var cmds []tea.Cmd
	if m.searchView != nil {
		cmds = append(cmds, m.searchView.Init())
	}
	return tea.Batch(cmds...)
}

func (m *AppModel) isChildInInputMode() bool {
	if m.activeTab == ViewSearch && m.searchView != nil {
		return m.searchView.IsInputMode()
	}
	return false
}

// Update handles global events, tab switching, and delegates to active child view.
func (m *AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case TUICommandMsg:
		switch msg.Command {
		case "tui_switch_view":
			if viewName, ok := msg.Args["view_name"].(string); ok {
				switch viewName {
				case "search":
					m.activeTab = ViewSearch
				case "dashboard":
					m.activeTab = ViewDashboard
				case "workspace":
					m.activeTab = ViewWorkspace
				case "people":
					m.activeTab = ViewPeople
				case "dream":
					m.activeTab = ViewRoutines
				}
			}
			return m, nil

		case "tui_open_note":
			if filepath, ok := msg.Args["filepath"].(string); ok {
				m.activeTab = ViewSearch
				if m.searchView != nil {
					m.searchView.searchInput.SetValue(filepath)
					// Em vez de depender do FTS5 que pode não indexar a string exata do caminho, injetamos diretamente:
					m.searchView.results = []search.SearchResult{
						{
							RelPath: filepath,
							Title:   "📄 Note",
							Scope:   "direct",
						},
					}
					m.searchView.selectedIndex = 0
					m.searchView.focusRight = true
					m.searchView.updatePreview()
				}
			}
			return m, nil
		case "tui_highlight_lines":
			if startF, ok1 := msg.Args["start"].(float64); ok1 {
				if endF, ok2 := msg.Args["end"].(float64); ok2 {
					if m.searchView != nil {
						m.searchView.highlightStart = int(startF)
						m.searchView.highlightEnd = int(endF)
						m.searchView.updatePreview()
					}
				}
			}
			return m, nil

		case "tui_flash_notification":
			if message, ok := msg.Args["message"].(string); ok {
				m.statusMsg = message
				// Clear after 5 seconds
				return m, tea.Tick(5*time.Second, func(t time.Time) tea.Msg { //nolint:revive
					return struct{ clearNotification bool }{true}
				})
			}
			return m, nil

		case "tui_update_widget":
			if content, ok := msg.Args["content"].(string); ok {
				if m.dashboardView != nil {
					m.dashboardView.customWidgetText = content
				}
			}
			return m, nil
		}

	case TUIStateQueryMsg:
		state := map[string]interface{}{
			"active_tab": m.tabs[m.activeTab],
		}
		if m.activeTab == ViewSearch && m.searchView != nil {
			if len(m.searchView.results) > 0 && m.searchView.selectedIndex < len(m.searchView.results) {
				state["open_file"] = m.searchView.results[m.searchView.selectedIndex].RelPath
			}
		}
		msg.ReplyChan <- state
		return m, nil

	case struct{ clearNotification bool }:
		m.statusMsg = ""
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		contentHeight := msg.Height - 4
		if contentHeight < 8 {
			contentHeight = 8
		}

		if m.searchView != nil {
			m.searchView.SetSize(msg.Width, contentHeight)
		}
		if m.dashboardView != nil {
			m.dashboardView.SetSize(msg.Width, contentHeight)
		}
		if m.workspaceView != nil {
			m.workspaceView.SetSize(msg.Width, contentHeight)
		}
		if m.peopleView != nil {
			m.peopleView.SetSize(msg.Width, contentHeight)
		}
		if m.routinesView != nil {
			m.routinesView.SetSize(msg.Width, contentHeight)
		}
		if m.skillsView != nil {
			m.skillsView.SetSize(msg.Width, contentHeight)
		}
		if m.hooksView != nil {
			m.hooksView.SetSize(msg.Width, contentHeight)
		}
		return m, nil

	case tea.MouseMsg:
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft && msg.Y == 1 {
			// Calculate tab clicks
			x := 0
			for i, t := range m.tabs {
				tabLen := lipgloss.Width(m.styles.TabActive.Render(t)) + 1 // +1 for space between tabs
				if msg.X >= x && msg.X < x+tabLen {
					m.activeTab = ViewType(i)
					return m, nil
				}
				x += tabLen
			}
		}

	case tea.KeyMsg:
		// 1. Help Modal handling
		if m.showHelp {
			if msg.String() == "esc" || msg.String() == "?" || msg.String() == "q" {
				m.showHelp = false
			}
			return m, nil
		}

		// 2. Global hard interrupt
		if msg.String() == "ctrl+c" {
			m.quitting = true
			return m, tea.Quit
		}

		// 3. If child view is in text input mode (e.g. typing in search box):
		if m.isChildInInputMode() {
			var childCmd tea.Cmd
			m.searchView, childCmd = m.searchView.Update(msg)
			return m, childCmd
		}

		// 4. Normal Mode commands
		switch msg.String() {
		case "q":
			m.quitting = true
			return m, tea.Quit

		case "?":
			m.showHelp = true
			return m, nil

		case "tab":
			m.activeTab = (m.activeTab + 1) % ViewType(len(m.tabs))
			return m, nil

		case "shift+tab":
			if m.activeTab == 0 {
				m.activeTab = ViewType(len(m.tabs) - 1)
			} else {
				m.activeTab--
			}
			return m, nil

		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			idx := int(msg.String()[0] - '1')
			if idx >= 0 && idx < len(m.tabs) {
				m.activeTab = ViewType(idx)
			}
			return m, nil
		}
	}

	// Delegate to Active Child View in Normal Mode
	var childCmd tea.Cmd
	switch m.activeTab {
	case ViewSearch:
		if m.searchView != nil {
			m.searchView, childCmd = m.searchView.Update(msg)
			cmds = append(cmds, childCmd)
		}
	case ViewDashboard:
		if m.dashboardView != nil {
			m.dashboardView, childCmd = m.dashboardView.Update(msg)
			cmds = append(cmds, childCmd)
		}
	case ViewWorkspace:
		if m.workspaceView != nil {
			m.workspaceView, childCmd = m.workspaceView.Update(msg)
			cmds = append(cmds, childCmd)
		}
	case ViewPeople:
		if m.peopleView != nil {
			m.peopleView, childCmd = m.peopleView.Update(msg)
			cmds = append(cmds, childCmd)
		}
	case ViewRoutines:
		if m.routinesView != nil {
			m.routinesView, childCmd = m.routinesView.Update(msg)
			cmds = append(cmds, childCmd)
		}
	case ViewSkills:
		if m.skillsView != nil {
			m.skillsView, childCmd = m.skillsView.Update(msg)
			cmds = append(cmds, childCmd)
		}
	case ViewHooks:
		if m.hooksView != nil {
			m.hooksView, childCmd = m.hooksView.Update(msg)
			cmds = append(cmds, childCmd)
		}
	}

	return m, tea.Batch(cmds...)
}

// View renders the full TUI screen with Header, Tabs, Active View, and Status Bar.
func (m *AppModel) View() string {
	if m.quitting {
		return "Farewell, Partner. Second Brain synchronized under the Zeroth Law.\n"
	}

	// 1. Header Bar
	headerTitle := m.styles.HeaderTitle.Render("🧠 R. DANEEL OLIVAW [Positronic Knowledge Engine]")
	headerVault := m.styles.HeaderVault.Render(fmt.Sprintf("📂 Sector: %s", m.vaultRoot))
	header := lipgloss.JoinHorizontal(lipgloss.Center, headerTitle, headerVault)

	// 2. Tab Bar
	var tabButtons []string
	for i, t := range m.tabs {
		if ViewType(i) == m.activeTab {
			tabButtons = append(tabButtons, m.styles.TabActive.Render(t))
		} else {
			tabButtons = append(tabButtons, m.styles.TabInactive.Render(t))
		}
	}
	tabBar := lipgloss.JoinHorizontal(lipgloss.Top, strings.Join(tabButtons, " "))

	// 3. Active Screen Content
	var content string
	switch m.activeTab {
	case ViewSearch:
		if m.searchView != nil {
			content = m.searchView.View()
		}
	case ViewDashboard:
		if m.dashboardView != nil {
			content = m.dashboardView.View()
		}
	case ViewWorkspace:
		if m.workspaceView != nil {
			content = m.workspaceView.View()
		}
	case ViewPeople:
		if m.peopleView != nil {
			content = m.peopleView.View()
		}
	case ViewRoutines:
		if m.routinesView != nil {
			content = m.routinesView.View()
		}
	case ViewSkills:
		if m.skillsView != nil {
			content = m.skillsView.View()
		}
	case ViewHooks:
		if m.hooksView != nil {
			content = m.hooksView.View()
		}
	}

	// 4. Status / Help Bar
	statusBar := m.renderStatusBar()

	// 5. Compose Layout without extra blank lines to avoid terminal buffer overflow
	screen := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		tabBar,
		content,
		statusBar,
	)

	// 6. Help Modal Overlay if active
	if m.showHelp {
		return m.renderHelpOverlay(screen)
	}

	return screen
}

func (m *AppModel) renderStatusBar() string {
	var sb strings.Builder

	if m.statusMsg != "" {
		notifBadge := lipgloss.NewStyle().Background(m.styles.Theme.Warning).Foreground(m.styles.Theme.TextDark).Bold(true).Padding(0, 1).Render("🔔 " + m.statusMsg)
		sb.WriteString(notifBadge)
		sb.WriteString("  ")
	}

	if m.isChildInInputMode() {
		modeBadge := lipgloss.NewStyle().Background(m.styles.Theme.Primary).Foreground(m.styles.Theme.TextDark).Bold(true).Padding(0, 1).Render("POSITRONIC SEARCH")
		sb.WriteString(modeBadge)
		sb.WriteString("  ")
		sb.WriteString(m.styles.StatusKey.Render("Esc"))
		sb.WriteString(" ")
		sb.WriteString(m.styles.StatusDesc.Render("Normal Mode"))
		sb.WriteString("  •  ")
		sb.WriteString(m.styles.StatusKey.Render("Enter"))
		sb.WriteString(" ")
		sb.WriteString(m.styles.StatusDesc.Render("Commit & Navigate"))
		sb.WriteString("  •  ")
		sb.WriteString(m.styles.StatusKey.Render("Ctrl+S"))
		sb.WriteString(" ")
		sb.WriteString(m.styles.StatusDesc.Render("Scope"))
	} else {
		modeBadge := lipgloss.NewStyle().Background(m.styles.Theme.Success).Foreground(m.styles.Theme.TextDark).Bold(true).Padding(0, 1).Render("ZEROTH LAW ACTIVE")
		sb.WriteString(modeBadge)
		sb.WriteString("  ")

		keys := []struct {
			k string
			d string
		}{
			{"/", "Query"},
			{"1-5 / Tab", "Sectors"},
			{"j/k", "Scan"},
			{"h/l", "Pane"},
			{"Enter", "Inspect ($EDITOR)"},
			{"r", "Refresh"},
			{"q", "Terminate"},
			{"?", "Directives"},
		}

		for i, key := range keys {
			sb.WriteString(m.styles.StatusKey.Render(key.k))
			sb.WriteString(" ")
			sb.WriteString(m.styles.StatusDesc.Render(key.d))
			if i < len(keys)-1 {
				sb.WriteString(" • ")
			}
		}
	}

	return m.styles.StatusBar.Width(m.width).Render(sb.String())
}

func (m *AppModel) renderHelpOverlay(background string) string { //nolint:revive
	var helpSB strings.Builder
	helpSB.WriteString(m.styles.HelpTitle.Render("📖 R. Daneel Olivaw — Positronic Engine Directives") + "\n\n")

	shortcuts := [][]string{
		{"Tab / Shift+Tab", "Cycle through the 5 Galactic Sectors"},
		{"1 .. 5", "Jump to Sector (1: Seldon Index, 2: Radar, 3: Work, 4: Partners, 5: Dream)"},
		{"/", "Activate Positronic Query Mode (type search freely)"},
		{"Esc", "Deactivate Query Mode and return to Zeroth Law normal scan"},
		{"Alt+S / Ctrl+S", "Cycle knowledge scope (all, tech, daily, meta, sources)"},
		{"j / k (or ↑ / ↓)", "Scan items across the knowledge pathway (or scroll preview)"},
		{"h / l (or ← / →)", "Switch pane focus between Index List and Detail Viewport"},
		{"PgUp / PgDown", "Deep scroll preview viewport or fast skip in index"},
		{"Enter", "Inspect and edit note in $EDITOR (Neovim / Cursor / VSCode)"},
		{"F", "Filter repositories in Galactic Workspace (all, dirty, clean, go, rust, node)"},
		{"r", "Reindex synaptic pathways and refresh view"},
		{"d", "Trigger Hypnagogic REM Dream Cycle immediately"},
		{"?", "Display / dismiss this directives overlay"},
		{"q / Ctrl+C", "Terminate session and synchronize with Second Brain"},
	}

	for _, s := range shortcuts {
		fmt.Fprintf(&helpSB, "  %-18s %s\n",
			m.styles.HelpKey.Render(s[0]),
			m.styles.HelpDesc.Render(s[1]))
	}

	helpSB.WriteString("\n" + lipgloss.NewStyle().Foreground(m.styles.Theme.TextMuted).Render("Press 'esc', '?' or 'q' to close directives overlay."))

	modal := m.styles.HelpModal.
		Width(72).
		Render(helpSB.String())

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modal)
}
