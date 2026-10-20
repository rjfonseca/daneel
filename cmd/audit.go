//nolint:revive // package comment not required
package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/rjfonseca/daneel/internal/auditor"
	"github.com/rjfonseca/daneel/internal/search"
)

var (
	auditSince      string
	auditDryRun     bool
	auditMineSkills bool
	auditEvals      bool
	auditApply      bool
	auditBrainDir   string
)

var auditCmd = &cobra.Command{
	Use:     "audit",
	Aliases: []string{"miner", "learn"},
	Short:   "Audits agent interactions, session transcripts, and cognitive learnings",
	Long: `🔍 Daneel Cognitive Auditor & Session Miner

Mines AI agent transcripts (e.g. Antigravity) to continuously improve the Second Brain:
- Detects user corrections and syncs them to meta/guias/aprendizados-agente.md
- Identifies repetitive multi-step prompts and proposes new SKILL.md scaffolds
- Evaluates knowledge gaps and missed vault opportunities
`,
}

var auditSessionsCmd = &cobra.Command{
	Use:     "sessions",
	Aliases: []string{"transcripts", "history"},
	Short:   "Audits AI agent session transcripts for corrections, skills, and evals",
	RunE: func(cmd *cobra.Command, args []string) error { //nolint:revive
		d, err := time.ParseDuration(auditSince)
		if err != nil {
			return fmt.Errorf("invalid duration for --since (e.g. 48h, 7d): %w", err)
		}

		vaultRoot := ""
		if cfg != nil {
			vaultRoot = cfg.VaultPath
		}

		effectiveDryRun := auditDryRun
		if !cmd.Flags().Changed("dry-run") && !auditApply {
			effectiveDryRun = true
		}

		miner := auditor.NewSessionMiner(auditor.AuditOptions{
			Since:      d,
			MineSkills: auditMineSkills,
			Evals:      auditEvals,
			Apply:      auditApply,
			DryRun:     effectiveDryRun,
			BrainDir:   auditBrainDir,
			VaultRoot:  vaultRoot,
		})

		report, err := miner.Audit(cmd.Context())
		if err != nil {
			return fmt.Errorf("session audit failed: %w", err)
		}

		if jsonOut {
			bytes, err := json.MarshalIndent(report, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(bytes))
			return nil
		}

		titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("75"))
		subStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
		headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
		passStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("82"))
		warnStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
		infoStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
		badgeCorrection := lipgloss.NewStyle().Background(lipgloss.Color("196")).Foreground(lipgloss.Color("16")).Bold(true).Padding(0, 1).Render("RULE CORRECTION")
		badgeSkill := lipgloss.NewStyle().Background(lipgloss.Color("141")).Foreground(lipgloss.Color("16")).Bold(true).Padding(0, 1).Render("SKILL PROPOSAL")
		badgeGap := lipgloss.NewStyle().Background(lipgloss.Color("208")).Foreground(lipgloss.Color("16")).Bold(true).Padding(0, 1).Render("KNOWLEDGE GAP")

		fmt.Println(titleStyle.Render(fmt.Sprintf("🧬 Daneel Session Miner & Evals (Last %s)", d.String())))
		fmt.Println(subStyle.Render(fmt.Sprintf("Scanned: %d conversation sessions, %d interaction turns", report.ScannedConversations, report.ScannedTurns)))
		fmt.Println(strings.Repeat("─", 80))

		if len(report.Learnings) == 0 && len(report.SkillProposals) == 0 {
			fmt.Printf("\n%s No actionable learnings or repetitive patterns found in the specified window.\n\n", passStyle.Render("✨"))
			return nil
		}

		// 1. User Corrections & Rules
		corrections := []auditor.ExtractedLearning{}
		gaps := []auditor.ExtractedLearning{}
		for _, l := range report.Learnings {
			switch l.SignalType {
			case auditor.SignalCorrection:
				corrections = append(corrections, l)
			case auditor.SignalKnowledgeGap:
				gaps = append(gaps, l)
			}
		}

		if len(corrections) > 0 {
			fmt.Println(headerStyle.Render(fmt.Sprintf("\n📌 Discovered User Corrections (%d):", len(corrections))))
			for i, c := range corrections {
				fmt.Printf(" %2d. %s %s\n", i+1, badgeCorrection, warnStyle.Render(c.Topic))
				fmt.Printf("     %s %s\n", subStyle.Render("Turn Context:"), infoStyle.Render(c.Description))
				fmt.Printf("     %s %s\n", subStyle.Render("Suggested Action:"), passStyle.Render(c.SuggestedAction))
				fmt.Println()
			}
		}

		// 2. Skill Proposals
		if len(report.SkillProposals) > 0 {
			fmt.Println(headerStyle.Render(fmt.Sprintf("\n⚡ Mined Skill Candidates (%d):", len(report.SkillProposals))))
			for i, s := range report.SkillProposals {
				fmt.Printf(" %2d. %s %s (Frequency: %d)\n", i+1, badgeSkill, titleStyle.Render(s.Name), s.Frequency)
				fmt.Printf("     %s %s\n", subStyle.Render("Pattern:"), infoStyle.Render(s.PromptPattern))
				fmt.Printf("     %s %s\n", subStyle.Render("Target:"), passStyle.Render(fmt.Sprintf(".agents/skills/propostas/%s/SKILL.md", s.Name)))
				fmt.Println()
			}
		}

		// 3. Evals / Knowledge Gaps
		if len(gaps) > 0 {
			fmt.Println(headerStyle.Render(fmt.Sprintf("\n🔍 Knowledge Gaps & Missed Opportunities (%d):", len(gaps))))
			for i, g := range gaps {
				fmt.Printf(" %2d. %s %s\n", i+1, badgeGap, warnStyle.Render(g.Topic))
				fmt.Printf("     %s %s\n", subStyle.Render("Doubt/Gap:"), infoStyle.Render(g.Description))
				fmt.Printf("     %s %s\n", subStyle.Render("Suggested Action:"), passStyle.Render(g.SuggestedAction))
				fmt.Println()
			}
		}

		fmt.Println(strings.Repeat("─", 80))
		if auditApply && !effectiveDryRun {
			fmt.Printf("%s Successfully applied %d learnings and wrote %d skill proposals to vault.\n\n",
				passStyle.Render("✅"), report.AppliedLearnings, report.AppliedSkills)
		} else {
			fmt.Printf("%s Dry run mode. Run with %s to persist learnings to %s\n\n",
				infoStyle.Render("💡"), warnStyle.Render("daneel audit sessions --apply"), passStyle.Render("meta/guias/aprendizados-agente.md"))
		}

		return nil
	},
}

