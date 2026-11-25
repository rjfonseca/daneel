//nolint:revive // package comment not required
package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	graphv1 "github.com/rjfonseca/daneel/api/v1"
	"github.com/rjfonseca/daneel/internal/dream"
	"github.com/rjfonseca/daneel/internal/ops/backlog"
	"github.com/rjfonseca/daneel/internal/vault"
)

// DashboardView presents the Staff Engineering Radar, Telemetry bars, Morning Handoff, and Active Backlog.
type DashboardView struct {
	vaultRoot        string
	styles           Styles
	backend          *Backend
	summary          *graphv1.GetDashboardSummaryResponse
	temporalData     *dream.TemporalWindowData
	backlogRep       *backlog.BacklogReport
	vaultMetrics     *vault.VaultMetrics
	backlogIndex     int
	showCompleted    bool
	loaded           bool
	loading          bool
	width            int
	height           int
	customWidgetText string
}

type dashboardLoadedMsg struct {
	summary *graphv1.GetDashboardSummaryResponse
	err     error
}

// NewDashboardView creates a new dashboard view without synchronous startup I/O.
func NewDashboardView(vaultRoot string, styles Styles) *DashboardView {
	return &DashboardView{
		vaultRoot: vaultRoot,
		styles:    styles,
	}
}

// SetBackend injects the daemon-first backend for unified dashboard telemetry.
func (v *DashboardView) SetBackend(b *Backend) {
	v.backend = b
}

// EnsureLoaded triggers async loading on first activation.
func (v *DashboardView) EnsureLoaded() tea.Cmd {
	if v.loaded || v.loading {
		return nil
	}
	return v.LoadAsync()
}

// LoadAsync fetches the dashboard summary asynchronously via daemon or local builders.
func (v *DashboardView) LoadAsync() tea.Cmd {
	v.loading = true
	backend := v.backend
	if backend == nil {
		backend = NewLocalBackend(v.vaultRoot, nil, nil)
	}
	return func() tea.Msg {
		resp, err := backend.GetDashboardSummary(context.Background(), 7)
		return dashboardLoadedMsg{summary: resp, err: err}
	}
}

// Refresh synchronously reloads dashboard data from the backend.
func (v *DashboardView) Refresh() {
	backend := v.backend
	if backend == nil {
		backend = NewLocalBackend(v.vaultRoot, nil, nil)
	}
	if resp, err := backend.GetDashboardSummary(context.Background(), 7); err == nil && resp != nil {
		v.applySummary(resp)
	}
}

func (v *DashboardView) applySummary(resp *graphv1.GetDashboardSummaryResponse) {
	v.loading = false
	v.loaded = true
	v.summary = resp
	if resp == nil {
		return
	}

	bRep := &backlog.BacklogReport{
		Items: make([]backlog.BacklogItem, 0, len(resp.BacklogItems)),
	}
	for _, item := range resp.BacklogItems {
		bRep.Items = append(bRep.Items, backlog.BacklogItem{
			Priority:    item.Priority,
			Title:       item.Title,
			Description: item.Description,
			Completed:   item.Completed,
		})
	}
	v.backlogRep = bRep
	v.clampBacklogIndex()
}

func (v *DashboardView) filteredBacklog() []backlog.BacklogItem {
	if v.backlogRep == nil {
		return nil
	}
	if v.showCompleted {
		return v.backlogRep.Items
	}
	var open []backlog.BacklogItem
	for _, item := range v.backlogRep.Items {
		if !item.Completed {
			open = append(open, item)
		}
	}
	return open
}

func (v *DashboardView) clampBacklogIndex() {
	items := v.filteredBacklog()
	if len(items) == 0 {
		v.backlogIndex = 0
		return
	}
	if v.backlogIndex >= len(items) {
		v.backlogIndex = len(items) - 1
	}
	if v.backlogIndex < 0 {
		v.backlogIndex = 0
	}
}

// Init initializes the view.
func (v *DashboardView) Init() tea.Cmd {
	return nil
}

// Update handles tea messages for the dashboard.
func (v *DashboardView) Update(msg tea.Msg) (*DashboardView, tea.Cmd) {
	switch msg := msg.(type) {
	case dashboardLoadedMsg:
		v.loading = false
		v.loaded = true
		if msg.err == nil && msg.summary != nil {
			v.applySummary(msg.summary)
		}
		return v, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "r":
			return v, v.LoadAsync()
		case "c":
			v.showCompleted = !v.showCompleted
			v.clampBacklogIndex()
			return v, nil
		case "up", "k":
			if v.backlogIndex > 0 {
				v.backlogIndex--
			}
			return v, nil
		case "down", "j":
			items := v.filteredBacklog()
			if v.backlogIndex < len(items)-1 {
				v.backlogIndex++
			}
			return v, nil
		}
	}
	return v, nil
}

