//nolint:revive // package comment not required
package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/rjfonseca/daneel/internal/ops/briefing"
)

var prepCmd = &cobra.Command{
	Use:     "prep <person-or-topic>",
	Aliases: []string{"preflight", "briefing", "1on1"},
	Short:   "Generates an executive strategic briefing for 1:1 meetings and technical alignments",
	Args:    cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error { //nolint:revive
		query := strings.Join(args, " ")

		svc := briefing.NewPreflightService(cfg.VaultPath)
		profile, err := svc.GenerateBriefing(query)
		if err != nil {
			return err
		}

		if jsonOut {
			bytes, err := json.MarshalIndent(profile, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(bytes))
			return nil
		}

		titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
		labelStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
		sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("244"))
		badgeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))

		fmt.Printf("%s %s\n\n", titleStyle.Render("👤 1:1 Briefing:"), labelStyle.Render(profile.Name))
		fmt.Printf("• Role / Title:    %s\n", profile.Cargo)
		fmt.Printf("• Squad / Tribe:   %s\n", profile.Squad)
		fmt.Printf("• Relationship:    %s\n", profile.Relacao)
		fmt.Printf("• Profile File:    %s\n\n", badgeStyle.Render(profile.RelPath))

		if len(profile.PendingTasks) > 0 {
			fmt.Println(sectionStyle.Render("📌 Current Focus & Active Backlog Tasks:"))
			for _, task := range profile.PendingTasks {
				fmt.Printf("  [ ] %s\n", task)
			}
			fmt.Println()
		}

		if len(profile.RecentDailyMentions) > 0 {
			fmt.Println(sectionStyle.Render("🗓️ Recent Daily Log Mentions (Last 30 Days):"))
			for _, m := range profile.RecentDailyMentions {
				fmt.Printf("  • %s\n", m)
			}
			fmt.Println()
		}

		if len(profile.RecentInteractions) > 0 {
			fmt.Println(sectionStyle.Render("🤝 Past Interaction History:"))
			for _, inter := range profile.RecentInteractions {
				fmt.Printf("  • %s\n", inter)
			}
			fmt.Println()
		}

		if len(profile.Topics) > 0 {
			fmt.Println(sectionStyle.Render("🎯 Topics of Interest & Growth / IDP:"))
			for _, top := range profile.Topics {
				fmt.Printf("  • %s\n", top)
			}
			fmt.Println()
		}

		return nil
	},
}

func init() {
	RootCmd.AddCommand(prepCmd)
}
