//nolint:revive // package comment not required
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/rjfonseca/daneel/internal/dream"
	"github.com/rjfonseca/daneel/internal/lore"
	"github.com/rjfonseca/daneel/internal/routine"
	"github.com/rjfonseca/daneel/internal/safefile"
	"github.com/rjfonseca/daneel/internal/search"
	"github.com/rjfonseca/daneel/internal/telemetry"
)

var (
	dreamDate        string
	dreamProfile     string
	dreamPrompt      string
	dreamInteractive bool
	dreamNoAgent     bool
	dreamLearn       bool
	dreamTopic       string
	dreamAction      string
	dreamNote        string
	dreamStats       bool
	dreamSynthesize  bool
)

const defaultDreamPromptTemplate = `Execute the complete Autonomous Second Brain Strategic Dream Cycle protocol (skill vault-dream-cycle) for date %s:
1. Reconcile the Staff Eng [A-E] telemetry in the daily log (daily/%s.md), calculating hours based on the activity log.
2. Run daneel doctor and perform cognitive repair of notes in notes/ (completing OKF frontmatter 'type' and 'description' and fixing broken relative links).
3. Update the Current Focus & Pendencies (Active Backlog) header at the top of notes/carreira/sumario-semanal-atividades-impactos-2026.md.
4. Promote high-certainty drafts from inbox/ to notes/ if applicable.
5. Record persistent learnings in meta/guias/aprendizados-agente.md if applicable.
6. Commit and push the git changes to origin/main.
Exit silently and terminate the session upon completing all steps.`

