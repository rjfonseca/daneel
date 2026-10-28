//nolint:revive // package comment not required
package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/rjfonseca/daneel/internal/config"
	"github.com/rjfonseca/daneel/internal/routine"
	"github.com/rjfonseca/daneel/internal/telemetry"
)

// TelemetryView provides an interactive AI FinOps and operational telemetry dashboard.
type TelemetryView struct {
	vaultRoot          string
	styles             Styles
	store              *routine.SQLiteStore
	cfg                *config.Config
	windowDays         int
	modelUsage         []routine.ModelUsageAgg
	routineWeights     []routine.RoutineWeightAgg
	recentSessions     []routine.SessionTelemetryRecord
	commandMetrics     []routine.CommandTelemetryAgg
	totalCostUSD       float64
	totalInputTokens   int64
	totalCacheTokens   int64
	totalOutputTokens  int64
	cacheEfficiencyPct float64
	dailyBudgetUSD     float64
	monthlyBudgetUSD   float64
	selectedSection    int // 0: Models/Sessions, 1: Routines, 2: Commands
	selectedIndex      int
	loaded             bool
	width              int
	height             int
	statusMsg          string
}

// NewTelemetryView constructs a new TelemetryView instance.
func NewTelemetryView(vaultRoot string, styles Styles) *TelemetryView {
	return &TelemetryView{
		vaultRoot:  vaultRoot,
		styles:     styles,
		windowDays: 7,
	}
}

// SetStore injects a custom store (used for hermetic unit tests).
func (v *TelemetryView) SetStore(st *routine.SQLiteStore) {
	v.store = st
}

// SetConfig sets a custom config.
func (v *TelemetryView) SetConfig(cfg *config.Config) {
	v.cfg = cfg
}

// SetSize updates the view dimensions.
func (v *TelemetryView) SetSize(w, h int) {
	v.width = w
	v.height = h
}

// Init initializes the telemetry view.
func (v *TelemetryView) Init() tea.Cmd {
	return nil
}

