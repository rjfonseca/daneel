//nolint:revive // package comment not required
package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/rjfonseca/daneel/internal/dream"
	"github.com/rjfonseca/daneel/internal/ops/daily"
)

var (
	handoffSince          string
	handoffDate           string
	handoffKind           = string(dream.HandoffKindAgentAction)
	handoffSummary        string
	handoffActions        []string
	handoffInbox          []string
	handoffAlerts         []string
	handoffResolvedAlerts []string
	handoffAppendEvo      bool
)

var handoffCmd = &cobra.Command{
	Use:   "handoff",
	Short: "Aggregates and records cumulative handoff events across REM cycles and cognitive closures",
}

var handoffSummaryCmd = &cobra.Command{
	Use:   "summary",
	Short: "Prints cumulative handoff events since the last morning handoff without mutating files or watermarks",
	RunE: func(cmd *cobra.Command, args []string) error { //nolint:revive
		rep, err := daily.RunMorningWithOptions(cfg.VaultPath, handoffDate, cfg, daily.MorningOptions{
			DryRun: true,
			Since:  handoffSince,
		})
		if err != nil {
			return fmt.Errorf("handoff summary failed: %w", err)
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
		sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("244"))
		warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214"))

		fmt.Printf("%s\n", titleStyle.Render("🤝 Cumulative Morning Handoff Summary"))
		fmt.Printf("• Window: %s ➔ %s\n", rep.WindowStart, rep.WindowEnd)
		fmt.Printf("• Automated REM Cycles in Window: %d\n\n", rep.DreamCyclesCount)

		if len(rep.AutonomousActions) > 0 {
			fmt.Println(sectionStyle.Render("🤖 Autonomous Actions & Closures:"))
			for _, act := range rep.AutonomousActions {
				fmt.Printf("  • %s\n", act)
			}
			fmt.Println()
		}
		if len(rep.InboxItems) > 0 {
			fmt.Println(warnStyle.Render(fmt.Sprintf("📥 Inbox Items (%d):", len(rep.InboxItems))))
			for _, item := range rep.InboxItems {
				fmt.Printf("  [ ] %s\n", item)
			}
			fmt.Println()
		}
		if len(rep.Alerts) > 0 {
			fmt.Println(warnStyle.Render(fmt.Sprintf("⚠️ Active Alerts (%d):", len(rep.Alerts))))
			for _, alert := range rep.Alerts {
				fmt.Printf("  ! %s\n", alert)
			}
			fmt.Println()
		}
		return nil
	},
}

var handoffRecordCmd = &cobra.Command{
	Use:   "record",
	Short: "Records a cognitive closure or agent action in the cumulative handoff ledger",
	RunE: func(cmd *cobra.Command, args []string) error { //nolint:revive
		kind := dream.HandoffEventKind(strings.TrimSpace(handoffKind))
		switch kind {
		case dream.HandoffKindEveningClosure, dream.HandoffKindAgentAction, dream.HandoffKindDreamCycle:
		default:
			return fmt.Errorf("invalid --kind %q (expected evening_closure, agent_action, or dream_cycle)", handoffKind)
		}

		now := time.Now()
		ev := dream.HandoffEvent{
			Kind:           kind,
			Timestamp:      now.Format(time.RFC3339),
			Date:           now.Format("2006-01-02"),
			Summary:        strings.TrimSpace(handoffSummary),
			Actions:        handoffActions,
			InboxPending:   handoffInbox,
			Alerts:         handoffAlerts,
			ResolvedAlerts: handoffResolvedAlerts,
		}

		if err := dream.AppendHandoffEvent(cfg.VaultPath, ev); err != nil {
			return fmt.Errorf("failed to append handoff event: %w", err)
		}
		if handoffAppendEvo {
			if err := dream.AppendCognitiveEvolutionLog(cfg.VaultPath, ev); err != nil {
				return fmt.Errorf("failed to append evolution log: %w", err)
			}
		}

		if jsonOut {
			data, err := json.MarshalIndent(ev, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(data))
			return nil
		}

		fmt.Printf("✅ Recorded handoff event (%s): %s\n", ev.Kind, ev.Summary)
		return nil
	},
}

func init() {
	handoffSummaryCmd.Flags().StringVar(&handoffSince, "since", "", "Override window start timestamp (RFC3339 or YYYY-MM-DD HH:MM)")
	handoffSummaryCmd.Flags().StringVarP(&handoffDate, "date", "d", "", "ISO reference date (YYYY-MM-DD)")

	handoffRecordCmd.Flags().StringVar(&handoffKind, "kind", string(dream.HandoffKindAgentAction), "Event kind (evening_closure | agent_action | dream_cycle)")
	handoffRecordCmd.Flags().StringVar(&handoffSummary, "summary", "", "Concise summary of the cognitive action or evening closure")
	handoffRecordCmd.Flags().StringSliceVar(&handoffActions, "action", nil, "Autonomous action completed (repeatable)")
	handoffRecordCmd.Flags().StringSliceVar(&handoffInbox, "inbox", nil, "Inbox item requiring human decision (repeatable)")
	handoffRecordCmd.Flags().StringSliceVar(&handoffAlerts, "alert", nil, "Active alert to surface (repeatable)")
	handoffRecordCmd.Flags().StringSliceVar(&handoffResolvedAlerts, "resolved-alert", nil, "Previously active alert resolved in this session (repeatable)")
	handoffRecordCmd.Flags().BoolVar(&handoffAppendEvo, "append-evolution", false, "Also append a chronological entry to meta/guias/agente-evolucao.md")

	handoffCmd.AddCommand(handoffSummaryCmd)
	handoffCmd.AddCommand(handoffRecordCmd)
	RootCmd.AddCommand(handoffCmd)
}