var (
	auditAgentsSince     string
	auditAgentsAgent     string
	auditAgentsApply     bool
	auditAgentsDryRun    bool
	auditAgentsBrainDir  string
	auditAgentsClaudeDir string
	auditAgentsCursorDir string
)

var auditAgentsCmd = &cobra.Command{
	Use:     "agents",
	Aliases: []string{"evals", "agent-evals"},
	Short:   "Audits AI agent session transcripts for missed vault opportunities and knowledge gaps",
	Long: `🧬 Daneel Agent Evals & Agnostic Telemetry

Scans interaction history across AI agents (Antigravity, Claude Code, Cursor) and classifies false negatives into:
1. Missed Opportunity (Agent Failure): The information exists in the Vault, but the agent interrogated the user instead of searching.
2. Knowledge Gap (Vault Failure): The agent asked about a technical topic missing from the Vault, signaling a new note or Harvester.

Runs a hybrid two-phase pipeline:
- Phase 1: Native Go deterministic filtering (time window, regex interrogation targeting, tool negation check).
- Phase 2: Classification and validation against SQLite FTS5 search engine and note resolution.
`,
	RunE: func(cmd *cobra.Command, args []string) error { //nolint:revive
		d, err := time.ParseDuration(auditAgentsSince)
		if err != nil {
			return fmt.Errorf("invalid duration for --since (e.g. 24h, 48h, 72h): %w", err)
		}

		vaultRoot := ""
		dbPath := ""
		if cfg != nil {
			vaultRoot = cfg.VaultPath
			dbPath = cfg.DBPath
		}

		var engine *search.Engine
		if dbPath != "" {
			eng, err := search.Open(dbPath, vaultRoot)
			if err == nil {
				engine = eng
				defer engine.Close() //nolint:errcheck,gosec
			}
		}

		effectiveDryRun := auditAgentsDryRun
		if !cmd.Flags().Changed("dry-run") && !auditAgentsApply {
			effectiveDryRun = true
		}

		evaluator := auditor.NewAgentEvaluator(engine, auditor.AgentEvalOptions{
			Since:       d,
			AgentFilter: auditAgentsAgent,
			Apply:       auditAgentsApply,
			DryRun:      effectiveDryRun,
			BrainDir:    auditAgentsBrainDir,
			ClaudeDir:   auditAgentsClaudeDir,
			CursorDir:   auditAgentsCursorDir,
			VaultRoot:   vaultRoot,
		})

		report, err := evaluator.Evaluate(cmd.Context())
		if err != nil {
			return fmt.Errorf("agent evals failed: %w", err)
		}

		if jsonOut {
			bytes, err := json.MarshalIndent(report, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(bytes))
			return nil
		}

		titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("75"))
		subStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
		headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
		passStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("82"))
		warnStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
		infoStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
		badgeMissed := lipgloss.NewStyle().Background(lipgloss.Color("196")).Foreground(lipgloss.Color("16")).Bold(true).Padding(0, 1).Render("MISSED OPPORTUNITY")
		badgeGap := lipgloss.NewStyle().Background(lipgloss.Color("141")).Foreground(lipgloss.Color("16")).Bold(true).Padding(0, 1).Render("KNOWLEDGE GAP")
		badgeAgent := lipgloss.NewStyle().Background(lipgloss.Color("238")).Foreground(lipgloss.Color("255")).Bold(true).Padding(0, 1)

		fmt.Println(titleStyle.Render(fmt.Sprintf("🧬 Daneel Agent Evals & Telemetry (Last %s) [Agent: %s]", d.String(), auditAgentsAgent)))
		fmt.Println(subStyle.Render(fmt.Sprintf("Scanned: %d turns | Suspicious Deltas: %d | Missed Opportunities: %d | Knowledge Gaps: %d",
			report.ScannedInteractions, report.SuspiciousDeltas, report.MissedOpportunities, report.KnowledgeGaps)))
		fmt.Println(strings.Repeat("─", 80))

		// Telemetry summary by agent
		if len(report.ByAgent) > 1 {
			fmt.Println(subStyle.Render("Agent Telemetry Breakdown:"))
			for agent, s := range report.ByAgent {
				fmt.Printf("  • %s: %d scanned, %d deltas, %d missed, %d gaps\n",
					badgeAgent.Render(agent), s.ScannedInteractions, s.SuspiciousDeltas, s.MissedOpportunities, s.KnowledgeGaps)
			}
			fmt.Println(strings.Repeat("─", 80))
		}

		if len(report.Results) == 0 {
			fmt.Printf("\n%s No suspicious deltas or missed opportunities detected in the specified window.\n\n", passStyle.Render("✨"))
			return nil
		}

		missedList := []auditor.AgentEvalResult{}
		gapList := []auditor.AgentEvalResult{}
		for _, r := range report.Results {
			switch r.Classification {
			case auditor.ClassificationMissedOpportunity:
				missedList = append(missedList, r)
			case auditor.ClassificationKnowledgeGap:
				gapList = append(gapList, r)
			}
		}

		// 1. Missed Opportunities (Agent Failures)
		if len(missedList) > 0 {
			fmt.Println(headerStyle.Render(fmt.Sprintf("\n🔴 Missed Opportunities - Agent Autonomy Failure (%d):", len(missedList))))
			fmt.Println(subStyle.Render("   Information exists in the Vault, but the agent interrogated the user instead of searching."))
			fmt.Println()
			for i, m := range missedList {
				fmt.Printf(" %2d. %s %s %s\n", i+1, badgeMissed, badgeAgent.Render(m.AgentName), warnStyle.Render(m.Doubt))
				fmt.Printf("     %s %s\n", subStyle.Render("User Prompt:"), infoStyle.Render(m.UserPrompt))
				fmt.Printf("     %s %s\n", subStyle.Render("Agent Question:"), infoStyle.Render(m.AgentReply))
				fmt.Printf("     %s %s\n", subStyle.Render("Vault Note(s):"), passStyle.Render(strings.Join(m.MatchedNotes, ", ")))
				fmt.Printf("     %s %s\n", subStyle.Render("Prompt Refinement:"), infoStyle.Render(m.SuggestedAction))
				fmt.Println()
			}
		}

		// 2. Knowledge Gaps (Vault Coverage Failures)
		if len(gapList) > 0 {
			fmt.Println(headerStyle.Render(fmt.Sprintf("\n🟣 Knowledge Gaps - Vault Coverage Deficit (%d):", len(gapList))))
			fmt.Println(subStyle.Render("   Agent requested technical clarification, but no matching documentation exists in the Vault."))
			fmt.Println()
			for i, g := range gapList {
				fmt.Printf(" %2d. %s %s %s\n", i+1, badgeGap, badgeAgent.Render(g.AgentName), titleStyle.Render(g.Doubt))
				fmt.Printf("     %s %s\n", subStyle.Render("User Prompt:"), infoStyle.Render(g.UserPrompt))
				fmt.Printf("     %s %s\n", subStyle.Render("Agent Question:"), infoStyle.Render(g.AgentReply))
				fmt.Printf("     %s %s\n", subStyle.Render("Suggested Action:"), passStyle.Render(g.SuggestedAction))
				fmt.Println()
			}
		}

		fmt.Println(strings.Repeat("─", 80))
		if auditAgentsApply && !effectiveDryRun {
			fmt.Printf("%s Successfully persisted %d eval learnings and synchronized telemetry metrics.\n\n",
				passStyle.Render("✅"), report.AppliedCount)
		} else {
			fmt.Printf("%s Dry run mode. Run with %s to persist evals to %s and metrics.\n\n",
				infoStyle.Render("💡"), warnStyle.Render("daneel audit agents --apply"), passStyle.Render("meta/guias/aprendizados-agente.md"))
		}

		return nil
	},
}