// Refresh reloads aggregates and telemetry data from store.
func (v *TelemetryView) Refresh() {
	if v.windowDays <= 0 {
		v.windowDays = 7
	}

	var st *routine.SQLiteStore
	var shouldClose bool

	if v.store != nil {
		st = v.store
		shouldClose = false
	} else {
		opened, err := routine.OpenDefaultStore(v.vaultRoot)
		if err != nil {
			v.statusMsg = fmt.Sprintf("Error opening store: %v", err)
			return
		}
		st = opened
		shouldClose = true
	}

	if shouldClose {
		defer func() { _ = st.Close() }()
	}

	// Load configuration if not yet set
	if v.cfg == nil && v.vaultRoot != "" {
		cfg, err := config.LoadConfig(v.vaultRoot, "", false)
		if err == nil {
			v.cfg = cfg
		}
	}

	if v.cfg != nil {
		finOpsCfg := v.cfg.GetFinOpsConfig()
		v.dailyBudgetUSD = finOpsCfg.DailyBudgetUSD
		v.monthlyBudgetUSD = finOpsCfg.MonthlyBudgetUSD
	} else {
		defFinOps := config.DefaultFinOpsConfig()
		v.dailyBudgetUSD = defFinOps.DailyBudgetUSD
		v.monthlyBudgetUSD = defFinOps.MonthlyBudgetUSD
	}

	sinceDuration := time.Duration(v.windowDays) * 24 * time.Hour
	sinceUnix := time.Now().Add(-sinceDuration).Unix()

	// Live sync from transcripts only when in dynamic store mode
	if v.store == nil {
		parserOpts := telemetry.ParserOptions{
			Since:  sinceDuration,
			Config: v.cfg,
		}
		if v.vaultRoot != "" {
			brainDir := filepath.Join(v.vaultRoot, "brain")
			if fi, err := os.Stat(brainDir); err == nil && fi.IsDir() {
				parserOpts.BrainDir = brainDir
			}
		}
		_, _ = telemetry.SyncSessionsToStore(context.Background(), st, parserOpts)
	}

	ctx := context.Background()

	// 1. Model Usage Aggregates
	modelAggs, err := st.GetModelUsageAggregates(ctx, sinceUnix)
	if err == nil {
		v.modelUsage = modelAggs
	} else {
		v.modelUsage = nil
	}

	// 2. Routine Weight Aggregates
	routineWeights, err := st.GetRoutineWeightAggregates(ctx, sinceUnix)
	if err == nil {
		v.routineWeights = routineWeights
	} else {
		v.routineWeights = nil
	}

	// 3. Recent Sessions Telemetry
	recentSessions, err := st.GetRecentSessionsTelemetry(ctx, sinceUnix, 100)
	if err == nil {
		v.recentSessions = recentSessions
	} else {
		v.recentSessions = nil
	}

	// 4. Command Telemetry Aggregates
	cmdAggs, err := st.GetCommandAggregates(ctx, sinceUnix, 100)
	if err == nil {
		v.commandMetrics = cmdAggs
	} else {
		v.commandMetrics = nil
	}

	// Compute totals
	var totalIn, totalCache, totalOut int64
	var totalCost float64

	for _, s := range v.recentSessions {
		totalIn += s.InputTokens
		totalCache += s.CacheReadTokens
		totalOut += s.OutputTokens
		totalCost += s.EstimatedCostUSD
	}

	for _, rw := range v.routineWeights {
		totalIn += rw.TotalInputTokens
		totalCache += rw.TotalCacheTokens
		totalOut += rw.TotalOutputTokens
		totalCost += rw.TotalCostUSD
	}

	v.totalInputTokens = totalIn
	v.totalCacheTokens = totalCache
	v.totalOutputTokens = totalOut
	v.totalCostUSD = totalCost

	if totalIn+totalCache > 0 {
		v.cacheEfficiencyPct = (float64(totalCache) / float64(totalIn+totalCache)) * 100.0
	} else {
		v.cacheEfficiencyPct = 0
	}

	v.loaded = true
}

// Update handles user interaction in TelemetryView.
func (v *TelemetryView) Update(msg tea.Msg) (*TelemetryView, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "r":
			v.Refresh()
			return v, nil
		case "w":
			switch v.windowDays {
			case 7:
				v.windowDays = 30
			case 30:
				v.windowDays = 1
			default:
				v.windowDays = 7
			}
			v.Refresh()
			return v, nil
		case "h", "left":
			if v.selectedSection > 0 {
				v.selectedSection--
				v.selectedIndex = 0
			}
			return v, nil
		case "l", "right":
			if v.selectedSection < 2 {
				v.selectedSection++
				v.selectedIndex = 0
			}
			return v, nil
		case "j", "down":
			v.selectedIndex++
			maxLen := 0
			switch v.selectedSection {
			case 0:
				maxLen = len(v.recentSessions)
			case 1:
				maxLen = len(v.routineWeights)
			case 2:
				maxLen = len(v.commandMetrics)
			}
			if maxLen > 0 && v.selectedIndex >= maxLen {
				v.selectedIndex = maxLen - 1
			}
			return v, nil
		case "k", "up":
			if v.selectedIndex > 0 {
				v.selectedIndex--
			}
			return v, nil
		}
	}
	return v, nil
}