var dreamCmd = &cobra.Command{
	Use:     "dream [flags]",
	Aliases: []string{"rem", "sleep", "night"},
	Short:   "Orchestrates the end-to-end cognitive REM Dream Cycle via the configured AI agent",
	Long: `🌙 Daneel Dream: Autonomous End-to-End Cognitive REM Dream Cycle

Orchestrates the complete overnight consolidation:
1. Performs deterministic vault hygiene, MOC sync, and SQLite FTS5 re-indexing
2. Invokes the configured AI Agent (e.g. agy, claude) with the formal Dream Cycle prompt
3. Agent executes Staff telemetry reconciliation, doctor repair, weekly backlog update, and Git sync
4. Exits cleanly with return code 0, enabling chaining:
   $ daneel dream && systemctl poweroff
   $ daneel dream && shutdown now
`,
	RunE: func(cmd *cobra.Command, args []string) error { //nolint:revive
		if dreamLearn && dreamTopic != "" && dreamAction != "" {
			err := dream.RecordFeedback(cfg.VaultPath, dreamTopic, "manual_feedback", dreamAction, dreamNote)
			if err != nil {
				return fmt.Errorf("failed to record learning: %w", err)
			}
			fmt.Printf("✅ Positronic learning pathway recorded for '%s'.\n", dreamTopic)
			return nil
		}

		if dreamDate == "" {
			dreamDate = time.Now().Format("2006-01-02")
		}
		dreamStartTime := time.Now()

		partnerName := "Partner"
		if cfg != nil && cfg.Vault.Owner != "" {
			partnerName = fmt.Sprintf("Partner %s", strings.Fields(cfg.Vault.Owner)[0])
		}

		titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("75"))
		stepStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
		passStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("82"))
		metaStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))

		if !jsonOut {
			fmt.Println(titleStyle.Render(fmt.Sprintf("🌙 Hypnagogic Cognitive REM Dream Cycle [%s] — %s", dreamDate, partnerName)))
			fmt.Println(strings.Repeat("─", 80))
			// 1. Pre-flight Deterministic Hygiene
			fmt.Printf("%s Running pre-flight deterministic hygiene & FTS5 re-indexing...\n", stepStyle.Render("⚡ [Phase 1/2]"))
		}

		engine := dream.NewDreamEngine(cfg.VaultPath)
		engine.SetSynthesize(dreamSynthesize)
		report, err := engine.RunFullCycle(dreamDate, true, false)
		if err != nil {
			return fmt.Errorf("pre-flight hygiene failed: %w", err)
		}

		cmdCtx := cmd.Context()
		if cmdCtx == nil {
			cmdCtx = context.Background()
		}
		ctx, cancel := context.WithTimeout(cmdCtx, 2*time.Second)
		if sEngine, err := search.Open(cfg.DBPath, cfg.VaultPath); err == nil {
			_, _, _ = sEngine.IndexVault(ctx, cfg.GetIndexedDirs(), false)
			sEngine.Close() //nolint:errcheck,gosec // fully read or safe to ignore
		}
		cancel()

		// Finalize Phase 1 FinOps and update metrics dashboard
		finalizeDreamFinOps(cmdCtx, cfg.VaultPath, dreamStartTime, report, 0, "")

		if jsonOut {
			data, err := json.MarshalIndent(report, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(data))
			return nil
		}

		fmt.Printf("   %s Synaptic graph synchronized and indexed.\n", passStyle.Render("✅"))
		if len(report.RecentSessions) > 0 {
			fmt.Printf("   %s Triaged %d agent session log(s) from inbox/sessions/.\n", passStyle.Render("✅"), len(report.RecentSessions))
		}
		fmt.Println()

		if dreamStats || verbose {
			renderMetacognitiveStats(report)
		}

		// If --no-agent flag is specified, stop after Phase 1
		if dreamNoAgent {
			fmt.Printf("%s Pre-flight hygiene complete (--no-agent specified). Exiting.\n\n", passStyle.Render("✨"))
			fmt.Println(lore.FormatQuote(lore.GetRandomQuote()))
			fmt.Println()
			return nil
		}

		// 2. Resolve Agent Profile
		prof, activeName, err := cfg.GetChatProfile(dreamProfile)
		if err != nil {
			return fmt.Errorf("failed to resolve chat profile: %w", err)
		}
		if strings.TrimSpace(prof.Command) == "" {
			return fmt.Errorf("command for profile '%s' is empty in .daneel.yaml", activeName)
		}

		// 3. Build Prompt
		activePrompt := buildDreamPrompt(report, dreamDate, dreamPrompt)

		// 4. Tokenize Command and configure agent invocation
		commandTokens, err := splitCommand(prof.Command)
		if err != nil {
			return fmt.Errorf("failed to parse command for profile '%s': %w", activeName, err)
		}
		if len(commandTokens) == 0 {
			return fmt.Errorf("no command specified in profile '%s'", activeName)
		}

		// Format prompt argument according to agent type
		isAgy := false
		for _, tok := range commandTokens {
			if strings.Contains(tok, "agy") {
				isAgy = true
				break
			}
		}

		if isAgy {
			if dreamInteractive {
				commandTokens = append(commandTokens, "-i", activePrompt)
			} else {
				commandTokens = append(commandTokens, "--print-timeout", "2h", "-p", activePrompt)
			}
		} else {
			// Claude or standard agent
			commandTokens = append(commandTokens, "-p", activePrompt)
		}

		// Expand ~
		for i, tok := range commandTokens {
			commandTokens[i] = expandHomeInToken(tok)
		}

		binName := commandTokens[0]
		binPath, err := exec.LookPath(binName)
		if err != nil {
			return fmt.Errorf("executable '%s' for profile '%s' not found in PATH", binName, activeName)
		}

		// Prepare Environment
		envMap := make(map[string]string)
		for _, e := range os.Environ() {
			parts := strings.SplitN(e, "=", 2)
			if len(parts) == 2 {
				envMap[parts[0]] = parts[1]
			}
		}
		for k, v := range prof.Env {
			envMap[k] = v
		}
		var envList []string
		for k, v := range envMap {
			envList = append(envList, fmt.Sprintf("%s=%s", k, v))
		}

		// Change Cwd to Vault Root
		if cfg.VaultPath != "" {
			if err := os.Chdir(cfg.VaultPath); err != nil {
				return fmt.Errorf("failed to change to vault directory (%s): %w", cfg.VaultPath, err)
			}
		}

		fmt.Printf("%s Invoking AI Agent (%s) for cognitive consolidation...\n",
			stepStyle.Render("🤖 [Phase 2/2]"), metaStyle.Render(activeName))
		fmt.Println(metaStyle.Render("   Entering hypnagogic REM state. All cognitive tasks will execute autonomously..."))
		fmt.Println()

		// Run Agent Subprocess
		agentCmd := exec.Command(binPath, commandTokens[1:]...) //nolint:gosec // intentional
		agentCmd.Env = envList
		agentCmd.Stdin = os.Stdin
		agentCmd.Stdout = os.Stdout
		agentCmd.Stderr = os.Stderr

		runErr := agentCmd.Run()
		exitCode := 0
		var errorMsg string
		if runErr != nil {
			exitCode = 1
			errorMsg = runErr.Error()
		}
		finalizeDreamFinOps(cmdCtx, cfg.VaultPath, dreamStartTime, report, exitCode, errorMsg)

		if runErr != nil {
			return fmt.Errorf("agent dream cycle failed: %w", runErr)
		}

		fmt.Println()
		fmt.Printf("%s Dream Cycle completed successfully. The Second Brain is consolidated.\n\n", passStyle.Render("🌙 Positronic REM Complete:"))
		fmt.Println(lore.FormatQuote(lore.GetRandomQuote()))
		fmt.Println()

		return nil
	},
}