// SetSize updates dimensions.
func (v *DashboardView) SetSize(w, h int) {
	v.width = w
	v.height = h
}

// View renders the Staff Dashboard.
func (v *DashboardView) View() string {
	if !v.loaded && v.summary == nil && v.temporalData == nil {
		return "Loading Staff Telemetry & Dashboard..."
	}

	leftWidth, rightWidth, availH := CalculateLayoutDimensions(v.width, v.height, 0, 0.50)

	var teleSB strings.Builder
	v.renderLeftPane(&teleSB, leftWidth)

	var backSB strings.Builder
	v.renderRightPane(&backSB, rightWidth, availH)

	return RenderSplitLayout(SplitLayoutConfig{
		Width:        v.width,
		Height:       v.height,
		LeftRatio:    0.50,
		TopBar:       "",
		LeftContent:  teleSB.String(),
		RightContent: backSB.String(),
		LeftStyle:    v.styles.Panel,
		RightStyle:   v.styles.Panel,
	})
}

func (v *DashboardView) renderLeftPane(teleSB *strings.Builder, leftWidth int) {
	windowDays := 7
	daysWithLogs := 0
	totalHours := 0.0
	var driftAlerts []string
	var frictionSignals []*graphv1.FrictionSignalItem

	if v.summary != nil {
		if v.summary.WindowDays > 0 {
			windowDays = int(v.summary.WindowDays)
		}
		daysWithLogs = int(v.summary.DaysWithLogs)
		totalHours = v.summary.TotalHours
		driftAlerts = v.summary.DriftAlerts
		frictionSignals = v.summary.FrictionSignals
	} else if v.temporalData != nil {
		if v.temporalData.WindowDays > 0 {
			windowDays = v.temporalData.WindowDays
		}
		daysWithLogs = v.temporalData.DaysWithLogs
		totalHours = v.temporalData.TotalHours
		driftAlerts = v.temporalData.DriftAlerts
		for _, fs := range v.temporalData.FrictionSignals {
			frictionSignals = append(frictionSignals, &graphv1.FrictionSignalItem{Date: fs.Date, Line: fs.Line})
		}
	}

	fmt.Fprintf(teleSB, "%s (%d-Day Window — %.1fh across %d logs):\n\n",
		v.styles.PanelTitle.Render("📊 Staff Engineering Telemetry"),
		windowDays, totalHours, daysWithLogs)

	if v.summary != nil && len(v.summary.Taxonomy) > 0 {
		for _, tax := range v.summary.Taxonomy {
			label, target, color := v.taxonomyStyle(tax.Id, tax.TargetMin, tax.TargetMax)
			teleSB.WriteString(v.renderProgressBar(label, tax.Percentage, target, color, leftWidth-8))
			teleSB.WriteString("\n")
		}
	} else if v.temporalData != nil {
		pctA := firstPercentage(v.temporalData.Percentages, "A_Estrategico", "A")
		pctB := firstPercentage(v.temporalData.Percentages, "B_ImpactoLocal", "B")
		pctC := firstPercentage(v.temporalData.Percentages, "C_GlueWork", "C")
		pctE := firstPercentage(v.temporalData.Percentages, "E_Overhead", "E")

		teleSB.WriteString(v.renderProgressBar("[A] Strategic    ", pctA, ">= 30%", v.styles.Theme.Primary, leftWidth-8) + "\n")
		teleSB.WriteString(v.renderProgressBar("[B] Local Impact ", pctB, ">= 30%", v.styles.Theme.Info, leftWidth-8) + "\n")
		teleSB.WriteString(v.renderProgressBar("[C] Glue Work    ", pctC, "<= 30%", v.styles.Theme.Secondary, leftWidth-8) + "\n")
		teleSB.WriteString(v.renderProgressBar("[E] Overhead     ", pctE, "<= 10%", v.styles.Theme.Warning, leftWidth-8) + "\n")
	}
	teleSB.WriteString("\n")

	if len(driftAlerts) > 0 {
		for _, da := range driftAlerts {
			teleSB.WriteString(lipgloss.NewStyle().Foreground(v.styles.Theme.Error).Bold(true).Render("⚠️ Drift: ") + da + "\n")
		}
	} else {
		teleSB.WriteString(lipgloss.NewStyle().Foreground(v.styles.Theme.Success).Bold(true).Render("✅ Time allocation aligned with Staff targets.\n"))
	}

	if v.summary != nil && v.summary.Handoff != nil {
		h := v.summary.Handoff
		teleSB.WriteString("\n" + v.styles.PanelTitle.Render(fmt.Sprintf("🤝 Morning Handoff (%s):", h.Date)) + "\n")
		fmt.Fprintf(teleSB, "  • REM Cycles: %d | Inbox: %d | Alerts: %d\n",
			h.DreamCyclesCount, len(h.InboxItems), len(h.Alerts))
		for i, act := range h.AutonomousActions {
			if i >= 2 {
				break
			}
			fmt.Fprintf(teleSB, "  • %s\n", truncateText(act, max(20, leftWidth-8)))
		}
		for i, al := range h.Alerts {
			if i >= 2 {
				break
			}
			teleSB.WriteString(lipgloss.NewStyle().Foreground(v.styles.Theme.Warning).Render("  ! "+truncateText(al, max(20, leftWidth-8))) + "\n")
		}
		if len(h.Suggestions) > 0 {
			teleSB.WriteString(lipgloss.NewStyle().Foreground(v.styles.Theme.TextMuted).Render("  💡 "+truncateText(h.Suggestions[0], max(20, leftWidth-8))) + "\n")
		}
	}

	if len(frictionSignals) > 0 {
		teleSB.WriteString("\n" + v.styles.PanelTitle.Render("⚠️ Friction Signals:") + "\n")
		for _, fs := range frictionSignals[:min(2, len(frictionSignals))] {
			fmt.Fprintf(teleSB, "  • [%s] %s\n", fs.Date, truncateText(fs.Line, max(20, leftWidth-18)))
		}
	}

	var notes, links, orphans int
	if v.summary != nil && v.summary.TotalNotes > 0 {
		notes = int(v.summary.TotalNotes)
		links = int(v.summary.TotalLinks)
		orphans = int(v.summary.OrphanNotes)
	} else if v.vaultMetrics != nil {
		notes = v.vaultMetrics.TotalNotes
		links = v.vaultMetrics.TotalLinks
		orphans = v.vaultMetrics.OrphanNotes
	}
	if notes > 0 {
		ratio := float64(links) / float64(notes)
		teleSB.WriteString("\n" + lipgloss.NewStyle().Foreground(v.styles.Theme.Info).Bold(true).
			Render(fmt.Sprintf("🧠 Vault: %d notes • %d links (%.2f/note) • %d orphans\n", notes, links, ratio, orphans)))
	}
}