// View renders the dual-pane FinOps & Vault Primitives monitor.
func (v *TelemetryView) View() string {
	if !v.loaded {
		v.Refresh()
	}

	leftW, rightW, _ := CalculateLayoutDimensions(v.width, v.height, 0, 0.50)

	// ==================== LEFT PANE ====================
	var leftSB strings.Builder
	titleLeft := fmt.Sprintf("💰 AI FinOps, Budget & Model Matrix [Last %dd]", v.windowDays)
	leftSB.WriteString(v.styles.PanelTitle.Render(titleLeft) + "\n\n")

	// 1. Budget Gauges & Totals
	costStr := fmt.Sprintf("$%.4f", v.totalCostUSD)
	if v.totalCostUSD >= 0.01 {
		costStr = fmt.Sprintf("$%.3f", v.totalCostUSD)
	}

	fmt.Fprintf(&leftSB, "  • Total AI Spend: %s\n",
		lipgloss.NewStyle().Bold(true).Foreground(v.styles.Theme.Primary).Render(costStr))

	if v.dailyBudgetUSD > 0 {
		fmt.Fprintf(&leftSB, "  • Daily Budget:   %s\n",
			v.renderBudgetBar(v.totalCostUSD, v.dailyBudgetUSD, leftW-20))
	}
	if v.monthlyBudgetUSD > 0 {
		fmt.Fprintf(&leftSB, "  • Monthly Budget: %s\n",
			v.renderBudgetBar(v.totalCostUSD, v.monthlyBudgetUSD, leftW-20))
	}

	tokenSummary := fmt.Sprintf("In: %s  •  Cache: %s  •  Out: %s",
		formatTokens(v.totalInputTokens),
		formatTokens(v.totalCacheTokens),
		formatTokens(v.totalOutputTokens),
	)
	fmt.Fprintf(&leftSB, "  • Total Tokens:   %s\n", tokenSummary)

	cacheColor := v.styles.Theme.Success
	if v.cacheEfficiencyPct < 30 {
		cacheColor = v.styles.Theme.Warning
	}
	fmt.Fprintf(&leftSB, "  • Cache Hit Rate: %s\n",
		lipgloss.NewStyle().Bold(true).Foreground(cacheColor).Render(fmt.Sprintf("%.1f%%", v.cacheEfficiencyPct)))

	sepLeft := lipgloss.NewStyle().Foreground(v.styles.Theme.Border).Render(strings.Repeat("─", leftW-4))
	leftSB.WriteString("\n" + sepLeft + "\n")

	// 2. Model & Agent Breakdown Table
	leftSB.WriteString(lipgloss.NewStyle().Bold(true).Foreground(v.styles.Theme.Info).Render("📊 Model & Agent Breakdown:") + "\n")
	fmt.Fprintf(&leftSB, "  %-16s %-12s %8s %10s %9s\n", "Model", "Agent", "Sess/Sub", "Tokens", "Cost $USD")

	if len(v.modelUsage) == 0 {
		leftSB.WriteString(lipgloss.NewStyle().Foreground(v.styles.Theme.TextMuted).Render("  No model usage data recorded in this window.\n"))
	} else {
		for _, m := range v.modelUsage {
			mCost := fmt.Sprintf("$%.4f", m.EstimatedCostUSD)
			if m.EstimatedCostUSD >= 0.01 {
				mCost = fmt.Sprintf("$%.3f", m.EstimatedCostUSD)
			}
			totTokens := m.InputTokens + m.CacheReadTokens + m.OutputTokens
			sessSub := fmt.Sprintf("%d/%d", m.SessionsCount, m.SubagentsCount)

			modelName := m.Model
			if len(modelName) > 16 {
				modelName = modelName[:15] + "…"
			}
			agentName := m.Agent
			if len(agentName) > 12 {
				agentName = agentName[:11] + "…"
			}

			row := fmt.Sprintf("  %-16s %-12s %8s %10s %9s",
				modelName, agentName, sessSub, formatTokens(totTokens), mCost)
			leftSB.WriteString(row + "\n")
		}
	}

	leftSB.WriteString(sepLeft + "\n")

	// 3. Recent AI Sessions & Subagents
	leftSB.WriteString(lipgloss.NewStyle().Bold(true).Foreground(v.styles.Theme.Secondary).Render("🤖 Recent AI Sessions & Subagents:") + "\n")

	if len(v.recentSessions) == 0 {
		leftSB.WriteString(lipgloss.NewStyle().Foreground(v.styles.Theme.TextMuted).Render("  No recent AI sessions captured.\n"))
	} else {
		maxSessions := 7
		if len(v.recentSessions) < maxSessions {
			maxSessions = len(v.recentSessions)
		}
		for i := 0; i < maxSessions; i++ {
			s := v.recentSessions[i]
			isSub := s.ParentSessionID != ""
			prefix := "• "
			indent := "  "
			if isSub {
				prefix = "↳ "
				indent = "    "
			}

			sCost := fmt.Sprintf("$%.4f", s.EstimatedCostUSD)
			if s.EstimatedCostUSD >= 0.01 {
				sCost = fmt.Sprintf("$%.3f", s.EstimatedCostUSD)
			}

			sid := s.SessionID
			if len(sid) > 14 {
				sid = sid[:13] + "…"
			}
			model := s.Model
			if len(model) > 14 {
				model = model[:13] + "…"
			}

			line := fmt.Sprintf("%s%-14s %-14s %2dt %1dsub %s",
				prefix, sid, model, s.TurnsCount, s.SubagentsCount, sCost)

			if s.SkillsUsed != "" {
				skills := s.SkillsUsed
				if len(skills) > 18 {
					skills = skills[:17] + "…"
				}
				line += fmt.Sprintf(" [%s]", skills)
			}

			if v.selectedSection == 0 && i == v.selectedIndex {
				leftSB.WriteString(v.styles.ListItemSelected.Width(leftW-4).Render("▸ "+indent+line) + "\n")
			} else {
				leftSB.WriteString(indent + line + "\n")
			}
		}
	}

	// ==================== RIGHT PANE ====================
	var rightSB strings.Builder
	titleRight := "⚙️ Routine Weights & ⚡ Vault Primitives"
	rightSB.WriteString(v.styles.PanelTitle.Render(titleRight) + "\n\n")

	// 1. Heaviest Routines Leaderboard
	rightSB.WriteString(lipgloss.NewStyle().Bold(true).Foreground(v.styles.Theme.Primary).Render("⚙️ Heaviest Routines Leaderboard:") + "\n")
	fmt.Fprintf(&rightSB, "  %-14s %5s %12s %6s %10s %9s\n", "Routine", "Runs", "Avg/Max ms", "Items", "Tokens", "Cost $USD")

	if len(v.routineWeights) == 0 {
		rightSB.WriteString(lipgloss.NewStyle().Foreground(v.styles.Theme.TextMuted).Render("  No routine executions recorded in this window.\n"))
	} else {
		for i, rw := range v.routineWeights {
			rCost := fmt.Sprintf("$%.4f", rw.TotalCostUSD)
			if rw.TotalCostUSD >= 0.01 {
				rCost = fmt.Sprintf("$%.3f", rw.TotalCostUSD)
			}
			lat := fmt.Sprintf("%.0f/%dms", rw.AvgDurationMs, rw.MaxDurationMs)
			totTokens := rw.TotalInputTokens + rw.TotalCacheTokens + rw.TotalOutputTokens

			rName := rw.RoutineName
			if len(rName) > 14 {
				rName = rName[:13] + "…"
			}

			line := fmt.Sprintf("%-14s %5d %12s %6d %10s %9s",
				rName, rw.TotalRuns, lat, rw.TotalItems, formatTokens(totTokens), rCost)

			if v.selectedSection == 1 && i == v.selectedIndex {
				rightSB.WriteString(v.styles.ListItemSelected.Width(rightW-4).Render("▸   "+line) + "\n")
			} else {
				rightSB.WriteString("    " + line + "\n")
			}
		}
	}

	sepRight := lipgloss.NewStyle().Foreground(v.styles.Theme.Border).Render(strings.Repeat("─", rightW-4))
	rightSB.WriteString("\n" + sepRight + "\n")

	// 2. CLI & MCP Primitives Latency
	rightSB.WriteString(lipgloss.NewStyle().Bold(true).Foreground(v.styles.Theme.Info).Render("⚡ CLI & MCP Primitives Latency:") + "\n")
	fmt.Fprintf(&rightSB, "  %-4s %-16s %6s %13s %6s\n", "Surf", "Command", "Calls", "Avg/Max ms", "Errors")

	if len(v.commandMetrics) == 0 {
		rightSB.WriteString(lipgloss.NewStyle().Foreground(v.styles.Theme.TextMuted).Render("  No command invocations recorded in this window.\n"))
	} else {
		for i, cmd := range v.commandMetrics {
			lat := fmt.Sprintf("%.1f/%dms", cmd.AvgDurationMs, cmd.MaxDurationMs)
			cmdName := cmd.Command
			if len(cmdName) > 16 {
				cmdName = cmdName[:15] + "…"
			}

			line := fmt.Sprintf("%-4s %-16s %6d %13s %6d",
				cmd.Surface, cmdName, cmd.TotalCalls, lat, cmd.ErrorCalls)

			if v.selectedSection == 2 && i == v.selectedIndex {
				rightSB.WriteString(v.styles.ListItemSelected.Width(rightW-4).Render("▸   "+line) + "\n")
			} else {
				rightSB.WriteString("    " + line + "\n")
			}
		}
	}

	// 3. Footer hints
	rightSB.WriteString("\n" + sepRight + "\n")
	hints := lipgloss.NewStyle().Foreground(v.styles.Theme.TextMuted).Render("Key hints: [r] Refresh • [w] Window (1d/7d/30d) • [h/l] Pane • [j/k] Scroll")
	rightSB.WriteString("  " + hints + "\n")

	leftStyle := v.styles.Panel
	rightStyle := v.styles.Panel
	if v.selectedSection == 0 {
		leftStyle = v.styles.PanelFocused
	} else {
		rightStyle = v.styles.PanelFocused
	}

	return RenderSplitLayout(SplitLayoutConfig{
		Width:        v.width,
		Height:       v.height,
		LeftRatio:    0.50,
		TopBar:       "",
		LeftContent:  leftSB.String(),
		RightContent: rightSB.String(),
		LeftStyle:    leftStyle,
		RightStyle:   rightStyle,
	})
}

