//nolint:revive // package comment not required
package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/rjfonseca/daneel/internal/dream"
	"github.com/rjfonseca/daneel/internal/ops/backlog"
	"github.com/rjfonseca/daneel/internal/vault"
)

// DashboardView presents the Staff Engineering Radar, Telemetry bars, and Active Backlog.
type DashboardView struct {
	vaultRoot        string
	styles           Styles
	temporalData     *dream.TemporalWindowData
	stratRadar       *dream.StrategicRadar
	backlogRep       *backlog.BacklogReport
	vaultMetrics     *vault.VaultMetrics
	backlogIndex     int
	width            int
	height           int
	customWidgetText string
}

// NewDashboardView creates a new dashboard view.
func NewDashboardView(vaultRoot string, styles Styles) *DashboardView {
	dv := &DashboardView{
		vaultRoot:    vaultRoot,
		styles:       styles,
		backlogIndex: 0,
	}
	dv.Refresh()
	return dv
}

// Refresh reloads telemetry and backlog from the vault.
func (v *DashboardView) Refresh() {
	today := time.Now().Format("2006-01-02")
	learnings := dream.LoadLearnings(v.vaultRoot)

	temp, _ := dream.AnalyzeTemporalWindow(v.vaultRoot, today, 7)
	v.temporalData = temp

	if temp != nil {
		v.stratRadar = dream.CrossSynthesizeTechBusiness(v.vaultRoot, today, temp, nil, learnings)
	}

	bRep, _ := backlog.GetActiveBacklog(v.vaultRoot)
	v.backlogRep = bRep

	vm := vault.CalculateVaultMetrics(v.vaultRoot)
	v.vaultMetrics = &vm
}

// Init initializes the view.
func (v *DashboardView) Init() tea.Cmd {
	return nil
}