func renderMetacognitiveStats(report *dream.FullDreamReport) {
	if report == nil || report.Metacognition == nil {
		return
	}
	meta := report.Metacognition

	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("75"))
	phaseStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	durStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	warnBadge := lipgloss.NewStyle().Background(lipgloss.Color("208")).Foreground(lipgloss.Color("16")).Bold(true).Padding(0, 1).Render("BOTTLENECK")
	insightBadge := lipgloss.NewStyle().Background(lipgloss.Color("141")).Foreground(lipgloss.Color("16")).Bold(true).Padding(0, 1).Render("INSIGHT")
	synthBadge := lipgloss.NewStyle().Background(lipgloss.Color("82")).Foreground(lipgloss.Color("16")).Bold(true).Padding(0, 1).Render("SYNTHESIS")
	borderBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("63")).
		Padding(1, 2).
		MarginBottom(1)

	var sb strings.Builder
	sb.WriteString(headerStyle.Render(fmt.Sprintf("🧠 Metacognitive Dream Engine Profiling [%d ms total]\n", meta.TotalDurationMs)))
	sb.WriteString(strings.Repeat("─", 74) + "\n\n")

	sb.WriteString(lipgloss.NewStyle().Bold(true).Render("⏱️  Phase Timing Breakdown:\n"))
	for _, t := range meta.Timings {
		fmt.Fprintf(&sb, "  • %-42s %8s   %s\n",
			phaseStyle.Render(t.Name),
			durStyle.Render(fmt.Sprintf("%d ms", t.DurationMs)),
			descStyle.Render(t.Description),
		)
	}

	if len(meta.Bottlenecks) > 0 {
		sb.WriteString("\n" + lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("208")).Render("⚠️  Detected Bottlenecks:\n"))
		for _, b := range meta.Bottlenecks {
			fmt.Fprintf(&sb, "  %s %s\n", warnBadge, b)
		}
	}

	if len(meta.Syntheses) > 0 {
		sb.WriteString("\n" + lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("82")).Render("🌐 Induced Knowledge Syntheses:\n"))
		for _, s := range meta.Syntheses {
			path := s.CreatedPath
			if path == "" {
				path = s.Slug
			}
			fmt.Fprintf(&sb, "  %s %s [%s]\n", synthBadge, s.Title, path)
		}
	}

	if len(meta.Insights) > 0 {
		sb.WriteString("\n" + lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("141")).Render("💡 Metacognitive Self-Reflections:\n"))
		for _, in := range meta.Insights {
			fmt.Fprintf(&sb, "  %s [%s] %s\n", insightBadge, strings.ToUpper(in.Category), in.Observation)
			fmt.Fprintf(&sb, "     Action: %s\n", in.SuggestedAction)
		}
	}

	if meta.InputTokens > 0 || meta.EstimatedCostUSD > 0 {
		sb.WriteString("\n" + lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214")).Render("💰 FinOps & Token Usage:\n"))
		if meta.Model != "" {
			fmt.Fprintf(&sb, "  • %-20s %s\n", "Model:", meta.Model)
		}
		fmt.Fprintf(&sb, "  • %-20s %d in / %d cache read / %d out\n", "Tokens:", meta.InputTokens, meta.CacheReadTokens, meta.OutputTokens)
		if meta.SubagentsCount > 0 {
			fmt.Fprintf(&sb, "  • %-20s %d\n", "Subagents Spawned:", meta.SubagentsCount)
		}
		fmt.Fprintf(&sb, "  • %-20s $%.4f USD\n", "Estimated Cost:", meta.EstimatedCostUSD)
	}

	fmt.Fprintf(&sb, "\n📜 Evolution Log: %s\n", meta.EvolutionLogPath)

	fmt.Println(borderBox.Render(sb.String()))
}

