//nolint:revive // package comment not required
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/rjfonseca/daneel/internal/routine"
	"github.com/rjfonseca/daneel/internal/telemetry"
)

var (
	telemetryDays       int
	telemetryRoutines   bool
	telemetrySessions   bool
	telemetryCommands   bool
	telemetrySyncLatest bool
)

// OperationalTelemetryReport represents the unified FinOps and usage telemetry payload.
type OperationalTelemetryReport struct {
	WindowDays           int                              `json:"window_days"`
	TotalCostUSD         float64                          `json:"total_cost_usd"`
	DailyBudgetUSD       float64                          `json:"daily_budget_usd"`
	MonthlyBudgetUSD     float64                          `json:"monthly_budget_usd"`
	TotalInputTokens     int64                            `json:"total_input_tokens"`
	TotalCacheReadTokens int64                            `json:"total_cache_read_tokens"`
	TotalOutputTokens    int64                            `json:"total_output_tokens"`
	CacheEfficiencyPct   float64                          `json:"cache_efficiency_pct"`
	TotalSessions        int                              `json:"total_sessions"`
	TotalSubagents       int                              `json:"total_subagents"`
	ModelUsage           []routine.ModelUsageAgg          `json:"model_usage"`
	RoutineWeights       []routine.RoutineWeightAgg       `json:"routine_weights"`
	RecentSessions       []routine.SessionTelemetryRecord `json:"recent_sessions"`
	CommandMetrics       []routine.CommandTelemetryAgg    `json:"command_metrics"`
}