// Update handles tea messages for the dashboard.
func (v *DashboardView) Update(msg tea.Msg) (*DashboardView, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "r":
			v.Refresh()
			return v, nil
		case "up", "k":
			if v.backlogIndex > 0 {
				v.backlogIndex--
			}
			return v, nil
		case "down", "j":
			if v.backlogRep != nil && v.backlogIndex < len(v.backlogRep.Items)-1 {
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
	if v.temporalData == nil {
		return "Loading Staff Telemetry & Radar..."
	}

	leftWidth, rightWidth, _ := CalculateLayoutDimensions(v.width, v.height, 0, 0.50)

	var teleSB strings.Builder
	fmt.Fprintf(&teleSB, "%s (7-Day Window — %.1fh total across %d days):\n\n",
		v.styles.PanelTitle.Render("📊 Staff Engineering Telemetry"),
		v.temporalData.TotalHours,
		v.temporalData.DaysWithLogs)

	pctA := v.temporalData.Percentages["A"]
	pctB := v.temporalData.Percentages["B"]
	pctC := v.temporalData.Percentages["C"]
	pctE := v.temporalData.Percentages["E"]

	teleSB.WriteString(v.renderProgressBar("[A] Strategic    ", pctA, ">= 30%", v.styles.Theme.Primary, leftWidth-8))
	teleSB.WriteString("\n")
	teleSB.WriteString(v.renderProgressBar("[B] Local Impact ", pctB, ">= 30%", v.styles.Theme.Info, leftWidth-8))
	teleSB.WriteString("\n")
	teleSB.WriteString(v.renderProgressBar("[C] Glue Work    ", pctC, "<= 30%", v.styles.Theme.Secondary, leftWidth-8))
	teleSB.WriteString("\n")
	teleSB.WriteString(v.renderProgressBar("[E] Overhead     ", pctE, "<= 10%", v.styles.Theme.Warning, leftWidth-8))
	teleSB.WriteString("\n\n")

	if len(v.temporalData.DriftAlerts) > 0 {
		for _, da := range v.temporalData.DriftAlerts {
			teleSB.WriteString(lipgloss.NewStyle().Foreground(v.styles.Theme.Error).Bold(true).Render("⚠️ Staff Drift: ") + da + "\n")
		}
	} else {
		teleSB.WriteString(lipgloss.NewStyle().Foreground(v.styles.Theme.Success).Bold(true).Render("✅ Healthy time allocation aligned with Staff Engineering targets.\n"))
	}

	if v.stratRadar != nil && len(v.stratRadar.TractionSummary) > 0 {
		teleSB.WriteString("\n" + v.styles.PanelTitle.Render("🏢 Corporate Strategic Radar:") + "\n")
		limit := 4
		if len(v.stratRadar.TractionSummary) < limit {
			limit = len(v.stratRadar.TractionSummary)
		}
		for _, tr := range v.stratRadar.TractionSummary[:limit] {
			fmt.Fprintf(&teleSB, "  • %-22s %s (%d mentions)\n", tr.Initiative, tr.Status, tr.Count)
		}
	}

	if len(v.temporalData.FrictionSignals) > 0 {
		teleSB.WriteString("\n" + v.styles.PanelTitle.Render("⚠️ Technical Debt & Friction Signals:") + "\n")
		for _, fs := range v.temporalData.FrictionSignals[:2] {
			fmt.Fprintf(&teleSB, "  • [%s] %s\n", fs.Date, fs.Line)
		}
	}

	if v.vaultMetrics != nil {
		teleSB.WriteString("\n" + v.styles.PanelTitle.Render("🧠 Second Brain Health & Metrics:") + "\n")
		fmt.Fprintf(&teleSB, "  • Total Notes: %s\n", lipgloss.NewStyle().Bold(true).Render(fmt.Sprintf("%d", v.vaultMetrics.TotalNotes)))
		fmt.Fprintf(&teleSB, "  • Total Links: %d (%.2f links/note)\n", v.vaultMetrics.TotalLinks, v.vaultMetrics.LinksPerNote)

		orphanStyle := lipgloss.NewStyle().Foreground(v.styles.Theme.Success)
		if v.vaultMetrics.OrphanNotes > 10 {
			orphanStyle = lipgloss.NewStyle().Foreground(v.styles.Theme.Warning)
		}
		fmt.Fprintf(&teleSB, "  • Orphan Notes: %s\n", orphanStyle.Render(fmt.Sprintf("%d", v.vaultMetrics.OrphanNotes)))
	}

	var backSB strings.Builder
	backSB.WriteString(v.styles.PanelTitle.Render("📌 Active Backlog & Current Focus") + "\n\n")

	if v.backlogRep != nil && len(v.backlogRep.Items) > 0 {
		for i, item := range v.backlogRep.Items {
			prioBadge := v.renderPriorityBadge(item.Priority)
			statusIcon := "[ ]"
			if item.Completed {
				statusIcon = "[x]"
			}

			title := item.Title
			if len(title) > rightWidth-22 {
				title = title[:rightWidth-25] + "..."
			}

			line := fmt.Sprintf("%s %s %s", prioBadge, statusIcon, title)
			if i == v.backlogIndex {
				backSB.WriteString(v.styles.ListItemSelected.Width(rightWidth - 4).Render("▸ " + line))
			} else {
				backSB.WriteString(v.styles.ListItem.Width(rightWidth - 4).Render("  " + line))
			}
			backSB.WriteString("\n")
		}

		if v.backlogIndex < len(v.backlogRep.Items) {
			curItem := v.backlogRep.Items[v.backlogIndex]
			backSB.WriteString("\n" + lipgloss.NewStyle().Foreground(v.styles.Theme.Border).Render(strings.Repeat("─", rightWidth-6)) + "\n")
			backSB.WriteString(lipgloss.NewStyle().Bold(true).Foreground(v.styles.Theme.Primary).Render("🔍 Task Details:\n"))
			backSB.WriteString(lipgloss.NewStyle().Foreground(v.styles.Theme.Text).Render(curItem.Description) + "\n")
		}
	} else {
		backSB.WriteString(lipgloss.NewStyle().Foreground(v.styles.Theme.TextMuted).Render("No active tasks in backlog.\n"))
	}

	if v.customWidgetText != "" {
		backSB.WriteString("\n" + lipgloss.NewStyle().Foreground(v.styles.Theme.Border).Render(strings.Repeat("─", rightWidth-6)) + "\n")
		backSB.WriteString(lipgloss.NewStyle().Bold(true).Foreground(v.styles.Theme.Info).Render("🤖 AI Command Center Component:\n"))
		backSB.WriteString(lipgloss.NewStyle().Foreground(v.styles.Theme.Text).Render(v.customWidgetText) + "\n")
	}

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
