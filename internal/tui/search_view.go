//nolint:revive // package comment not required
package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"

	"github.com/rjfonseca/daneel/internal/search"
)

// SearchView coordinates split-pane search and rich markdown preview.
type SearchView struct {
	vaultRoot      string
	engine         *search.Engine
	styles         Styles
	searchInput    textinput.Model
	viewport       viewport.Model
	results        []search.SearchResult
	selectedIndex  int
	selectedScope  string
	scopes         []string
	scopeIndex     int
	width          int
	height         int
	focusRight     bool
	inputMode      bool
	highlightStart int
	highlightEnd   int
	followMode     bool
	hintInput      string
	linkHints      map[string]string
}

// NewSearchView creates a new search view.
func NewSearchView(vaultRoot string, engine *search.Engine, styles Styles) *SearchView {
	ti := textinput.New()
	ti.Placeholder = "Type '/' to search, 'j/k' to navigate..."
	ti.CharLimit = 100
	ti.Width = 35
	ti.Prompt = "🔍 "
	ti.PromptStyle = lipgloss.NewStyle().Foreground(styles.Theme.Primary).Bold(true)
	ti.Blur() // Start in Normal Navigation mode

	vp := viewport.New(60, 20)

	sv := &SearchView{
		vaultRoot:     vaultRoot,
		engine:        engine,
		styles:        styles,
		searchInput:   ti,
		viewport:      vp,
		scopes:        []string{"all", "tech", "daily", "meta", "sources"},
		scopeIndex:    0,
		selectedScope: "all",
		results:       []search.SearchResult{},
		selectedIndex: 0,
		inputMode:     false,
	}

	sv.performSearch()
	return sv
}

// Init initializes the view.
func (v *SearchView) Init() tea.Cmd {
	return nil
}

// IsInputMode returns true if the search text input is currently focused.
func (v *SearchView) IsInputMode() bool {
	return v.inputMode
}