func init() {
	auditSessionsCmd.Flags().StringVar(&auditSince, "since", "48h", "Time window for session analysis (e.g. 24h, 48h, 72h)")
	auditSessionsCmd.Flags().BoolVar(&auditDryRun, "dry-run", true, "Simulate without persisting changes to the vault")
	auditSessionsCmd.Flags().BoolVar(&auditMineSkills, "mine-skills", false, "Focus on mining repetitive prompts into new skills")
	auditSessionsCmd.Flags().BoolVar(&auditEvals, "evals", false, "Analyze agent questioning and missed opportunities")
	auditSessionsCmd.Flags().BoolVar(&auditApply, "apply", false, "Persist extracted learnings to meta/guias/aprendizados-agente.md")
	auditSessionsCmd.Flags().StringVar(&auditBrainDir, "brain-dir", "", "Custom brain directory for session transcripts (testing)")

	auditAgentsCmd.Flags().StringVar(&auditAgentsSince, "since", "48h", "Time window for agent log evaluation (e.g. 24h, 48h, 72h)")
	auditAgentsCmd.Flags().StringVar(&auditAgentsAgent, "agent", "all", "Filter by specific agent (all, antigravity, claude-code, cursor)")
	auditAgentsCmd.Flags().BoolVar(&auditAgentsApply, "apply", false, "Persist extracted eval learnings and update dashboard metrics")
	auditAgentsCmd.Flags().BoolVar(&auditAgentsDryRun, "dry-run", true, "Simulate without modifying vault notes or metrics")
	auditAgentsCmd.Flags().StringVar(&auditAgentsBrainDir, "brain-dir", "", "Custom brain directory for Antigravity transcripts")
	auditAgentsCmd.Flags().StringVar(&auditAgentsClaudeDir, "claude-dir", "", "Custom directory for Claude Code transcripts")
	auditAgentsCmd.Flags().StringVar(&auditAgentsCursorDir, "cursor-dir", "", "Custom directory for Cursor workspace storage")

	auditCmd.AddCommand(auditSessionsCmd)
	auditCmd.AddCommand(auditAgentsCmd)
	RootCmd.AddCommand(auditCmd)
}
