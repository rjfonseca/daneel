//nolint:revive // package comment not required
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/rjfonseca/daneel/internal/ops/workspace"
)

type repoItem struct {
	workspace.RepoInfo
	styles Styles
}

func (r repoItem) Title() string {
	statusBadge := r.styles.BadgeClean.Render("🟢")
	if r.Dirty {
		statusBadge = r.styles.BadgeDirty.Render(fmt.Sprintf("🟡 %dM", r.ModifiedCount+r.UntrackedCount))
	}
	// We pad ComponentType. Bubbles/list handles truncation for the whole line.
	return fmt.Sprintf("%-6s %-16s %s", statusBadge, r.ComponentType, r.Name)
}
func (r repoItem) Description() string { return "" }
func (r repoItem) FilterValue() string { return r.Name }

// WorkspaceView displays the galactic workspace of repositories.
type WorkspaceView struct {
	vaultRoot     string
	styles        Styles
	scanner       *workspace.Scanner
	report        *workspace.WorkspaceReport
	filteredRepos []workspace.RepoInfo
	listModel     *DaneelList
	filter        string
	filters       []string
	filterIndex   int
	width         int
	height        int
}

// NewWorkspaceView creates a new workspace view.
func NewWorkspaceView(vaultRoot string, styles Styles) *WorkspaceView {
	sc := workspace.NewScanner("", vaultRoot)
	wv := &WorkspaceView{
		vaultRoot:   vaultRoot,
		styles:      styles,
		scanner:     sc,
		filters:     []string{"all", "dirty", "clean", "go", "rust", "node"},
		filterIndex: 0,
		filter:      "all",
	}
	wv.listModel = NewDaneelList([]list.Item{}, 30, 10, styles)
	wv.Refresh()
	return wv
}

// Refresh rescans the workspace directory.
func (v *WorkspaceView) Refresh() {
	rep, err := v.scanner.Scan(false)
	if err == nil {
		v.report = rep
		v.applyFilter()
	}
}

func (v *WorkspaceView) applyFilter() {
	if v.report == nil {
		return
	}

	var all []workspace.RepoInfo
	for _, repos := range v.report.Categories {
		all = append(all, repos...)
	}

	var filtered []workspace.RepoInfo
	for _, r := range all {
		switch v.filter {
		case "dirty", "modificados":
			if r.Dirty {
				filtered = append(filtered, r)
			}
		case "clean", "limpos":
			if !r.Dirty {
				filtered = append(filtered, r)
			}
		case "go":
			if strings.Contains(r.Stack, "Go") {
				filtered = append(filtered, r)
			}
		case "rust":
			if strings.Contains(r.Stack, "Rust") {
				filtered = append(filtered, r)
			}
		case "node":
			if strings.Contains(r.Stack, "Node") || strings.Contains(r.Stack, "TypeScript") || strings.Contains(r.Stack, "JavaScript") {
				filtered = append(filtered, r)
			}
		default:
			filtered = append(filtered, r)
		}
	}

	v.filteredRepos = filtered

	var listItems []list.Item
	for _, r := range filtered {
		listItems = append(listItems, repoItem{r, v.styles})
	}
	v.listModel.SetItems(listItems)
	v.listModel.Select(0)
}

// Init initializes the view.
func (v *WorkspaceView) Init() tea.Cmd {
	return nil
}

// Update handles tea events for workspace view.
func (v *WorkspaceView) Update(msg tea.Msg) (*WorkspaceView, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "r", "w":
			v.Refresh()
			return v, nil
		case "f", "ctrl+f":
			v.filterIndex = (v.filterIndex + 1) % len(v.filters)
			v.filter = v.filters[v.filterIndex]
			v.applyFilter()
			return v, nil
		}

	case tea.MouseMsg:
		// Top Bar is at Y == 2 (Header = 0, Tabs = 1)
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft && msg.Y == 2 {
			x := 0
			for i, f := range v.filters {
				pillW := lipgloss.Width(v.styles.TabInactive.Render(strings.ToUpper(f)))
				if msg.X >= x && msg.X < x+pillW {
					v.filterIndex = i
					v.filter = v.filters[v.filterIndex]
					v.applyFilter()
					return v, nil
				}
				x += pillW + 1 // +1 for space between pills
			}
			return v, nil
		}
	case ListItemSelectedMsg:
		// Optional
	}

	v.listModel, cmd = v.listModel.Update(msg)
	return v, cmd
}

