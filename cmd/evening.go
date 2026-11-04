//nolint:revive // package comment not required
package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/rjfonseca/daneel/internal/ops/daily"
)

var eveningDate string

var eveningCmd = &cobra.Command{
	Use:   "evening",
	Short: "Reconciles explicit Staff Engineering time annotations from the daily log into YAML frontmatter",
	RunE: func(cmd *cobra.Command, args []string) error { //nolint:revive
		rep, err := daily.RunEvening(cfg.VaultPath, eveningDate, cfg)
		if err != nil {
			return fmt.Errorf("evening copilot reconciliation failed: %w", err)
		}

		if jsonOut {
			data, err := json.MarshalIndent(rep, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(data))
			return nil
		}

		titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
		labelStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
		badgeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
		dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))

		status := "frontmatter updated"
		if !rep.Updated {
			status = "preserved existing frontmatter (no inline annotations)"
		}

		fmt.Printf("%s %s (%s)\n\n", titleStyle.Render("🌇 Copilot Evening Reconciliation —"), labelStyle.Render(rep.Date), badgeStyle.Render(status))
		fmt.Printf("• Target File: %s\n", rep.FilePath)
		fmt.Printf("• Total Hours Tracked: %.2fh\n\n", rep.TotalHours)

		fmt.Println(dimStyle.Render("Staff Archetype Breakdown:"))
		fmt.Printf("  • [A] Strategic (tempo-A_Estrategico):   %.2fh\n", rep.HoursByCategory["A"])
		fmt.Printf("  • [B] Local Impact (tempo-B_ImpactoLocal): %.2fh\n", rep.HoursByCategory["B"])
		fmt.Printf("  • [C] Glue Work (tempo-C_GlueWork):       %.2fh\n", rep.HoursByCategory["C"])
		fmt.Printf("  • [D] Low Impact (tempo-D_LowImpact):     %.2fh\n", rep.HoursByCategory["D"])
		fmt.Printf("  • [E] Overhead (tempo-E_Overhead):        %.2fh\n", rep.HoursByCategory["E"])

		return nil
	},
}

func init() {
	eveningCmd.Flags().StringVarP(&eveningDate, "date", "d", "", "ISO reference date (YYYY-MM-DD)")
	RootCmd.AddCommand(eveningCmd)
}