func (v *DashboardView) renderRightPane(backSB *strings.Builder, rightWidth, availH int) {
	items := v.filteredBacklog()
	openCount := 0
	if v.backlogRep != nil {
		for _, it := range v.backlogRep.Items {
			if !it.Completed {
				openCount++
			}
		}
	}

	modeLabel := "open — 'c': show done"
	if v.showCompleted {
		modeLabel = "all — 'c': open only"
	}
	backSB.WriteString(v.styles.PanelTitle.Render(fmt.Sprintf("📌 Active Backlog (%d open)", openCount)) + " " +
		lipgloss.NewStyle().Foreground(v.styles.Theme.TextMuted).Render("["+modeLabel+"]") + "\n\n")

	if len(items) > 0 {
		maxVisible := availH - 12
		if maxVisible < 3 {
			maxVisible = 3
		}
		start := 0
		if v.backlogIndex >= maxVisible {
			start = v.backlogIndex - maxVisible + 1
		}
		end := min(len(items), start+maxVisible)

		for i := start; i < end; i++ {
			item := items[i]
			prioBadge := v.renderPriorityBadge(item.Priority)
			statusIcon := "[ ]"
			if item.Completed {
				statusIcon = "[x]"
			}

			title := item.Title
			maxTitleLen := rightWidth - 22
			if maxTitleLen > 3 && len(title) > maxTitleLen {
				title = title[:maxTitleLen-3] + "..."
			}

			line := fmt.Sprintf("%s %s %s", prioBadge, statusIcon, title)
			if i == v.backlogIndex {
				backSB.WriteString(v.styles.ListItemSelected.Width(max(10, rightWidth-4)).Render("▸ " + line))
			} else {
				backSB.WriteString(v.styles.ListItem.Width(max(10, rightWidth-4)).Render("  " + line))
			}
			backSB.WriteString("\n")
		}
		if end < len(items) || start > 0 {
			backSB.WriteString(lipgloss.NewStyle().Foreground(v.styles.Theme.TextMuted).
				Render(fmt.Sprintf("  (%d–%d of %d)\n", start+1, end, len(items))))
		}

		if v.backlogIndex < len(items) {
			curItem := items[v.backlogIndex]
			backSB.WriteString("\n" + lipgloss.NewStyle().Foreground(v.styles.Theme.Border).Render(strings.Repeat("─", max(1, rightWidth-6))) + "\n")
			backSB.WriteString(lipgloss.NewStyle().Bold(true).Foreground(v.styles.Theme.Primary).Render("🔍 Task Details:\n"))
			desc := curItem.Description
			if desc == "" {
				desc = curItem.Title
			}
			backSB.WriteString(lipgloss.NewStyle().Foreground(v.styles.Theme.Text).Width(max(10, rightWidth-4)).Render(desc) + "\n")
		}
	} else {
		backSB.WriteString(lipgloss.NewStyle().Foreground(v.styles.Theme.TextMuted).Render("No open tasks in active backlog.\n"))
	}

	if v.customWidgetText != "" {
		backSB.WriteString("\n" + lipgloss.NewStyle().Foreground(v.styles.Theme.Border).Render(strings.Repeat("─", max(1, rightWidth-6))) + "\n")
		backSB.WriteString(lipgloss.NewStyle().Bold(true).Foreground(v.styles.Theme.Info).Render("🤖 AI Command Center Component:\n"))
		backSB.WriteString(lipgloss.NewStyle().Foreground(v.styles.Theme.Text).Render(v.customWidgetText) + "\n")
	}
}

