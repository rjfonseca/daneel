//nolint:revive // package comment not required
package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/rjfonseca/daneel/internal/ops/daily"
)

var morningDate string

var morningCmd = &cobra.Command{
	Use:   "morning",
	Short: "Scaffolds the daily log with autonomous Dream Cycle handoff, inbox triage, and active P0/P1 priorities",
	RunE: func(cmd *cobra.Command, args []string) error { //nolint:revive
		rep, err := daily.RunMorning(cfg.VaultPath, morningDate, cfg)
		if err != nil {
			return fmt.Errorf("morning copilot routine failed: %w", err)
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
		sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("244"))
		badgeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
		warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214"))

		actionStatus := "scaffolded new log"
		if !rep.Created {
			actionStatus = "verified / synchronized existing log"
		}

		fmt.Printf("%s %s (%s)\n\n", titleStyle.Render("🌅 Copilot Morning Briefing —"), labelStyle.Render(rep.Date), badgeStyle.Render(actionStatus))
		fmt.Printf("• Target File: %s\n\n", rep.FilePath)

		if len(rep.AutonomousActions) > 0 {
			fmt.Println(sectionStyle.Render("🤖 Autonomous Actions & Healed Graph Links:"))
			for _, act := range rep.AutonomousActions {
				fmt.Printf("  • %s\n", act)
			}
			fmt.Println()
		}

		if len(rep.InboxItems) > 0 {
			fmt.Println(warnStyle.Render(fmt.Sprintf("📥 Inbox Items Requiring Human Direction (%d):", len(rep.InboxItems))))
			for _, item := range rep.InboxItems {
				fmt.Printf("  [ ] %s\n", item)
			}
			fmt.Println()
		}

		if len(rep.Alerts) > 0 {
			fmt.Println(warnStyle.Render(fmt.Sprintf("⚠️ Core Alerts Pending Attention (%d):", len(rep.Alerts))))
			for _, alert := range rep.Alerts {
				fmt.Printf("  ! %s\n", alert)
			}
			fmt.Println()
		}

		if len(rep.Suggestions) > 0 {
			fmt.Println(sectionStyle.Render("💡 Metacognition & Strategic Inquiries:"))
			for _, sug := range rep.Suggestions {
				fmt.Printf("  ? %s\n", sug)
			}
			fmt.Println()
		}

		if len(rep.BacklogP0) > 0 || len(rep.BacklogP1) > 0 {
			fmt.Println(sectionStyle.Render(fmt.Sprintf("🎯 Active Priorities (P0: %d | P1: %d):", len(rep.BacklogP0), len(rep.BacklogP1))))
			for _, p0 := range rep.BacklogP0 {
				fmt.Printf("  [P0] %s\n", p0)
			}
			for _, p1 := range rep.BacklogP1 {
				fmt.Printf("  [P1] %s\n", p1)
			}
			fmt.Println()
		}

		if len(rep.IngestedMeetings) > 0 {
			fmt.Println(sectionStyle.Render(fmt.Sprintf("🗓️ Ingested Meetings (%d):", len(rep.IngestedMeetings))))
			for _, m := range rep.IngestedMeetings {
				fmt.Printf("  • %s\n", m)
			}
			fmt.Println()
		}

		return nil
	},
}

func init() {
	morningCmd.Flags().StringVarP(&morningDate, "date", "d", "", "ISO reference date (YYYY-MM-DD)")
	RootCmd.AddCommand(morningCmd)
}
