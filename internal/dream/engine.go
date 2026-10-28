//nolint:revive,misspell // package comment not required
package dream

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/rjfonseca/daneel/internal/safefile"

	"gopkg.in/yaml.v3"

	"github.com/rjfonseca/daneel/internal/config"
	"github.com/rjfonseca/daneel/internal/ops/workspace"
	"github.com/rjfonseca/daneel/internal/routine"
	"github.com/rjfonseca/daneel/internal/telemetry"
	"github.com/rjfonseca/daneel/internal/vault"
)

// HandoffQuestion represents an interactive or strategic handoff prompt.
type HandoffQuestion struct {
	Topic    string `json:"topic"`
	Type     string `json:"type"`
	Question string `json:"question"`
	Context  string `json:"context,omitempty"`
}

// FullDreamReport contains all synthesized data from all 3 cognitive layers.
type FullDreamReport struct {
	Date              string                     `json:"date"`
	GeneratedAt       string                     `json:"generated_at"`
	TotalFilesScanned int                        `json:"total_files_scanned"`
	FixedLinksCount   int                        `json:"fixed_links_count"`
	BrokenLinksCount  int                        `json:"broken_links_count"`
	MissingMOCs       []string                   `json:"missing_mocs"`
	DenseCategories   []DenseCategory            `json:"dense_categories"`
	DailyTelemetry    map[string]interface{}     `json:"daily_telemetry,omitempty"`
	TemporalAnalysis  *TemporalWindowData        `json:"temporal_analysis"`
	StrategicRadar    *StrategicRadar            `json:"strategic_radar"`
	SyncedPeople      []string                   `json:"synced_people"`
	WorkspaceSummary  *workspace.WorkspaceReport `json:"workspace_summary,omitempty"`
	InboxTriaged      []InboxTriageItem          `json:"inbox_triaged"`
	RecentSessions    []SessionLogTriage         `json:"recent_sessions,omitempty"`
	EmergingConcepts  []EmergingConcept          `json:"emerging_concepts"`
	BacklogP0         []string                   `json:"backlog_p0"`
	BacklogP1         []string                   `json:"backlog_p1"`
	HandoffQuestions  []HandoffQuestion          `json:"handoff_questions"`
	Metacognition     *MetacognitiveReport       `json:"metacognition,omitempty"`
}

// DreamEngine coordinates the 5-layer cognitive REM cycle.
type DreamEngine struct { //nolint:revive
	vaultRoot  string
	synthesize bool
}

// NewDreamEngine creates a new dream cycle engine.
func NewDreamEngine(vaultRoot string) *DreamEngine {
	return &DreamEngine{vaultRoot: vaultRoot, synthesize: true}
}

// SetSynthesize enables or disables Phase 5 knowledge synthesis induction.
func (e *DreamEngine) SetSynthesize(enabled bool) *DreamEngine {
	e.synthesize = enabled
	return e
}

