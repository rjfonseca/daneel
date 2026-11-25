//nolint:revive // package comment not required
package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	graphv1 "github.com/rjfonseca/daneel/api/v1"
	"github.com/rjfonseca/daneel/internal/ops/workspace"
)

type repoItem struct {
	workspace.RepoInfo
	styles Styles
}

func (r repoItem) Title() string {
	badge := "🟢"
	if r.Dirty {
		badge = fmt.Sprintf("🟡%d", r.ModifiedCount+r.UntrackedCount)
	}
	stackTag := ""
	if r.Stack != "" && r.Stack != "Other" {
		stackTag = " " + lipgloss.NewStyle().Foreground(r.styles.Theme.TextMuted).Render("["+r.Stack+"]")
	}
	return fmt.Sprintf("%s %s%s", badge, r.Name, stackTag)
}
func (r repoItem) Description() string { return "" }
func (r repoItem) FilterValue() string { return r.Name }

// WorkspaceView displays the galactic workspace of repositories.
type WorkspaceView struct {
	vaultRoot     string
	workDir       string
	styles        Styles
	backend       *Backend
	report        *workspace.WorkspaceReport
	filteredRepos []workspace.RepoInfo
	listModel     *DaneelList
	filter        string
	filters       []string
	filterIndex   int
	fromCache     bool
	loaded        bool
	scanning      bool
	statusMsg     string
	width         int
	height        int
}

type workspaceLoadedMsg struct {
	resp *graphv1.GetWorkspaceOverviewResponse
	err  error
}

// NewWorkspaceView creates a new workspace view without blocking startup.
func NewWorkspaceView(vaultRoot string, styles Styles) *WorkspaceView {
	wv := &WorkspaceView{
		vaultRoot:   vaultRoot,
		styles:      styles,
		filters:     []string{"all", "dirty", "clean", "go", "rust", "node"},
		filterIndex: 0,
		filter:      "all",
	}
	wv.listModel = NewDaneelList([]list.Item{}, 30, 10, styles)
	return wv
}

// SetBackend injects the daemon-first backend for cached/live workspace access.
func (v *WorkspaceView) SetBackend(b *Backend) {
	v.backend = b
	if b != nil {
		v.workDir = b.WorkDir()
	}
}

// EnsureLoaded loads cached workspace repositories on first activation.
func (v *WorkspaceView) EnsureLoaded() tea.Cmd {
	if v.loaded || v.scanning {
		return nil
	}
	return v.loadAsync(false)
}

// RescanAsync triggers a live Git scan across workDir without blocking the UI thread.
func (v *WorkspaceView) RescanAsync() tea.Cmd {
	return v.loadAsync(true)
}

func (v *WorkspaceView) loadAsync(forceRescan bool) tea.Cmd {
	v.scanning = true
	backend := v.backend
	if backend == nil {
		backend = NewLocalBackend(v.vaultRoot, nil, nil)
	}
	return func() tea.Msg {
		resp, err := backend.GetWorkspaceOverview(context.Background(), forceRescan)
		return workspaceLoadedMsg{resp: resp, err: err}
	}
}

// Refresh synchronously refreshes workspace repositories (used in direct tests).
func (v *WorkspaceView) Refresh() {
	backend := v.backend
	if backend == nil {
		backend = NewLocalBackend(v.vaultRoot, nil, nil)
	}
	if resp, err := backend.GetWorkspaceOverview(context.Background(), false); err == nil && resp != nil {
		v.applyOverviewResponse(resp)
	}
}

func (v *WorkspaceView) applyOverviewResponse(resp *graphv1.GetWorkspaceOverviewResponse) {
	v.scanning = false
	v.loaded = true
	if resp == nil {
		return
	}
	if resp.WorkDir != "" {
		v.workDir = resp.WorkDir
	}
	v.fromCache = resp.FromCache
	v.statusMsg = ""

	repos := make([]workspace.RepoInfo, 0, len(resp.Repos))
	for _, item := range resp.Repos {
		repos = append(repos, workspace.RepoInfo{
			Name:              item.Name,
			RelPath:           item.RelPath,
			Category:          item.Category,
			Branch:            item.Branch,
			Stack:             item.Stack,
			ComponentType:     item.ComponentType,
			Commands:          item.Commands,
			Description:       item.Description,
			VaultLink:         item.VaultLink,
			LastCommitDate:    item.LastCommitDate,
			DaysSinceActivity: int(item.DaysSinceActivity),
			Dirty:             item.Dirty,
			ModifiedCount:     int(item.ModifiedCount),
			UntrackedCount:    int(item.UntrackedCount),
			FilesPreview:      append([]string(nil), item.FilesPreview...),
		})
	}

	v.report = &workspace.WorkspaceReport{
		TotalRepos: int(resp.TotalRepos),
		CleanCount: int(resp.CleanCount),
		DirtyCount: int(resp.DirtyCount),
		ScannedAt:  resp.ScannedAt,
		Categories: map[string][]workspace.RepoInfo{"all": repos},
	}
	v.applyFilter()
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

	listItems := make([]list.Item, 0, len(filtered))
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
	case workspaceLoadedMsg:
		v.scanning = false
		v.loaded = true
		if msg.err != nil {
			v.statusMsg = fmt.Sprintf("⚠️ Workspace error: %v", msg.err)
		} else if msg.resp != nil {
			v.applyOverviewResponse(msg.resp)
		}
		return v, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "r", "w":
			return v, v.RescanAsync()
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

	v.listModel.SetSize(max(10, leftWidth-4), max(3, availableHeight-4))
	v.listModel.XOffset = 1
	v.listModel.YOffset = 7
}