// Update handles tea messages for the search view.
func (v *SearchView) Update(msg tea.Msg) (*SearchView, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// 1. When in Search / Insert Mode
		if v.inputMode {
			switch msg.String() {
			case "esc":
				// Exit search mode to normal mode
				v.inputMode = false
				v.searchInput.Blur()
				return v, nil
			case "enter":
				// Commit query and return to normal mode for j/k navigation
				v.inputMode = false
				v.searchInput.Blur()
				return v, nil
			case "ctrl+s", "alt+s":
				v.scopeIndex = (v.scopeIndex + 1) % len(v.scopes)
				v.selectedScope = v.scopes[v.scopeIndex]
				v.performSearch()
				return v, nil
			}

			// Pass all other keystrokes (letters, numbers like '2', symbols, 'q', 'j', 'k') to text input
			prevVal := v.searchInput.Value()
			var tiCmd tea.Cmd
			v.searchInput, tiCmd = v.searchInput.Update(msg)
			cmds = append(cmds, tiCmd)

			if v.searchInput.Value() != prevVal {
				v.selectedIndex = 0
				v.performSearch()
			}
			return v, tea.Batch(cmds...)
		}

		// 2. When in Follow Mode (Link Hopping)
		if v.followMode {
			switch msg.String() {
			case "esc":
				v.followMode = false
				v.hintInput = ""
				v.updatePreview()
				return v, nil
			default:
				// Append valid characters
				if len(msg.String()) == 1 && msg.String() >= "a" && msg.String() <= "z" {
					v.hintInput += msg.String()
					// Check if exact match
					if targetPath, ok := v.linkHints[v.hintInput]; ok {
						v.followMode = false
						v.hintInput = ""
						// Emit a command to open this file inside the same search view
						return v, func() tea.Msg {
							return TUICommandMsg{
								Command: "tui_open_note",
								Args:    map[string]interface{}{"filepath": targetPath},
							}
						}
					}
					// Check if partial match exists
					hasPartial := false
					for k := range v.linkHints {
						if strings.HasPrefix(k, v.hintInput) {
							hasPartial = true
							break
						}
					}
					if !hasPartial {
						// Invalid hint typed, reset
						v.hintInput = ""
					}
				}
				return v, nil
			}
		}

		// 3. When in Normal Mode
		switch msg.String() {
		case "f":
			if len(v.results) > 0 && v.selectedIndex < len(v.results) {
				v.followMode = true
				v.hintInput = ""
				v.updatePreview()
			}
			return v, nil

		case "/", "i", "s":
			// Enter Search Mode!
			v.inputMode = true
			v.searchInput.Focus()
			return v, textinput.Blink

		case "ctrl+s", "alt+s":
			v.scopeIndex = (v.scopeIndex + 1) % len(v.scopes)
			v.selectedScope = v.scopes[v.scopeIndex]
			v.performSearch()
			return v, nil

		case "left", "h":
			v.focusRight = false
			return v, nil

		case "right", "l":
			v.focusRight = true
			return v, nil

		case "enter", "e":
			// Open selected note in editor
			if len(v.results) > 0 && v.selectedIndex < len(v.results) {
				res := v.results[v.selectedIndex]
				absPath := filepath.Join(v.vaultRoot, res.RelPath)
				editor := os.Getenv("EDITOR")
				if editor == "" {
					editor = "nvim"
				}
				cmd := exec.Command(editor, absPath)                     //nolint:gosec // intentional
				return v, tea.ExecProcess(cmd, func(err error) tea.Msg { //nolint:revive
					return nil
				})
			}
			return v, nil

		case "up", "k":
			if v.focusRight {
				v.viewport.ScrollUp(1)
				return v, nil
			}
			if v.selectedIndex > 0 {
				v.selectedIndex--
				v.updatePreview()
			}
			return v, nil

		case "down", "j":
			if v.focusRight {
				v.viewport.ScrollDown(1)
				return v, nil
			}
			if v.selectedIndex < len(v.results)-1 {
				v.selectedIndex++
				v.updatePreview()
			}
			return v, nil

		case "pgup", "ctrl+u":
			if v.focusRight {
				v.viewport.HalfPageUp()
				return v, nil
			}
			v.selectedIndex -= 5
			if v.selectedIndex < 0 {
				v.selectedIndex = 0
			}
			v.updatePreview()
			return v, nil

		case "pgdown", "ctrl+d":
			if v.focusRight {
				v.viewport.HalfPageDown()
				return v, nil
			}
			v.selectedIndex += 5
			if v.selectedIndex >= len(v.results) {
				v.selectedIndex = len(v.results) - 1
			}
			if v.selectedIndex < 0 {
				v.selectedIndex = 0
			}
			v.updatePreview()
			return v, nil
		}

	case tea.MouseMsg:
		// O TopRow ocupa as linhas 2, 3 e 4 (pois a SearchBox tem borda e 3 linhas de altura).
		// Os botões (pills) ficam centralizados verticalmente na linha 3.
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft && msg.Y >= 2 && msg.Y <= 4 {
			// Calculator larguras para descobrir onde o usuário clicou
			searchBoxW := lipgloss.Width(v.styles.SearchBox.Render(v.searchInput.View()))
			modeBadge := lipgloss.NewStyle().Background(v.styles.Theme.SurfaceLight).Foreground(v.styles.Theme.TextMuted).Bold(true).Padding(0, 1).Render("NORMAL (/ search)")
			if v.inputMode {
				modeBadge = lipgloss.NewStyle().Background(v.styles.Theme.Primary).Foreground(v.styles.Theme.TextDark).Bold(true).Padding(0, 1).Render("SEARCH (Esc exit)")
			}
			badgeW := lipgloss.Width(modeBadge)
			scopeLabelW := lipgloss.Width(lipgloss.NewStyle().Foreground(v.styles.Theme.TextMuted).Render("Scope (Alt+S): "))

			// Clicked search box?
			if msg.X < searchBoxW {
				v.inputMode = true
				v.searchInput.Focus()
				return v, textinput.Blink
			}

			// Clicked scopes?
			scopesStartX := searchBoxW + 1 + badgeW + 2 + scopeLabelW
			if msg.X >= scopesStartX {
				x := scopesStartX
				for i, s := range v.scopes {
					name := strings.ToUpper(s)
					pillW := lipgloss.Width(v.styles.TabInactive.Render(name))
					if msg.X >= x && msg.X < x+pillW {
						v.scopeIndex = i
						v.selectedScope = v.scopes[v.scopeIndex]
						v.performSearch()
						return v, nil
					}
					x += pillW + 1 // +1 for space between pills
				}
			}
			return v, nil
		}

		leftWidth := int(float64(v.width) * 0.38)
		if leftWidth < 30 {
			leftWidth = 30
		}

		// Click on lateral notes to open in main tab
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft && msg.Y >= 9 && msg.X < leftWidth {
			availableHeight := v.height - 3
			if availableHeight < 6 {
				availableHeight = 6
			}
			visibleCount := availableHeight - 4
			if visibleCount < 3 {
				visibleCount = 3
			}
			startIdx := 0
			if v.selectedIndex >= visibleCount {
				startIdx = v.selectedIndex - visibleCount + 1
			}

			visibleIdx := msg.Y - 9
			clickedIdx := startIdx + visibleIdx
			if clickedIdx >= 0 && clickedIdx < len(v.results) {
				v.selectedIndex = clickedIdx
				v.focusRight = true
				v.updatePreview()
				return v, nil
			}
		}

		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonWheelUp {
			if v.focusRight {
				if v.viewport.TotalLineCount() > v.viewport.Height {
					v.viewport.ScrollUp(3)
				}
			} else if v.selectedIndex > 0 {
				v.selectedIndex--
				v.updatePreview()
			}
			return v, nil
		}
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonWheelDown {
			if v.focusRight {
				if v.viewport.TotalLineCount() > v.viewport.Height {
					v.viewport.ScrollDown(3)
				}
			} else if v.selectedIndex < len(v.results)-1 {
				v.selectedIndex++
				v.updatePreview()
			}
			return v, nil
		}
	}

	return v, tea.Batch(cmds...)
}