// RunFullCycle executes the complete 3-layer autonomous consolidation.
func (e *DreamEngine) RunFullCycle(targetDate string, autoFix bool, dryRun bool) (*FullDreamReport, error) {
	if targetDate == "" {
		targetDate = time.Now().Format("2006-01-02")
	}

	learnings := LoadLearnings(e.vaultRoot)

	report := &FullDreamReport{
		Date:             targetDate,
		GeneratedAt:      time.Now().Format(time.RFC3339),
		MissingMOCs:      []string{},
		DenseCategories:  []DenseCategory{},
		SyncedPeople:     []string{},
		InboxTriaged:     []InboxTriageItem{},
		EmergingConcepts: []EmergingConcept{},
		BacklogP0:        []string{},
		BacklogP1:        []string{},
		HandoffQuestions: []HandoffQuestion{},
	}

	// 1. Layer 1: Deterministic Graph Hygiene (Links, MOCs, Workspace)
	startPhase1 := time.Now()
	targetMap := make(map[string]string)
	notes, err := vault.ScanVault(e.vaultRoot, []string{"notes", "meta", "daily", "inbox", "sources"})
	if err == nil {
		report.TotalFilesScanned = len(notes)
		for _, n := range notes {
			base := strings.TrimSuffix(filepath.Base(n.RelPath), ".md")
			targetMap[base] = n.RelPath
			targetMap[vault.NormalizeText(base)] = n.RelPath
			targetMap[vault.NormalizeText(n.Title)] = n.RelPath
			targetMap[n.RelPath] = n.RelPath
		}

		var brokenLinkDetails []string
		wikiLinkRegex := regexp.MustCompile(`\[\[([^\]|]+)(?:\|([^\]]+))?\]\]`)
		for _, n := range notes {
			if !strings.HasPrefix(n.RelPath, "notes/") && !strings.HasPrefix(n.RelPath, "public/") {
				continue
			}

			modified := false
			sourceDirRel := filepath.Dir(n.RelPath)

			newContent := wikiLinkRegex.ReplaceAllStringFunc(n.Content, func(match string) string {
				m := wikiLinkRegex.FindStringSubmatch(match)
				if len(m) < 2 {
					return match
				}
				targetName := strings.TrimSpace(m[1])
				label := targetName
				if len(m) > 2 && m[2] != "" {
					label = strings.TrimSpace(m[2])
				}

				normTarget := vault.NormalizeText(targetName)
				targetRelPath, found := targetMap[normTarget]
				if !found {
					targetRelPath, found = targetMap[targetName]
				}

				if found {
					relToSource, err := filepath.Rel(sourceDirRel, targetRelPath)
					if err == nil {
						relToSource = filepath.ToSlash(relToSource)
						modified = true
						report.FixedLinksCount++
						return fmt.Sprintf("[%s](%s)", label, relToSource)
					}
				} else {
					report.BrokenLinksCount++
					brokenLinkDetails = append(brokenLinkDetails, fmt.Sprintf("In %s: %s", filepath.Base(n.RelPath), targetName))
				}
				return match
			})

			if modified && !dryRun && autoFix {
				if err := safefile.WriteFile(e.vaultRoot, n.AbsPath, []byte(newContent), 0600); err != nil {
					return report, fmt.Errorf("save repaired note: %w", err)
				}
			}
		}

		if report.BrokenLinksCount > 0 {
			contextStr := strings.Join(brokenLinkDetails, "\n")
			if len(contextStr) > 500 {
				contextStr = contextStr[:497] + "..."
			}
			report.HandoffQuestions = append(report.HandoffQuestions, HandoffQuestion{
				Topic:    "Broken Links",
				Type:     "graph_hygiene",
				Question: fmt.Sprintf("Found %d broken links. Would you like to review and fix them?", report.BrokenLinksCount),
				Context:  contextStr,
			})
		}
	}

	// Sync MOCs
	missingMOCs, err := SyncMOCs(e.vaultRoot, autoFix && !dryRun)
	if err != nil {
		return report, err
	}
	report.MissingMOCs = missingMOCs

	// Graph Density Audit
	denseCats, _ := AuditGraphDensity(e.vaultRoot)
	report.DenseCategories = denseCats

	// Workspace Cataloging
	wsScanner := workspace.NewScanner("", e.vaultRoot)
	wsRep, err := wsScanner.Scan(false)
	if err == nil {
		report.WorkspaceSummary = wsRep
		if autoFix && !dryRun {
			mocContent := wsScanner.GenerateMOCContent(wsRep)
			if err := safefile.WriteFile(e.vaultRoot, wsRep.MOCFilePath, []byte(mocContent), 0600); err != nil {
				return report, fmt.Errorf("save workspace catalog: %w", err)
			}
		}
	}
	p1Duration := time.Since(startPhase1)

	// 2. Layer 2: Strategic Radar & Staff Drift
	startPhase2 := time.Now()
	// Daily Telemetry
	dailyFile := filepath.Join(e.vaultRoot, "daily", targetDate+".md")
	if data, err := os.ReadFile(dailyFile); err == nil { //nolint:gosec // intentional
		fmRegex := regexp.MustCompile(`(?s)^---\s*\n(.*?)\n---\s*\n`)
		if m := fmRegex.FindStringSubmatch(string(data)); len(m) > 1 {
			var rawFM map[string]interface{}
			if err := yaml.Unmarshal([]byte(m[1]), &rawFM); err == nil {
				cfg, _ := config.LoadConfig(e.vaultRoot, "", false)
				archetypes := cfg.GetTaxonomyArchetypes()

				report.DailyTelemetry = make(map[string]interface{})
				report.DailyTelemetry["file"] = fmt.Sprintf("daily/%s.md", targetDate)

				totalH := 0.0
				for _, arch := range archetypes {
					key := fmt.Sprintf("tempo-%s", arch.ID)
					t := toFloat(rawFM[key])
					if t == 0 {
						t = toFloat(rawFM[arch.ID])
					}
					report.DailyTelemetry[arch.ID] = t
					totalH += t
				}
				report.DailyTelemetry["total_hours"] = roundFloat(totalH, 1)
			}
		}
	}

	// 7-day Temporal Analysis
	tempAnalysis, _ := AnalyzeTemporalWindow(e.vaultRoot, targetDate, 7)
	report.TemporalAnalysis = tempAnalysis

	// Sync People interactions
	if autoFix && !dryRun {
		syncedPeople, err := SyncPeopleInteractionsFromDaily(e.vaultRoot, targetDate)
		if err != nil {
			return report, err
		}
		report.SyncedPeople = syncedPeople
	}
	p2Duration := time.Since(startPhase2)

	// 3. Layer 3: Triage, Handoff & Memory
	startPhase3 := time.Now()
	// Inbox Triage & Agent Session Ingestion
	inboxTriaged := TriageInboxItems(e.vaultRoot, learnings)
	report.InboxTriaged = inboxTriaged
	report.RecentSessions = TriageSessionLogs(e.vaultRoot)

	// Emerging Concepts
	emerging := DiscoverEmergingConcepts(e.vaultRoot, targetDate)
	report.EmergingConcepts = emerging

	// Strategic Radar
	stratRadar := CrossSynthesizeTechBusiness(e.vaultRoot, targetDate, tempAnalysis, emerging, learnings)
	report.StrategicRadar = stratRadar

	// Extract Backlog from sumario-semanal
	sumarioFile := filepath.Join(e.vaultRoot, "notes", "carreira", "sumario-semanal-atividades-impactos-2026.md")
	if _, err := os.Stat(sumarioFile); os.IsNotExist(err) {
		sumarioFile = filepath.Join(e.vaultRoot, "meta", "carreira", "sumario-semanal-atividades-impactos-2026.md")
	}
	if data, err := os.ReadFile(sumarioFile); err == nil { //nolint:gosec // intentional
		lines := strings.Split(string(data), "\n")
		currentPriority := ""

		for _, l := range lines {
			trimmed := strings.TrimSpace(l)
			if strings.HasPrefix(trimmed, "###") || strings.HasPrefix(trimmed, "## ") {
				if strings.Contains(trimmed, "P0") || strings.Contains(trimmed, "🔴") {
					currentPriority = "P0"
				} else if strings.Contains(trimmed, "P1") || strings.Contains(trimmed, "🟡") {
					currentPriority = "P1"
				} else {
					currentPriority = ""
				}
				continue
			}

			if strings.HasPrefix(trimmed, "- [ ]") {
				switch currentPriority {
				case "P0":
					report.BacklogP0 = append(report.BacklogP0, trimmed)
				case "P1":
					report.BacklogP1 = append(report.BacklogP1, trimmed)
				}
			}
		}
	}

	// Generate Handoff Questions
	for _, ec := range emerging {
		report.HandoffQuestions = append(report.HandoffQuestions, HandoffQuestion{
			Topic:    ec.Title,
			Type:     "emerging_concept",
			Question: fmt.Sprintf("Would you like to scaffold a technical note in notes/ for '%s'?", ec.Title),
			Context:  ec.Context,
		})
	}
	for _, it := range inboxTriaged {
		if it.Confidence != "HIGH" {
			report.HandoffQuestions = append(report.HandoffQuestions, HandoffQuestion{
				Topic:    it.File,
				Type:     "inbox_triage",
				Question: fmt.Sprintf("Promote draft '%s' to '%s' or keep in inbox?", it.File, it.SuggestedCategory),
				Context:  it.Snippet,
			})
		}
	}

	jiraRegex := regexp.MustCompile(`(?:\[|\b)([A-Z]{2,}-[0-9]+)(?:\]|\b)`)
	for _, bl := range append(report.BacklogP0, report.BacklogP1...) {
		matches := jiraRegex.FindAllStringSubmatch(bl, -1)
		for _, m := range matches {
			if len(m) > 1 {
				ticketID := m[1]
				report.HandoffQuestions = append(report.HandoffQuestions, HandoffQuestion{
					Topic:    ticketID,
					Type:     "jira_sync",
					Question: fmt.Sprintf("Ticket %s was found in your active backlog. Is it already closed in Jira? Should we mark it as done?", ticketID),
					Context:  bl,
				})
			}
		}
	}
	p3Duration := time.Since(startPhase3)

	// 4. Layer 4: Metacognition & Self-Evaluation
	startPhase4 := time.Now()
	timings := make([]StageTiming, 0, 5)
	timings = append(timings,
		StageTiming{
			Name:        "Phase 1: Deterministic Graph Hygiene",
			DurationMs:  p1Duration.Milliseconds(),
			ItemsCount:  report.TotalFilesScanned,
			Description: fmt.Sprintf("%d files scanned, %d fixed links", report.TotalFilesScanned, report.FixedLinksCount),
		},
		StageTiming{
			Name:        "Phase 2: Strategic Telemetry & Radar",
			DurationMs:  p2Duration.Milliseconds(),
			ItemsCount:  len(report.SyncedPeople),
			Description: fmt.Sprintf("%d people synced, radar compiled", len(report.SyncedPeople)),
		},
		StageTiming{
			Name:        "Phase 3: Zettelkasten Inbox Triage",
			DurationMs:  p3Duration.Milliseconds(),
			ItemsCount:  len(report.InboxTriaged),
			Description: fmt.Sprintf("%d drafts triaged, %d emerging concepts", len(report.InboxTriaged), len(report.EmergingConcepts)),
		},
	)
	metaRep := RunMetacognitivePass(e.vaultRoot, report, timings)
	p4Duration := time.Since(startPhase4)

	// 5. Layer 5: Active Zettelkasten Knowledge Synthesis
	startPhase5 := time.Now()
	var syntheses []SynthesisDraft
	if e.synthesize {
		syntheses, err = InduceKnowledgeSyntheses(e.vaultRoot, report, autoFix, dryRun)
		if err != nil {
			return report, err
		}
	}
	metaRep.Syntheses = syntheses
	p5Duration := time.Since(startPhase5)

	timings = append(timings, StageTiming{
		Name:        "Phase 4: Metacognition & Self-Evaluation",
		DurationMs:  p4Duration.Milliseconds(),
		ItemsCount:  len(metaRep.Insights),
		Description: fmt.Sprintf("%d insights, %d bottlenecks flagged", len(metaRep.Insights), len(metaRep.Bottlenecks)),
	})
	timings = append(timings, StageTiming{
		Name:        "Phase 5: Knowledge Synthesis Induction",
		DurationMs:  p5Duration.Milliseconds(),
		ItemsCount:  len(syntheses),
		Description: fmt.Sprintf("%d cross-domain synthesis notes induced", len(syntheses)),
	})
	metaRep.Timings = timings
	metaRep.TotalDurationMs = p1Duration.Milliseconds() + p2Duration.Milliseconds() + p3Duration.Milliseconds() + p4Duration.Milliseconds() + p5Duration.Milliseconds()
	report.Metacognition = metaRep

	// Persist derived artifacts only for an executing cycle.
	if !dryRun {
		handoffData, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return report, err
		}
		handoffPath := filepath.Join(e.vaultRoot, ".daneel", "cache", "dream_handoff.json")
		if err := safefile.WriteFile(e.vaultRoot, handoffPath, handoffData, 0600); err != nil {
			return report, fmt.Errorf("save dream handoff: %w", err)
		}
		if err := AppendEvolutionLog(e.vaultRoot, targetDate, report); err != nil {
			return report, err
		}
		if err := UpdateDashboardMetricas(e.vaultRoot, report, len(notes)); err != nil {
			return report, err
		}
	}

	return report, nil
}