var telemetryCmd = &cobra.Command{
	Use:     "telemetry",
	Aliases: []string{"finops", "stats", "metrics"},
	Short:   "Inspect Second Brain operational telemetry, routine weights, and AI FinOps token costs",
	Long: `📈 Daneel Telemetry & AI FinOps Deck

Inspect operational metrics, background routine weights, CLI & MCP tool invocation latency,
and cumulative token spend attributed across autonomous AI sessions and recursive subagents.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if telemetryDays <= 0 {
			telemetryDays = 7
		}

		vPath := ""
		if cfg != nil {
			vPath = cfg.VaultPath
		}

		store, err := routine.OpenDefaultStore(vPath)
		if err != nil {
			return fmt.Errorf("failed to open routine telemetry store: %w", err)
		}
		defer func() { _ = store.Close() }()

		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}

		sinceDuration := time.Duration(telemetryDays) * 24 * time.Hour
		sinceUnix := time.Now().Add(-sinceDuration).Unix()

		// Optional sync from transcripts
		if telemetrySyncLatest {
			parserOpts := telemetry.ParserOptions{
				Since:  sinceDuration,
				Config: cfg,
			}
			if vPath != "" {
				brainDir := filepath.Join(vPath, "brain")
				if fi, err := os.Stat(brainDir); err == nil && fi.IsDir() {
					parserOpts.BrainDir = brainDir
				}
			}
			_, _ = telemetry.SyncSessionsToStore(ctx, store, parserOpts)
		}

		// Retrieve data from store
		recentSessions, err := store.GetRecentSessionsTelemetry(ctx, sinceUnix, 100)
		if err != nil {
			recentSessions = []routine.SessionTelemetryRecord{}
		}

		modelAggs, err := store.GetModelUsageAggregates(ctx, sinceUnix)
		if err != nil {
			modelAggs = []routine.ModelUsageAgg{}
		}

		routineWeights, err := store.GetRoutineWeightAggregates(ctx, sinceUnix)
		if err != nil {
			routineWeights = []routine.RoutineWeightAgg{}
		}

		commandMetrics, err := store.GetCommandAggregates(ctx, sinceUnix, 100)
		if err != nil {
			commandMetrics = []routine.CommandTelemetryAgg{}
		}

		// Calculate FinOps totals
		var totalIn, totalCache, totalOut int64
		var totalCost float64
		var totalSubagents int

		for _, s := range recentSessions {
			totalIn += s.InputTokens
			totalCache += s.CacheReadTokens
			totalOut += s.OutputTokens
			totalCost += s.EstimatedCostUSD
			totalSubagents += s.SubagentsCount
		}

		cachePct := 0.0
		if totalIn+totalCache > 0 {
			cachePct = float64(totalCache) / float64(totalIn+totalCache) * 100.0
		}

		dailyBudget := 0.0
		monthlyBudget := 0.0
		if cfg != nil {
			finOpsCfg := cfg.GetFinOpsConfig()
			dailyBudget = finOpsCfg.DailyBudgetUSD
			monthlyBudget = finOpsCfg.MonthlyBudgetUSD
		}

		report := OperationalTelemetryReport{
			WindowDays:           telemetryDays,
			TotalCostUSD:         totalCost,
			DailyBudgetUSD:       dailyBudget,
			MonthlyBudgetUSD:     monthlyBudget,
			TotalInputTokens:     totalIn,
			TotalCacheReadTokens: totalCache,
			TotalOutputTokens:    totalOut,
			CacheEfficiencyPct:   cachePct,
			TotalSessions:        len(recentSessions),
			TotalSubagents:       totalSubagents,
			ModelUsage:           modelAggs,
			RoutineWeights:       routineWeights,
			RecentSessions:       recentSessions,
			CommandMetrics:       commandMetrics,
		}

		if jsonOut {
			data, err := json.MarshalIndent(report, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(data))
			return nil
		}

		renderTelemetryDashboard(report)
		return nil
	},
}

func renderTelemetryDashboard(r OperationalTelemetryReport) {
	showAll := !telemetryRoutines && !telemetrySessions && !telemetryCommands

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("75"))
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	valStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("220"))
	costStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("63")).
		Padding(1, 2).
		MarginBottom(1)

	var sb strings.Builder
	sb.WriteString(titleStyle.Render(fmt.Sprintf("📊 Operational Telemetry & AI FinOps Dashboard [%dd Window]\n", r.WindowDays)))
	sb.WriteString(strings.Repeat("─", 80) + "\n\n")

	// 1. FinOps & Budget Summary
	if showAll || telemetrySessions {
		sb.WriteString(headerStyle.Render("💰 AI FinOps & Budget Summary\n"))
		sb.WriteString(strings.Repeat("─", 40) + "\n")
		fmt.Fprintf(&sb, "  • %-22s %s\n", "Total Estimated Spend:", costStyle.Render(fmt.Sprintf("$%.4f USD", r.TotalCostUSD)))
		if r.DailyBudgetUSD > 0 {
			burnPct := (r.TotalCostUSD / float64(r.WindowDays)) / r.DailyBudgetUSD * 100.0
			fmt.Fprintf(&sb, "  • %-22s $%.2f USD/day (%s burn rate)\n", "Daily Budget:", r.DailyBudgetUSD, formatBurnRate(burnPct))
		}
		if r.MonthlyBudgetUSD > 0 {
			projMonth := (r.TotalCostUSD / float64(r.WindowDays)) * 30.0
			fmt.Fprintf(&sb, "  • %-22s $%.2f USD (Projected: $%.2f USD)\n", "Monthly Budget:", r.MonthlyBudgetUSD, projMonth)
		}
		fmt.Fprintf(&sb, "  • %-22s %d input / %d cache read / %d output\n", "Token Consumption:", r.TotalInputTokens, r.TotalCacheReadTokens, r.TotalOutputTokens)
		fmt.Fprintf(&sb, "  • %-22s %s\n", "Cache Efficiency:", valStyle.Render(fmt.Sprintf("%.1f%%", r.CacheEfficiencyPct)))
		fmt.Fprintf(&sb, "  • %-22s %d root sessions, %d subagents\n\n", "Agent Activity:", r.TotalSessions, r.TotalSubagents)

		if len(r.ModelUsage) > 0 {
			sb.WriteString(dimStyle.Render("  Model Breakdown:\n"))
			for _, m := range r.ModelUsage {
				fmt.Fprintf(&sb, "    - %-18s (%-12s): %3d sess, %3d subagents | %d in, %d cache, %d out | %s\n",
					m.Model, m.Agent, m.SessionsCount, m.SubagentsCount, m.InputTokens, m.CacheReadTokens, m.OutputTokens, costStyle.Render(fmt.Sprintf("$%.4f", m.EstimatedCostUSD)))
			}
			sb.WriteString("\n")
		}
	}

	// 2. Routine Weight & Cost Leaderboard
	if showAll || telemetryRoutines {
		sb.WriteString(headerStyle.Render("⚙️ Routine Weight & Cost Leaderboard\n"))
		sb.WriteString(strings.Repeat("─", 40) + "\n")
		if len(r.RoutineWeights) == 0 {
			sb.WriteString(dimStyle.Render("  No routine runs recorded in this window.\n\n"))
		} else {
			for _, rw := range r.RoutineWeights {
				fmt.Fprintf(&sb, "  • %-16s %3d runs (%d ok, %d fail) | avg %5.0fms (max %5dms) | %4d items | %s\n",
					rw.RoutineName, rw.TotalRuns, rw.SuccessRuns, rw.FailedRuns, rw.AvgDurationMs, rw.MaxDurationMs, rw.TotalItems, costStyle.Render(fmt.Sprintf("$%.4f", rw.TotalCostUSD)))
				if rw.TotalInputTokens > 0 {
					fmt.Fprintf(&sb, "    %s\n", dimStyle.Render(fmt.Sprintf("tokens: %d in / %d cache / %d out", rw.TotalInputTokens, rw.TotalCacheTokens, rw.TotalOutputTokens)))
				}
			}
			sb.WriteString("\n")
		}
	}

	// 3. Recent AI Sessions & Subagents
	if showAll || telemetrySessions {
		sb.WriteString(headerStyle.Render("🤖 Recent AI Sessions & Subagents\n"))
		sb.WriteString(strings.Repeat("─", 40) + "\n")
		if len(r.RecentSessions) == 0 {
			sb.WriteString(dimStyle.Render("  No recent AI sessions recorded.\n\n"))
		} else {
			count := 0
			for _, s := range r.RecentSessions {
				count++
				if count > 10 {
					break
				}
				prefix := s.SessionID
				if len(prefix) > 16 {
					prefix = prefix[:16] + "…"
				}
				skills := s.SkillsUsed
				if len(skills) > 28 {
					skills = skills[:28] + "…"
				}
				fmt.Fprintf(&sb, "  • %-18s [%-11s | %-16s] %2d turns, %d subagents | %s",
					prefix, s.Agent, s.Model, s.TurnsCount, s.SubagentsCount, costStyle.Render(fmt.Sprintf("$%.4f", s.EstimatedCostUSD)))
				if skills != "" {
					fmt.Fprintf(&sb, " | %s", dimStyle.Render(skills))
				}
				sb.WriteString("\n")
			}
			sb.WriteString("\n")
		}
	}

	// 4. Vault Primitives & MCP Latency
	if showAll || telemetryCommands {
		sb.WriteString(headerStyle.Render("⚡ Vault Primitives & MCP Latency\n"))
		sb.WriteString(strings.Repeat("─", 40) + "\n")
		if len(r.CommandMetrics) == 0 {
			sb.WriteString(dimStyle.Render("  No CLI or MCP command invocations recorded.\n\n"))
		} else {
			for _, c := range r.CommandMetrics {
				errRate := 0.0
				if c.TotalCalls > 0 {
					errRate = float64(c.ErrorCalls) / float64(c.TotalCalls) * 100.0
				}
				fmt.Fprintf(&sb, "  • [%-3s] %-20s %4d calls | avg %5.1fms (max %4dms) | %s\n",
					strings.ToUpper(c.Surface), c.Command, c.TotalCalls, c.AvgDurationMs, c.MaxDurationMs, formatErrorRate(errRate))
			}
			sb.WriteString("\n")
		}
	}

	fmt.Println(boxStyle.Render(sb.String()))
}

func formatBurnRate(pct float64) string {
	if pct > 100.0 {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Bold(true).Render(fmt.Sprintf("%.0f%% OVER BUDGET", pct))
	}
	if pct > 80.0 {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render(fmt.Sprintf("%.0f%%", pct))
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color("82")).Render(fmt.Sprintf("%.0f%%", pct))
}

func formatErrorRate(rate float64) string {
	if rate > 0.0 {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Render(fmt.Sprintf("%.1f%% err", rate))
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color("82")).Render("0% err")
}

func init() {
	telemetryCmd.Flags().IntVarP(&telemetryDays, "days", "d", 7, "Rolling window in days")
	telemetryCmd.Flags().BoolVar(&telemetryRoutines, "routines", false, "Show only routine weight & execution telemetry")
	telemetryCmd.Flags().BoolVar(&telemetrySessions, "sessions", false, "Show only AI sessions, models, and subagent token costs")
	telemetryCmd.Flags().BoolVar(&telemetrySessions, "finops", false, "Alias for --sessions: show AI token costs and models")
	telemetryCmd.Flags().BoolVar(&telemetryCommands, "commands", false, "Show only CLI & MCP primitive latency and call counts")
	telemetryCmd.Flags().BoolVar(&telemetryCommands, "primitives", false, "Alias for --commands: show CLI & MCP tool primitives")
	telemetryCmd.Flags().BoolVar(&telemetrySyncLatest, "sync", true, "Sync latest AI transcripts before displaying")

	RootCmd.AddCommand(telemetryCmd)
}
