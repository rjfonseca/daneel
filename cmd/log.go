//nolint:revive // package comment not required
package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/spf13/cobra"

	"github.com/rjfonseca/daneel/internal/ops/tracking"
)

var (
	logDate    string
	logRecent  int
	logWeeks   int
	logMonthly bool
)

var logCmd = &cobra.Command{
	Use:     "log",
	Aliases: []string{"time", "daily"},
	Short:   "Displays activity logs and time tracking metrics from daily logs",
	RunE: func(cmd *cobra.Command, args []string) error { //nolint:revive
		// 1. Rolling weekly comparative breakdown
		if cmd.Flags().Changed("weeks") || logWeeks > 0 {
			numWeeks := logWeeks
			if numWeeks <= 0 {
				numWeeks = 4
			}
			weeks, err := tracking.AggregateByWeek(cfg.VaultPath, numWeeks)
			if err != nil {
				return fmt.Errorf("failed to aggregate weekly telemetry: %w", err)
			}

			if jsonOut {
				bytes, err := json.MarshalIndent(weeks, "", "  ")
				if err != nil {
					return err
				}
				fmt.Println(string(bytes))
				return nil
			}

			renderPeriodsTable(fmt.Sprintf("📊 Rolling Weekly Staff Telemetry (Last %d Weeks)", len(weeks)), weeks)
			return nil
		}

		// 2. Historical monthly trend
		if logMonthly {
			months, err := tracking.AggregateByMonth(cfg.VaultPath, 6)
			if err != nil {
				return fmt.Errorf("failed to aggregate monthly telemetry: %w", err)
			}

			if jsonOut {
				bytes, err := json.MarshalIndent(months, "", "  ")
				if err != nil {
					return err
				}
				fmt.Println(string(bytes))
				return nil
			}

			renderPeriodsTable(fmt.Sprintf("📈 Historical Monthly Staff Telemetry (Last %d Months)", len(months)), months)
			return nil
		}

		// 3. Recent daily logs list
		if logRecent > 0 {
			logs, err := tracking.ListDailyLogs(cfg.VaultPath, logRecent)
			if err != nil {
				return fmt.Errorf("failed to list daily logs: %w", err)
			}

			if jsonOut {
				bytes, err := json.MarshalIndent(logs, "", "  ")
				if err != nil {
					return err
				}
				fmt.Println(string(bytes))
				return nil
			}

			titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
			fmt.Println(titleStyle.Render(fmt.Sprintf("📋 History of the Last %d Daily Logs:", len(logs))))
			for _, l := range logs {
				fmt.Printf("• %s | Total: %.2fh  (A: %.2fh | B: %.2fh | C: %.2fh | D: %.2fh | E: %.2fh)\n",
					l.Date, l.TotalHours, l.TempoA, l.TempoB, l.TempoC, l.TempoD, l.TempoE)
			}
			return nil
		}

		// 4. Single daily log detail
		dailyLog, err := tracking.GetDailyLog(cfg.VaultPath, logDate)
		if err != nil {
			return fmt.Errorf("daily log not found for date '%s': %w", logDate, err)
		}

		if jsonOut {
			bytes, err := json.MarshalIndent(dailyLog, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(bytes))
			return nil
		}

		titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
		labelStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
		sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("244"))

		fmt.Println(titleStyle.Render(fmt.Sprintf("📋 Daily Log: %s", dailyLog.Date)))
		fmt.Printf("• Total Registered Hours: %s\n\n", labelStyle.Render(fmt.Sprintf("%.2fh", dailyLog.TotalHours)))

		fmt.Println(sectionStyle.Render("📊 Taxonomy Breakdown:"))
		fmt.Printf("  • [A] Strategic:        %.2fh\n", dailyLog.TempoA)
		fmt.Printf("  • [B] Local Impact:     %.2fh\n", dailyLog.TempoB)
		fmt.Printf("  • [C] Leadership Glue:  %.2fh\n", dailyLog.TempoC)
		fmt.Printf("  • [D] Low Impact:       %.2fh\n", dailyLog.TempoD)
		fmt.Printf("  • [E] Overhead:         %.2fh\n\n", dailyLog.TempoE)

		if len(dailyLog.Activities) > 0 {
			fmt.Println(sectionStyle.Render("🗓️ Key Activities:"))
			for _, act := range dailyLog.Activities {
				fmt.Printf("  • %s\n", act)
			}
			fmt.Println()
		}

		if len(dailyLog.Alignments) > 0 {
			fmt.Println(sectionStyle.Render("👥 Alignments & Interactions:"))
			for _, al := range dailyLog.Alignments {
				fmt.Printf("  • %s\n", al)
			}
			fmt.Println()
		}

		if len(dailyLog.Discoveries) > 0 {
			fmt.Println(sectionStyle.Render("🧠 Knowledge & Discoveries:"))
			for _, disc := range dailyLog.Discoveries {
				fmt.Printf("  • %s\n", disc)
			}
			fmt.Println()
		}

		return nil
	},
}