// SetSize updates dimensions.
func (v *WorkspaceView) SetSize(w, h int) {
	v.width = w
	v.height = h
	leftWidth, _, availableHeight := CalculateLayoutDimensions(w, h, 2, 0.45)

	v.listModel.SetSize(leftWidth-2, availableHeight-3) // -3 for borders and title
	v.listModel.XOffset = 1
	v.listModel.YOffset = 7
}

// View renders the workspace explorer.
func (v *WorkspaceView) View() string {
	if v.report == nil {
		return "Scanning workspace repositories in ~/Work..."
	}

	leftWidth := int(float64(v.width) * 0.45)
	if leftWidth < 35 {
		leftWidth = 35
	}
	rightWidth := v.width - leftWidth - 3
	if rightWidth < 20 {
		rightWidth = 20
	}

	var filterPills strings.Builder
	for i, f := range v.filters {
		name := strings.ToUpper(f)
		if f == v.filter {
			filterPills.WriteString(v.styles.TabActive.Render(name))
		} else {
			filterPills.WriteString(v.styles.TabInactive.Render(name))
		}
		if i < len(v.filters)-1 {
			filterPills.WriteString(" ")
		}
	}

	statsText := fmt.Sprintf("📊 %d repositories (%s clean | %s dirty)",
		v.report.TotalRepos,
		v.styles.BadgeClean.Render(fmt.Sprintf("%d", v.report.CleanCount)),
		v.styles.BadgeDirty.Render(fmt.Sprintf("%d", v.report.DirtyCount)),
	)

	topBar := lipgloss.JoinHorizontal(lipgloss.Center,
		filterPills.String(),
		"   ",
		lipgloss.NewStyle().Foreground(v.styles.Theme.TextMuted).Render("(Filter: F)  "),
		statsText,
	)

	var listSB strings.Builder
	fmt.Fprintf(&listSB, "%s (%d listed):\n\n", v.styles.PanelTitle.Render("📁 Repositories"), len(v.filteredRepos))
	listSB.WriteString(v.listModel.View())

	var detailSB strings.Builder
	if len(v.filteredRepos) > 0 && v.listModel.Index() >= 0 && v.listModel.Index() < len(v.filteredRepos) {
		r := v.filteredRepos[v.listModel.Index()]

		detailSB.WriteString(v.styles.PanelTitle.Render(fmt.Sprintf("📦 %s", r.Name)) + "\n\n")
		fmt.Fprintf(&detailSB, "📁 **Path:** `%s`\n", r.RelPath)
		fmt.Fprintf(&detailSB, "🌿 **Branch:** `%s`\n", r.Branch)
		fmt.Fprintf(&detailSB, "⚡ **Stack:** `%s`\n", r.Stack)
		fmt.Fprintf(&detailSB, "🏗️ **Component Type:** %s\n", r.ComponentType)
		fmt.Fprintf(&detailSB, "🧪 **Test / Build:** %s\n", r.Commands)
		fmt.Fprintf(&detailSB, "📅 **Last Commit:** %s (%d days ago)\n", r.LastCommitDate, r.DaysSinceActivity)

		if r.VaultLink != "" {
			fmt.Fprintf(&detailSB, "🔗 **Vault Note:** [%s](file://%s/%s)\n", r.Name, v.vaultRoot, r.VaultLink)
		}

		detailSB.WriteString("\n---\n\n")
		fmt.Fprintf(&detailSB, "📝 **Description:** %s\n\n", r.Description)

		if r.Dirty {
			detailSB.WriteString(v.styles.BadgeDirty.Render("⚠️ Local Uncommitted Changes:\n"))
			fmt.Fprintf(&detailSB, "   • Modified Files: %d\n", r.ModifiedCount)
			fmt.Fprintf(&detailSB, "   • Untracked Files: %d\n\n", r.UntrackedCount)
			if len(r.FilesPreview) > 0 {
				detailSB.WriteString("   **File Preview:**\n")
				for _, fp := range r.FilesPreview {
					fmt.Fprintf(&detailSB, "   - `%s`\n", fp)
				}
			}
		} else {
			detailSB.WriteString(v.styles.BadgeClean.Render("✅ Repository Clean and Synchronized.\n"))
		}
	} else {
		detailSB.WriteString(lipgloss.NewStyle().Foreground(v.styles.Theme.TextMuted).Render("No repository selected.\n"))
	}

	leftPane := v.styles.Panel.
		Width(leftWidth).MaxWidth(leftWidth).
		Render(listSB.String())

	rightPane := v.styles.Panel.
		Width(rightWidth).MaxWidth(rightWidth).
		Render(detailSB.String())

	splitView := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, " ", rightPane)
	return lipgloss.JoinVertical(lipgloss.Left, topBar, "", splitView)
}