// View renders the workspace explorer within bounded split-pane dimensions.
func (v *WorkspaceView) View() string {
	targetDir := v.workDir
	if targetDir == "" && v.backend != nil {
		targetDir = v.backend.WorkDir()
	}
	if targetDir == "" {
		targetDir = "~/Projects"
	}

	if v.report == nil {
		if v.statusMsg != "" {
			return v.statusMsg
		}
		return fmt.Sprintf("Loading workspace repositories in %s...", targetDir)
	}

	_, rightWidth, _ := CalculateLayoutDimensions(v.width, v.height, 2, 0.45)

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

	syncBadge := "⚡ live"
	if v.scanning {
		syncBadge = "⏳ scanning..."
	} else if v.fromCache {
		syncBadge = "⚡ cached (r: rescan)"
	}

	var statsText string
	if v.width > 0 && v.width < 110 {
		statsText = fmt.Sprintf(" %d (%s/%s) %s",
			v.report.TotalRepos,
			v.styles.BadgeClean.Render(fmt.Sprintf("%d", v.report.CleanCount)),
			v.styles.BadgeDirty.Render(fmt.Sprintf("%d", v.report.DirtyCount)),
			syncBadge,
		)
	} else {
		statsText = fmt.Sprintf("  📊 %d repos (%s clean | %s dirty) • %s",
			v.report.TotalRepos,
			v.styles.BadgeClean.Render(fmt.Sprintf("%d", v.report.CleanCount)),
			v.styles.BadgeDirty.Render(fmt.Sprintf("%d", v.report.DirtyCount)),
			syncBadge,
		)
	}

	topBarRaw := lipgloss.JoinHorizontal(lipgloss.Center,
		filterPills.String(),
		lipgloss.NewStyle().Foreground(v.styles.Theme.TextMuted).Render(" (F)"),
		statsText,
	)
	topBar := topBarRaw
	if v.width > 0 {
		topBar = lipgloss.NewStyle().MaxWidth(v.width).Render(topBarRaw)
	}

	var listSB strings.Builder
	fmt.Fprintf(&listSB, "%s (%d):\n\n", v.styles.PanelTitle.Render("📁 Repositories"), len(v.filteredRepos))
	listSB.WriteString(v.listModel.View())

	var detailSB strings.Builder
	detailWidth := max(10, rightWidth-4)
	labelStyle := lipgloss.NewStyle().Bold(true).Foreground(v.styles.Theme.Info)
	valStyle := lipgloss.NewStyle().Foreground(v.styles.Theme.Text)

	if len(v.filteredRepos) > 0 && v.listModel.Index() >= 0 && v.listModel.Index() < len(v.filteredRepos) {
		r := v.filteredRepos[v.listModel.Index()]

		detailSB.WriteString(v.styles.PanelTitle.Render(fmt.Sprintf("📦 %s", r.Name)) + "\n\n")
		writeKV(&detailSB, labelStyle, valStyle, "Path", r.RelPath)
		writeKV(&detailSB, labelStyle, valStyle, "Branch", r.Branch)
		writeKV(&detailSB, labelStyle, valStyle, "Stack", r.Stack)
		if r.ComponentType != "" {
			writeKV(&detailSB, labelStyle, valStyle, "Type", r.ComponentType)
		}
		if r.Commands != "" {
			writeKV(&detailSB, labelStyle, valStyle, "Test/Build", r.Commands)
		}
		if r.LastCommitDate != "" {
			writeKV(&detailSB, labelStyle, valStyle, "Last Commit", fmt.Sprintf("%s (%dd ago)", r.LastCommitDate, r.DaysSinceActivity))
		}
		if r.VaultLink != "" {
			writeKV(&detailSB, labelStyle, valStyle, "Vault Note", r.VaultLink)
		}

		detailSB.WriteString("\n" + lipgloss.NewStyle().Foreground(v.styles.Theme.Border).Render(strings.Repeat("─", max(1, rightWidth-6))) + "\n")
		if r.Description != "" {
			detailSB.WriteString(valStyle.Width(detailWidth).Render(r.Description) + "\n\n")
		}

		if r.Dirty {
			detailSB.WriteString(v.styles.BadgeDirty.Render("⚠️ Local Uncommitted Changes:") + "\n")
			fmt.Fprintf(&detailSB, "  • Modified: %d  • Untracked: %d\n", r.ModifiedCount, r.UntrackedCount)
			for i, fp := range r.FilesPreview {
				if i >= 8 {
					break
				}
				fmt.Fprintf(&detailSB, "  - %s\n", truncateText(fp, detailWidth-4))
			}
		} else {
			detailSB.WriteString(v.styles.BadgeClean.Render("✅ Repository Clean and Synchronized.") + "\n")
		}
	} else {
		detailSB.WriteString(lipgloss.NewStyle().Foreground(v.styles.Theme.TextMuted).Render("No repository selected.\n"))
	}

	return RenderSplitLayout(SplitLayoutConfig{
		Width:        v.width,
		Height:       v.height,
		LeftRatio:    0.45,
		TopBar:       topBar,
		LeftContent:  listSB.String(),
		RightContent: detailSB.String(),
		LeftStyle:    v.styles.Panel,
		RightStyle:   v.styles.Panel,
	})
}

func writeKV(sb *strings.Builder, labelStyle, valStyle lipgloss.Style, key, val string) {
	if val == "" {
		return
	}
	fmt.Fprintf(sb, "%s %s\n", labelStyle.Render(key+":"), valStyle.Render(val))
}