func (v *SearchView) performSearch() {
	query := strings.TrimSpace(v.searchInput.Value())
	if query == "" {
		query = "arquitetura OR sistemas OR staff OR kubernetes"
	}

	if v.engine != nil {
		res, err := v.engine.Search(context.Background(), query, v.selectedScope, 30)
		if err == nil {
			v.results = res
			v.updatePreview()
		}
	}
}

func (v *SearchView) updatePreview() {
	if len(v.results) == 0 || v.selectedIndex >= len(v.results) {
		v.viewport.SetContent("No note selected.")
		return
	}

	res := v.results[v.selectedIndex]
	absPath := filepath.Join(v.vaultRoot, res.RelPath)
	data, err := os.ReadFile(absPath) //nolint:gosec // intentional
	if err != nil {
		v.viewport.SetContent(fmt.Sprintf("Failed to read note: %v", err))
		return
	}

	content := string(data)
	if v.highlightStart > 0 && v.highlightEnd >= v.highlightStart {
		lines := strings.Split(content, "\n")
		for i := v.highlightStart - 1; i < v.highlightEnd && i < len(lines); i++ {
			if i >= 0 {
				lines[i] = "<mark>" + lines[i] + "</mark>"
			}
		}
		content = strings.Join(lines, "\n")
	}

	var backlinksText string
	if v.engine != nil {
		links, err := v.engine.GetBacklinks(res.RelPath)
		if err == nil && len(links) > 0 {
			var sb strings.Builder
			sb.WriteString("\n\n---\n\n### 🔗 Backlinks & References:\n\n")
			for _, l := range links {
				fmt.Fprintf(&sb, "- 📄 `%s`\n", l.SourceRelPath)
			}
			backlinksText = sb.String()
		}
	}

	fullMarkdown := content + backlinksText

	if v.followMode {
		v.linkHints = make(map[string]string)
		hintCounter := 0

		getHint := func() string {
			chars := "abcdefghijklmnopqrstuvwxyz"
			if hintCounter < 26 {
				return string(chars[hintCounter])
			}
			return string(chars[hintCounter/26-1]) + string(chars[hintCounter%26])
		}

		// Markdown links [text](relPath)
		reMdLink := regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
		fullMarkdown = reMdLink.ReplaceAllStringFunc(fullMarkdown, func(match string) string {
			sub := reMdLink.FindStringSubmatch(match)
			if len(sub) < 3 {
				return match
			}
			target := sub[2]
			if strings.HasPrefix(target, "http") || strings.HasPrefix(target, "file:") {
				return match
			}
			hint := getHint()
			v.linkHints[hint] = target
			hintCounter++

			// Highlight the hint itself so it's visible. Glamour supports <mark> or just **bold**.
			// We can use a bold yellow or just brackets.
			return fmt.Sprintf("[[%s] %s](%s)", hint, sub[1], sub[2])
		})

		// Backlinks rendering for hints (we outputted them as code blocks, let's regex those)
		reBacklink := regexp.MustCompile(`(?m)^- 📄 ` + "`" + `([^` + "`" + `]+)` + "`" + `$`)
		fullMarkdown = reBacklink.ReplaceAllStringFunc(fullMarkdown, func(match string) string {
			sub := reBacklink.FindStringSubmatch(match)
			if len(sub) < 2 {
				return match
			}
			hint := getHint()
			v.linkHints[hint] = sub[1]
			hintCounter++
			return fmt.Sprintf("- 📄 [%s] `%s`", hint, sub[1])
		})

		// If in follow mode, show a small header indicating we are waiting for input
		header := fmt.Sprintf("\n> **FOLLOW MODE:** Type hint to navigate (Current: `%s`)\n\n", v.hintInput)
		fullMarkdown = header + fullMarkdown
	}

	renderer, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle("dark"),
		glamour.WithWordWrap(v.viewport.Width-4),
	)

	if err == nil {
		rendered, err := renderer.Render(fullMarkdown)
		if err == nil {
			v.viewport.SetContent(rendered)
			v.viewport.GotoTop()
			return
		}
	}

	v.viewport.SetContent(content)
	v.viewport.GotoTop()
}