func finalizeDreamFinOps(ctx context.Context, vPath string, dreamStartTime time.Time, report *dream.FullDreamReport, exitCode int, errorMsg string) {
	if cfg == nil || report == nil {
		return
	}
	if vPath == "" {
		vPath = cfg.VaultPath
	}
	store, err := routine.OpenDefaultStore(vPath)
	if err != nil || store == nil {
		return
	}
	defer func() { _ = store.Close() }()

	if ctx == nil {
		ctx = context.Background()
	}

	completedAt := time.Now()
	durationMs := completedAt.Sub(dreamStartTime).Milliseconds()

	parserOpts := telemetry.ParserOptions{
		Since:  7 * 24 * time.Hour,
		Config: cfg,
	}
	if vPath != "" {
		brainDir := filepath.Join(vPath, "brain")
		if fi, err := os.Stat(brainDir); err == nil && fi.IsDir() {
			parserOpts.BrainDir = brainDir
		}
	}

	summaries, _ := telemetry.SyncSessionsToStore(ctx, store, parserOpts)
	matched := telemetry.FindSessionInWindow(summaries, dreamStartTime, completedAt, "dream")
	if matched == nil {
		for i := range summaries {
			if summaries[i].Record.RoutineName == "dream" || strings.Contains(summaries[i].Record.SkillsUsed, "vault-dream-cycle") {
				matched = &summaries[i]
				break
			}
		}
	}

	if report.Metacognition != nil && matched != nil {
		report.Metacognition.InputTokens = matched.TotalInputTokens
		report.Metacognition.CacheReadTokens = matched.TotalCacheTokens
		report.Metacognition.OutputTokens = matched.TotalOutputTokens
		report.Metacognition.EstimatedCostUSD = matched.TotalCostUSD
		report.Metacognition.SubagentsCount = matched.Record.SubagentsCount
		report.Metacognition.Model = matched.Record.Model

		handoffPath := filepath.Join(vPath, ".daneel", "dream_handoff.json")
		if data, err := json.MarshalIndent(report, "", "  "); err == nil {
			_ = safefile.WriteFile(vPath, handoffPath, data, 0600)
		}
	}

	_ = dream.UpdateDashboardMetricas(vPath, report, report.TotalFilesScanned)

	status := "success"
	if exitCode != 0 || errorMsg != "" {
		status = "failed"
	}

	run := routine.RoutineRun{
		RoutineName:    "dream",
		StartedAt:      dreamStartTime.Unix(),
		CompletedAt:    completedAt.Unix(),
		DurationMs:     durationMs,
		TriggerSource:  "cli",
		ExitCode:       exitCode,
		Status:         status,
		Summary:        fmt.Sprintf("Dream cycle finished with %d files scanned in %dms", report.TotalFilesScanned, durationMs),
		ErrorMessage:   errorMsg,
		ItemsProcessed: report.TotalFilesScanned,
	}
	if report.Metacognition != nil {
		run.InputTokens = report.Metacognition.InputTokens
		run.CacheReadTokens = report.Metacognition.CacheReadTokens
		run.OutputTokens = report.Metacognition.OutputTokens
		run.EstimatedCostUSD = report.Metacognition.EstimatedCostUSD
		run.Model = report.Metacognition.Model
		if matched != nil {
			run.SessionID = matched.Record.SessionID
		}
	}
	_ = store.RecordRun(ctx, run)
}

func buildDreamPrompt(report *dream.FullDreamReport, targetDate, customPrompt string) string {
	activePrompt := customPrompt
	if activePrompt == "" {
		activePrompt = fmt.Sprintf(defaultDreamPromptTemplate, targetDate, targetDate)
	}

	if report != nil && len(report.RecentSessions) > 0 {
		var b strings.Builder
		b.WriteString("\n\n### Triaged Agent Sessions (inbox/sessions/):\n")
		for _, s := range report.RecentSessions {
			fmt.Fprintf(&b, "- **%s** (%s) in `%s` [%d mins, %d events]\n",
				s.File, s.Agent, s.Directory, s.DurationMinutes, s.EventsCount)
		}
		b.WriteString("Review and synthesize these agent sessions into relevant daily notes, architecture documents, or backlog items.\n")
		activePrompt += b.String()
	}

	return activePrompt
}

func init() {
	dreamCmd.Flags().StringVarP(&dreamDate, "date", "d", "", "ISO reference date (YYYY-MM-DD)")
	dreamCmd.Flags().StringVarP(&dreamProfile, "profile", "p", "", "Name of the chat profile to execute (e.g. agy, claude)")
	dreamCmd.Flags().StringVar(&dreamPrompt, "prompt", "", "Custom prompt override for the dream cycle")
	dreamCmd.Flags().BoolVarP(&dreamInteractive, "interactive", "i", false, "Launch agent interactively (-i) instead of headless print mode (-p)")
	dreamCmd.Flags().BoolVar(&dreamNoAgent, "no-agent", false, "Run only pre-flight hygiene and diagnostics without invoking AI agent")
	dreamCmd.Flags().BoolVar(&dreamLearn, "learn", false, "Record feedback / learning")
	dreamCmd.Flags().StringVar(&dreamTopic, "topic", "", "Learning topic")
	dreamCmd.Flags().StringVar(&dreamAction, "action", "", "Learned action or category")
	dreamCmd.Flags().StringVar(&dreamNote, "note", "", "Additional context note")
	dreamCmd.Flags().BoolVar(&dreamStats, "stats", false, "Display engine profiling, phase timing breakdown, and metacognitive insights")
	dreamCmd.Flags().BoolVar(&dreamStats, "metacognition", false, "Alias for --stats: display engine profiling and metacognitive insights")
	dreamCmd.Flags().BoolVar(&dreamSynthesize, "synthesize", true, "Enable active Zettelkasten knowledge synthesis induction")
	RootCmd.AddCommand(dreamCmd)
}