func renderPeriodsTable(title string, periods []tracking.PeriodSummary) {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	legendStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Italic(true)

	fmt.Println(titleStyle.Render(title))
	if len(periods) == 0 {
		fmt.Println("No telemetry periods found.")
		return
	}

	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("255")).Align(lipgloss.Center)
	cellStyle := lipgloss.NewStyle().Padding(0, 1)

	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("240"))).
		Headers("PERIOD", "DAYS", "[A] STRAT", "[B] LOCAL", "[C] GLUE", "[D] LOW", "[E] OVH", "TOTAL", "LEVERAGE", "DIAGNOSIS"). //nolint:misspell
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return headerStyle
			}
			s := cellStyle
			if row >= 0 && row < len(periods) {
				switch col {
				case 2: // A: Indigo
					return s.Foreground(lipgloss.Color("75"))
				case 3: // B: Cyan
					return s.Foreground(lipgloss.Color("86"))
				case 4: // C: Purple
					return s.Foreground(lipgloss.Color("141"))
				case 5: // D: Muted
					return s.Foreground(lipgloss.Color("244"))
				case 6: // E: Amber
					return s.Foreground(lipgloss.Color("214"))
				case 7: // Total: White bold
					return s.Bold(true).Foreground(lipgloss.Color("255"))
				case 8: // Leverage: Green or Yellow or Red
					rate := periods[row].LeverageRate
					if rate >= 0.50 {
						return s.Bold(true).Foreground(lipgloss.Color("82"))
					} else if rate >= 0.35 {
						return s.Bold(true).Foreground(lipgloss.Color("220"))
					}
					return s.Bold(true).Foreground(lipgloss.Color("196"))
				}
			}
			return s
		})

	for _, p := range periods {
		t.Row(
			p.Label,
			fmt.Sprintf("%dd", p.DaysCount),
			fmt.Sprintf("%.1fh", p.TempoA),
			fmt.Sprintf("%.1fh", p.TempoB),
			fmt.Sprintf("%.1fh", p.TempoC),
			fmt.Sprintf("%.1fh", p.TempoD),
			fmt.Sprintf("%.1fh", p.TempoE),
			fmt.Sprintf("%.1fh", p.TotalHours),
			fmt.Sprintf("%.1f%%", p.LeverageRate*100),
			p.Diagnosis,
		)
	}

	fmt.Println(t.Render())
	fmt.Println(legendStyle.Render("Targets: [A] Strat 35-45% | [B] Local 25-35% | [C] Glue 15-25% | [E] Ovh <15% | Leverage (A+C)/Total >= 50%")) //nolint:misspell
}

func init() {
	logCmd.Flags().StringVarP(&logDate, "date", "d", "today", "Daily log date (YYYY-MM-DD or 'today')")
	logCmd.Flags().IntVar(&logRecent, "recent", 0, "Display history for the last N days")
	logCmd.Flags().IntVarP(&logWeeks, "weeks", "w", 0, "Display rolling weekly comparative breakdown for N weeks (e.g. 4)")
	logCmd.Flags().BoolVarP(&logMonthly, "monthly", "m", false, "Display historical monthly telemetry trend")
	RootCmd.AddCommand(logCmd)
}