// SetSize updates dimensions of the split-pane.
func (v *SearchView) SetSize(w, h int) {
	v.width = w
	v.height = h

	_, rightWidth, availableHeight := CalculateLayoutDimensions(w, h, 2, 0.38)

	v.viewport.Width = rightWidth - 4
	v.viewport.Height = availableHeight - 4
	v.updatePreview()
}

// View renders the search split-pane.
func (v *SearchView) View() string {
	var scopePills strings.Builder
	for i, s := range v.scopes {
		name := strings.ToUpper(s)
		if s == v.selectedScope {
			scopePills.WriteString(v.styles.TabActive.Render(name))
		} else {
			scopePills.WriteString(v.styles.TabInactive.Render(name))
		}
		if i < len(v.scopes)-1 {
			scopePills.WriteString(" ")
		}
	}

	// Mode badge
	modeBadge := lipgloss.NewStyle().Background(v.styles.Theme.SurfaceLight).Foreground(v.styles.Theme.TextMuted).Bold(true).Padding(0, 1).Render("NORMAL (/ search)")
	searchBoxStyle := v.styles.SearchBox
	if v.inputMode {
		modeBadge = lipgloss.NewStyle().Background(v.styles.Theme.Primary).Foreground(v.styles.Theme.TextDark).Bold(true).Padding(0, 1).Render("SEARCH (Esc exit)")
		searchBoxStyle = v.styles.PanelFocused
	}

	topRow := lipgloss.JoinHorizontal(
		lipgloss.Center,
		searchBoxStyle.Render(v.searchInput.View()),
		" ",
		modeBadge,
		"  ",
		lipgloss.NewStyle().Foreground(v.styles.Theme.TextMuted).Render("Scope (Alt+S): "),
		scopePills.String(),
	)

	leftWidth, _, availableHeight := CalculateLayoutDimensions(v.width, v.height, lipgloss.Height(topRow), 0.38)

	var listContent strings.Builder
	fmt.Fprintf(&listContent, "%s (%d found):\n\n", v.styles.PanelTitle.Render("📋 Notes"), len(v.results))

	visibleCount := availableHeight - 4
	if visibleCount < 3 {
		visibleCount = 3
	}

	startIdx := 0
	if v.selectedIndex >= visibleCount {
		startIdx = v.selectedIndex - visibleCount + 1
	}
	endIdx := startIdx + visibleCount
	if endIdx > len(v.results) {
		endIdx = len(v.results)
	}

	for i := startIdx; i < endIdx; i++ {
		r := v.results[i]
		badge := v.renderScopeBadge(r.Scope)
		title := r.Title
		if len(title) > leftWidth-16 {
			title = title[:leftWidth-19] + "..."
		}

		line := fmt.Sprintf("%s %s", badge, title)
		if i == v.selectedIndex {
			if !v.focusRight {
				listContent.WriteString(v.styles.ListItemSelected.Width(leftWidth - 4).Render("▸ " + line))
			} else {
				listContent.WriteString(v.styles.ListItem.Width(leftWidth - 4).Render("▸ " + line))
			}
		} else {
			listContent.WriteString(v.styles.ListItem.Width(leftWidth - 4).Render("  " + line))
		}
		listContent.WriteString("\n")
	}

	if len(v.results) == 0 {
		listContent.WriteString(lipgloss.NewStyle().Foreground(v.styles.Theme.TextMuted).Render("\n  No notes found.\n"))
	}

	leftBoxStyle := v.styles.SplitLeft
	if !v.focusRight && !v.inputMode {
		leftBoxStyle = v.styles.PanelFocused
	}

	rightBoxStyle := v.styles.SplitRight
	if v.focusRight {
		rightBoxStyle = v.styles.PanelFocused
	}

	previewHeader := ""
	if len(v.results) > 0 && v.selectedIndex < len(v.results) {
		previewHeader = fmt.Sprintf("📄 %s (%s)", v.results[v.selectedIndex].Title, v.results[v.selectedIndex].RelPath)
	} else {
		previewHeader = "📄 Note Preview"
	}

	rightContent := lipgloss.JoinVertical(
		lipgloss.Left,
		v.styles.PanelTitle.Render(previewHeader),
		v.viewport.View(),
	)

	return RenderSplitLayout(SplitLayoutConfig{
		Width:        v.width,
		Height:       v.height,
		LeftRatio:    0.38,
		TopBar:       topRow,
		LeftContent:  listContent.String(),
		RightContent: rightContent,
		LeftStyle:    leftBoxStyle,
		RightStyle:   rightBoxStyle,
	})
}

func (v *SearchView) renderScopeBadge(scope string) string {
	switch strings.ToLower(scope) {
	case "tech":
		return v.styles.BadgeTech.Render("Tech")
	case "daily":
		return v.styles.BadgeDaily.Render("Daily")
	case "meta":
		return v.styles.BadgeMeta.Render("Meta")
	case "sources":
		return v.styles.BadgeSource.Render("Source")
	case "inbox":
		return v.styles.BadgeInbox.Render("Inbox")
	default:
		return v.styles.BadgeTech.Render(scope)
	}
}
