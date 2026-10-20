//nolint:revive // package comment not required
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/rjfonseca/daneel/internal/daemon"
	"github.com/rjfonseca/daneel/internal/dream"
	"github.com/rjfonseca/daneel/internal/lore"
	"github.com/rjfonseca/daneel/internal/search"
)

var (
	tidyDate   string
	tidyNoFix  bool
	tidyLearn  bool
	tidyTopic  string
	tidyAction string
	tidyNote   string
)

var tidyCmd = &cobra.Command{
	Use:     "tidy",
	Aliases: []string{"consolidate", "sync", "hygiene", "clean"},
	Short:   "Runs deterministic vault hygiene: WikiLinks conversion, MOC synchronization, people timeline sync, and FTS5 indexing",
	Long: `⚡ Daneel Tidy (Deterministic Vault Hygiene & Synchronization)

Performs ultra-fast local maintenance on the Vault (< 50ms):
• Converts [[WikiLinks]] to canonical relative Markdown links
• Synchronizes orphan notes into their respective thematic MOCs
• Catalogs local workspace repositories (~/Work) and updates systems MOC
• Synchronizes colleague interaction timelines from daily logs
• Audits graph density and pre-indexes SQLite FTS5 database`,
	RunE: func(cmd *cobra.Command, args []string) error { //nolint:revive
		if tidyLearn && tidyTopic != "" && tidyAction != "" {
			err := dream.RecordFeedback(cfg.VaultPath, tidyTopic, "manual_feedback", tidyAction, tidyNote)
			if err != nil {
				return fmt.Errorf("failed to record learning: %w", err)
			}
			fmt.Printf("✅ Positronic learning pathway recorded for '%s'.\n", tidyTopic)
			return nil
		}

		if tidyDate == "" {
			tidyDate = time.Now().Format("2006-01-02")
		}

		engine := dream.NewDreamEngine(cfg.VaultPath)
		report, err := engine.RunFullCycle(tidyDate, !tidyNoFix, tidyNoFix)
		if err != nil {
			return fmt.Errorf("tidy cycle failed: %w", err)
		}

		// Reindex if files were modified
		if !tidyNoFix {
			cmdCtx := cmd.Context()
			if cmdCtx == nil {
				cmdCtx = context.Background()
			}
			if sEngine, err := search.Open(cfg.DBPath, cfg.VaultPath); err == nil {
				_, _, _ = sEngine.IndexVault(cmdCtx, cfg.GetIndexedDirs(), false)
				sEngine.Close() //nolint:errcheck,gosec // fully read or safe to ignore
			}

			if cli, err := daemon.NewClient(); err == nil {
				_ = cli.PublishUIEvent(4, "", "")
				_ = cli.Close()
			}
		}

		if jsonOut {
			bytes, err := json.MarshalIndent(report, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(bytes))
			return nil
		}

		partnerName := "Partner"
		if cfg != nil && cfg.Vault.Owner != "" {
			partnerName = fmt.Sprintf("Partner %s", strings.Fields(cfg.Vault.Owner)[0])
		}

		titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("75"))
		passStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("82"))
		alertStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196"))
		pathStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
		numStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("220"))

		fmt.Println(titleStyle.Render(fmt.Sprintf("🧹 Vault Hygiene & Deterministic Consolidation [%s] — %s", tidyDate, partnerName)))
		fmt.Println(strings.Repeat("─", 80))

		// 1. Telemetry
		fmt.Println("\n1. 📅 Daily Telemetry:")
		if report.DailyTelemetry != nil {
			fmt.Printf("   ✅ Daily log `%s`: %s recorded.\n",
				report.DailyTelemetry["file"],
				passStyle.Render(fmt.Sprintf("%.1fh", report.DailyTelemetry["total_hours"])),
			)
			for _, arch := range cfg.GetTaxonomyArchetypes() {
				if val, ok := report.DailyTelemetry[arch.ID]; ok && val != nil {
					fmt.Printf("      • [%s] %v\n", arch.ID, val)
				}
			}
		} else {
			fmt.Println("   ⚠️ Daily log missing calculated telemetry. Agent can reconcile.")
		}

		// 2. 7-day Temporal Analysis & Drift
		fmt.Println("\n2. 📊 Staff Engineering Temporal Analysis (7-Day Window):")
		if report.TemporalAnalysis != nil {
			ta := report.TemporalAnalysis
			fmt.Printf("   ⏱️ Total accumulated: %s across %d active days.\n", numStyle.Render(fmt.Sprintf("%.1fh", ta.TotalHours)), ta.DaysWithLogs)
			fmt.Print("   📈 Distribution: ")
			for _, arch := range cfg.GetTaxonomyArchetypes() {
				fmt.Printf("[%s] %.1f%% | ", arch.ID, ta.Percentages[arch.ID])
			}
			fmt.Println()
			if len(ta.DriftAlerts) > 0 {
				for _, da := range ta.DriftAlerts {
					fmt.Printf("   %s %s\n", alertStyle.Render("⚠️ Staff Drift:"), da)
				}
			} else {
				fmt.Printf("   %s Time allocation perfectly aligned with Staff Eng career goals.\n", passStyle.Render("✅"))
			}
		}

		// 3. Strategic Radar
		fmt.Println("\n3. 🧭 Psychohistorical Strategic Radar (Cross-Domain Mapping):")
		if report.StrategicRadar != nil {
			fmt.Println("   🏢 Strategic Initiative Traction (7 Days):")
			limit := 4
			if len(report.StrategicRadar.TractionSummary) < limit {
				limit = len(report.StrategicRadar.TractionSummary)
			}
			for _, tr := range report.StrategicRadar.TractionSummary[:limit] {
				fmt.Printf("      • %s: %s (%d mentions)\n", tr.Initiative, tr.Status, tr.Count)
			}
			if len(report.StrategicRadar.StrategicInsights) > 0 {
				fmt.Println("   💡 High-Value Strategic Connections Identified:")
				for _, si := range report.StrategicRadar.StrategicInsights {
					fmt.Printf("      • %s\n", si)
				}
			}
		}

		// 4. Friction Signals
		if report.TemporalAnalysis != nil && len(report.TemporalAnalysis.FrictionSignals) > 0 {
			fmt.Println("\n4. ⚠️ Technical Debt & Recurring Friction Alerts:")
			for _, fs := range report.TemporalAnalysis.FrictionSignals {
				fmt.Printf("   • [%s] %s\n", fs.Date, fs.Line)
			}
		}

		// 5. People sync
		fmt.Println("\n5. 🤝 Encounters & Partners Alignment:")
		if len(report.SyncedPeople) > 0 {
			fmt.Printf("   ✅ %d colleague interaction timeline(s) automatically synchronized:\n", len(report.SyncedPeople))
			for _, p := range report.SyncedPeople {
				fmt.Printf("      • %s\n", p)
			}
		} else {
			fmt.Println("   ✅ All partner interaction timelines are up to date.")
		}

		// 6. Graph Self-Healing
		fmt.Println("\n6. 🩹 Synaptic Self-Healing & Graph Integrity:")
		fmt.Printf("   ✅ %d WikiLinks converted to canonical relative Markdown.\n", report.FixedLinksCount)
		if report.BrokenLinksCount > 0 {
			fmt.Printf("   ⚠️ %d broken links pending review.\n", report.BrokenLinksCount)
		} else {
			fmt.Println("   ✅ Zero broken links in Vault.")
		}

		// 7. MOCs & Workspace
		fmt.Println("\n7. 🗺️ Seldon Index & Galactic Workspace Sync (~/Work):")
		if len(report.MissingMOCs) > 0 {
			fmt.Printf("   ✅ %d note(s) mapped into thematic MOCs.\n", len(report.MissingMOCs))
		} else {
			fmt.Println("   ✅ 100% of technical notes indexed in thematic MOCs.")
		}
		if report.WorkspaceSummary != nil {
			fmt.Printf("   ⚡ Workspace: %d repositories cataloged (%d clean / %d with local changes).\n",
				report.WorkspaceSummary.TotalRepos, report.WorkspaceSummary.CleanCount, report.WorkspaceSummary.DirtyCount)
		}

		// 8. Inbox Triage
		fmt.Println("\n8. 📥 Smart Inbox Triage (Learned Heuristics):")
		if len(report.InboxTriaged) > 0 {
			for _, it := range report.InboxTriaged {
				badge := passStyle.Render("🟢 HIGH")
				switch it.Confidence {
				case "MEDIUM":
					badge = numStyle.Render("🟡 MEDIUM")
				case "LOW":
					badge = alertStyle.Render("🔴 LOW")
				}
				fmt.Printf("   [%s] `%s` ➔ Suggestion: `%s` (Score: %d%%)\n", badge, it.File, it.SuggestedCategory, it.Score)
			}
		} else {
			fmt.Println("   ✅ Inbox empty and fully triaged.")
		}

		// 9. Emerging Concepts
		fmt.Println("\n9. 💡 Emerging Concepts & Discoveries:")
		if len(report.EmergingConcepts) > 0 {
			for _, ec := range report.EmergingConcepts {
				fmt.Printf("   🌱 `%s`\n", ec.Title)
			}
		} else {
			fmt.Println("   ℹ️ No new latent concepts detected today.")
		}

		// 10. Active Backlog
		fmt.Println("\n10. 🎯 Priority Focus for Tomorrow (Active Backlog):")
		if len(report.BacklogP0) > 0 {
			fmt.Println("   🔴 P0 (Urgent / Blockers):")
			limit := 3
			if len(report.BacklogP0) < limit {
				limit = len(report.BacklogP0)
			}
			for _, item := range report.BacklogP0[:limit] {
				fmt.Printf("      %s\n", item)
			}
		}
		if len(report.BacklogP1) > 0 {
			fmt.Println("   🟡 P1 (Strategic / Governance):")
			limit := 3
			if len(report.BacklogP1) < limit {
				limit = len(report.BacklogP1)
			}
			for _, item := range report.BacklogP1[:limit] {
				fmt.Printf("      %s\n", item)
			}
		}

		// 11. Handoff Questions
		fmt.Printf("\n11. ❓ Morning Handoff Decisions for %s:\n", partnerName)
		if len(report.HandoffQuestions) > 0 {
			for i, q := range report.HandoffQuestions {
				fmt.Printf("   %d. [%s] %s\n", i+1, strings.ToUpper(q.Type), q.Question)
			}
		} else {
			fmt.Println("   ✅ No open decisions requiring direction at this time.")
		}

		fmt.Println("\n" + strings.Repeat("─", 80))
		fmt.Printf("✨ Vault Hygiene Completed! Cached at `%s`.\n\n", pathStyle.Render(".agents/cache/dream_handoff.json"))
		fmt.Println(lore.FormatQuote(lore.GetRandomQuote()))
		fmt.Println()

		return nil
	},
}

func init() {
	tidyCmd.Flags().StringVarP(&tidyDate, "date", "d", "", "ISO reference date (YYYY-MM-DD)")
	tidyCmd.Flags().BoolVar(&tidyNoFix, "no-fix", false, "Run in dry-run mode without modifying files")
	tidyCmd.Flags().BoolVar(&tidyLearn, "learn", false, "Record feedback / learning")
	tidyCmd.Flags().StringVar(&tidyTopic, "topic", "", "Learning topic")
	tidyCmd.Flags().StringVar(&tidyAction, "action", "", "Learned action or category")
	tidyCmd.Flags().StringVar(&tidyNote, "note", "", "Additional context note")
	RootCmd.AddCommand(tidyCmd)
}