func toFloat(v interface{}) float64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case string:
		f, _ := strconv.ParseFloat(val, 64)
		return f
	}
	return 0
}

func roundFloat(val float64, precision uint) float64 {
	ratio := 1.0
	for i := uint(0); i < precision; i++ {
		ratio *= 10.0
	}
	return float64(int(val*ratio+0.5)) / ratio
}

// UpdateDashboardMetricas generates a beautiful markdown dashboard and overwrites meta/dashboard-metricas.md
func UpdateDashboardMetricas(vaultRoot string, report *FullDreamReport, totalNotes int) error {
	var sb strings.Builder

	sb.WriteString("# 🦅 Painel Central do Cérebro (Auto-Gerado pelo Dream Cycle)\n\n")
	fmt.Fprintf(&sb, "*Última atualização autônoma:* %s\n\n", report.GeneratedAt)

	sb.WriteString("## 🏥 1. Inventário & Saúde do Zettelkasten\n")
	fmt.Fprintf(&sb, "- **Total de Entradas (Todas as Pastas):** %d notas escaneadas\n", totalNotes)

	if report.WorkspaceSummary != nil {
		fmt.Fprintf(&sb, "- **Tamanho do Workspace Local:** %d repositórios em disco\n", report.WorkspaceSummary.TotalRepos)
	}
	fmt.Fprintf(&sb, "- **Limpeza e Fixação de Links:** %d links curados no último ciclo\n", report.FixedLinksCount)
	if report.BrokenLinksCount > 0 {
		fmt.Fprintf(&sb, "- **⚠️ Alerta de Débito:** %d links quebrados encontrados\n", report.BrokenLinksCount)
	}
	sb.WriteString("\n")

	sb.WriteString("## 📊 2. Alocação de Staff Eng (Janela Deslizante de 7 Dias)\n")
	if report.TemporalAnalysis != nil {
		fmt.Fprintf(&sb, "- **Volume de Trabalho Registrado:** %.1fh em %d dias ativos\n", report.TemporalAnalysis.TotalHours, report.TemporalAnalysis.DaysWithLogs)
		fmt.Fprintf(&sb, "- **[A] Estratégico:** %.1f%% (Meta: >= 30%%)\n", report.TemporalAnalysis.Percentages["A"])
		fmt.Fprintf(&sb, "- **[B] Impacto Local:** %.1f%% (Meta: >= 30%%)\n", report.TemporalAnalysis.Percentages["B"])
		fmt.Fprintf(&sb, "- **[C] Glue Work:** %.1f%% (Meta: <= 30%%)\n", report.TemporalAnalysis.Percentages["C"])
		fmt.Fprintf(&sb, "- **[E] Overhead:** %.1f%% (Meta: <= 10%%)\n", report.TemporalAnalysis.Percentages["E"])
	} else {
		sb.WriteString("- *Dados de telemetria indisponíveis para a janela de 7 dias.*\n")
	}
	sb.WriteString("\n")

	// 3. Preserve Section 3 (Telemetria de Agentes & Evals) if present in existing dashboard
	dashPath := filepath.Join(vaultRoot, "meta", "dashboard-metricas.md")
	if data, err := os.ReadFile(dashPath); err == nil { //nolint:gosec // intentional
		evalRegex := regexp.MustCompile(`(?s)(## 🤖 3\. Telemetria de Agentes & Evals.*?(?:\n## |\z))`)
		if m := evalRegex.FindStringSubmatch(string(data)); len(m) > 1 {
			sb.WriteString(m[1])
			if !strings.HasSuffix(m[1], "\n") {
				sb.WriteString("\n")
			}
			if !strings.HasSuffix(m[1], "\n\n") {
				sb.WriteString("\n")
			}
		}
	}

	// 4. Section 4: Metacognição & Evolução Autônoma do Agente
	if report.Metacognition != nil {
		sb.WriteString("## 🧠 4. Metacognição & Evolução Autônoma do Agente\n")
		fmt.Fprintf(&sb, "- **Tempo Total do Ciclo:** %dms\n", report.Metacognition.TotalDurationMs)
		fmt.Fprintf(&sb, "- **Gargalos Detectados:** %d\n", len(report.Metacognition.Bottlenecks))
		fmt.Fprintf(&sb, "- **Sínteses de Conhecimento Geradas:** %d\n", len(report.Metacognition.Syntheses))
		sb.WriteString("- **Diário de Evolução:** [[meta/guias/agente-evolucao.md]]\n\n")
	}

	// 5. Section 5: Telemetria Operacional, Custos de IA (FinOps) & Uso do Vault
	sb.WriteString("## 💰 5. Telemetria Operacional, Custos de IA (FinOps) & Uso do Vault\n")
	store, storeErr := routine.OpenDefaultStore(vaultRoot)
	if storeErr == nil && store != nil {
		defer func() { _ = store.Close() }()
		ctx := context.Background()
		parserOpts := telemetry.ParserOptions{
			Since: 7 * 24 * time.Hour,
		}
		if vaultRoot != "" {
			brainDir := filepath.Join(vaultRoot, "brain")
			if fi, err := os.Stat(brainDir); err == nil && fi.IsDir() {
				parserOpts.BrainDir = brainDir
			}
		}
		summaries, _ := telemetry.SyncSessionsToStore(ctx, store, parserOpts)

		since7d := time.Now().Add(-7 * 24 * time.Hour).Unix()
		sessions, _ := store.GetRecentSessionsTelemetry(ctx, since7d, 200)

		var totalSessions, totalSubagents int
		var totalIn, totalCache, totalOut int64
		var totalCostUSD float64

		for _, s := range sessions {
			totalSessions++
			totalSubagents += s.SubagentsCount
			totalIn += s.InputTokens
			totalCache += s.CacheReadTokens
			totalOut += s.OutputTokens
			totalCostUSD += s.EstimatedCostUSD
		}
		if len(sessions) == 0 && len(summaries) > 0 {
			for _, sum := range summaries {
				totalSessions++
				totalSubagents += sum.Record.SubagentsCount
				totalIn += sum.Record.InputTokens
				totalCache += sum.Record.CacheReadTokens
				totalOut += sum.Record.OutputTokens
				totalCostUSD += sum.Record.EstimatedCostUSD
			}
		}

		cacheRate := 0.0
		if totalIn+totalCache > 0 {
			cacheRate = float64(totalCache) / float64(totalIn+totalCache) * 100.0
		}

		dreamCost := 0.0
		if report.Metacognition != nil {
			dreamCost = report.Metacognition.EstimatedCostUSD
		}

		fmt.Fprintf(&sb, "- **Sessões de IA & Subagentes (7d):** %d sessões, %d subagentes disparados\n", totalSessions, totalSubagents)
		fmt.Fprintf(&sb, "- **Consumo de Tokens (7d):** %d in / %d cache read / %d out (Taxa de Cache: %.1f%%)\n", totalIn, totalCache, totalOut, cacheRate)
		fmt.Fprintf(&sb, "- **Custo Estimado de IA (7d):** $%.4f USD (Último Dream Cycle: $%.4f USD)\n", totalCostUSD, dreamCost)

		// Top Heaviest Routines
		if weights, err := store.GetRoutineWeightAggregates(ctx, since7d); err == nil && len(weights) > 0 {
			sb.WriteString("- **Rotinas Mais Pesadas (7d):**\n")
			for i, w := range weights {
				if i >= 3 {
					break
				}
				fmt.Fprintf(&sb, "  - **%s:** %.0fms méd (%d execuções, %d itens, $%.4f USD)\n",
					w.RoutineName, w.AvgDurationMs, w.TotalRuns, w.TotalItems, w.TotalCostUSD)
			}
		}

		// Top Vault Primitives
		if cmds, err := store.GetCommandAggregates(ctx, since7d, 5); err == nil && len(cmds) > 0 {
			sb.WriteString("- **Primitivas do Vault Mais Utilizadas (CLI / MCP):**\n")
			for i, c := range cmds {
				if i >= 3 {
					break
				}
				fmt.Fprintf(&sb, "  - `[%s] %s`: %d chamadas (latência média: %.1fms)\n",
					strings.ToUpper(c.Surface), c.Command, c.TotalCalls, c.AvgDurationMs)
			}
		}
	} else {
		sb.WriteString("- *Dados de telemetria operacional indisponíveis.*\n")
	}
	sb.WriteString("\n")

	return safefile.WriteFile(vaultRoot, dashPath, []byte(sb.String()), 0600)
}