func (v *TelemetryView) renderBudgetBar(spend, budget float64, width int) string {
	if budget <= 0 {
		return fmt.Sprintf("$%.2f (no limit)", spend)
	}
	pct := (spend / budget) * 100.0
	barWidth := width - 24
	if barWidth < 6 {
		barWidth = 6
	}

	filled := int((pct / 100.0) * float64(barWidth))
	if filled > barWidth {
		filled = barWidth
	}
	if filled < 0 {
		filled = 0
	}

	barColor := v.styles.Theme.Success
	if pct >= 80 {
		barColor = v.styles.Theme.Warning
	}
	if pct >= 100 {
		barColor = v.styles.Theme.Error
	}

	fillStr := strings.Repeat("█", filled)
	emptyStr := strings.Repeat("░", barWidth-filled)

	barRendered := lipgloss.NewStyle().Foreground(barColor).Render(fillStr) +
		lipgloss.NewStyle().Foreground(v.styles.Theme.Border).Render(emptyStr)

	return fmt.Sprintf("$%.2f / $%.2f [%s] %5.1f%%", spend, budget, barRendered, pct)
}

func formatTokens(tokens int64) string {
	if tokens >= 1_000_000 {
		return fmt.Sprintf("%.2fM", float64(tokens)/1_000_000.0)
	}
	if tokens >= 1_000 {
		return fmt.Sprintf("%.1fk", float64(tokens)/1_000.0)
	}
	return fmt.Sprintf("%d", tokens)
}