func (v *DashboardView) taxonomyStyle(id string, targetMin, targetMax float64) (string, string, lipgloss.Color) {
	upper := strings.ToUpper(strings.TrimSpace(id))
	target := ""
	if targetMin > 0 {
		target = fmt.Sprintf(">= %.0f%%", targetMin)
	} else if targetMax > 0 {
		target = fmt.Sprintf("<= %.0f%%", targetMax)
	}

	switch {
	case strings.HasPrefix(upper, "A"):
		if target == "" {
			target = ">= 30%"
		}
		return "[A] Strategic    ", target, v.styles.Theme.Primary
	case strings.HasPrefix(upper, "B"):
		if target == "" {
			target = ">= 30%"
		}
		return "[B] Local Impact ", target, v.styles.Theme.Info
	case strings.HasPrefix(upper, "C"):
		if target == "" {
			target = "<= 30%"
		}
		return "[C] Glue Work    ", target, v.styles.Theme.Secondary
	case strings.HasPrefix(upper, "E"):
		if target == "" {
			target = "<= 10%"
		}
		return "[E] Overhead     ", target, v.styles.Theme.Warning
	default:
		return fmt.Sprintf("[%-1s] %-13s", id[:min(1, len(id))], id), target, v.styles.Theme.Text
	}
}

func firstPercentage(m map[string]float64, keys ...string) float64 {
	for _, k := range keys {
		if val, ok := m[k]; ok {
			return val
		}
	}
	return 0
}

func truncateText(s string, maxLen int) string {
	if maxLen <= 3 || len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func (v *DashboardView) renderProgressBar(label string, pct float64, target string, color lipgloss.Color, maxWidth int) string {
	barWidth := maxWidth - 34
	if barWidth < 10 {
		barWidth = 10
	}

	filled := int((pct / 100.0) * float64(barWidth))
	if filled > barWidth {
		filled = barWidth
	}
	if filled < 0 {
		filled = 0
	}

	fillStr := strings.Repeat("█", filled)
	emptyStr := strings.Repeat("░", barWidth-filled)

	barRendered := lipgloss.NewStyle().Foreground(color).Render(fillStr) +
		lipgloss.NewStyle().Foreground(v.styles.Theme.Border).Render(emptyStr)

	return fmt.Sprintf(" %s  %s  %5.1f%% (Target %s)",
		lipgloss.NewStyle().Bold(true).Render(label),
		barRendered,
		pct,
		target,
	)
}

func (v *DashboardView) renderPriorityBadge(prio string) string {
	switch prio {
	case "P0":
		return lipgloss.NewStyle().Background(v.styles.Theme.Error).Foreground(v.styles.Theme.TextDark).Bold(true).Padding(0, 1).Render("P0")
	case "P1":
		return lipgloss.NewStyle().Background(v.styles.Theme.Warning).Foreground(v.styles.Theme.TextDark).Bold(true).Padding(0, 1).Render("P1")
	case "P2":
		return lipgloss.NewStyle().Background(v.styles.Theme.Info).Foreground(v.styles.Theme.TextDark).Bold(true).Padding(0, 1).Render("P2")
	default:
		return lipgloss.NewStyle().Background(v.styles.Theme.Border).Foreground(v.styles.Theme.Text).Padding(0, 1).Render(prio)
	}
}
