//nolint:revive // package comment not required
package auditor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/rjfonseca/daneel/internal/config"
	"github.com/rjfonseca/daneel/internal/dream"
	"github.com/rjfonseca/daneel/internal/search"
)

var (
	// questionDetectRegex identifies question marks or interrogation starters in agent responses.
	questionDetectRegex = regexp.MustCompile(`(?i)(\?|\b(qual|como|onde|quem|quando|qual é|qual o|quais|qual a|me confirme|você pode me informar|pode confirmar|pode me passar|what|where|how|which|who|can you provide|could you clarify|please specify|do you know)\b)`)

	// conversationalPreambleRegex strips filler phrases when extracting central queries.
	conversationalPreambleRegex = regexp.MustCompile(`(?i)^(como (funciona|configuro|rodo|executo|faço)|onde (fica|está|encontro)|qual (é o|é a|são os|são as|o|a)|what is the|how does|where is the|how to configure|can you explain)\s+`)
)

// AgentEvaluator coordinates Phase 1 (deterministic filter) and Phase 2 (vault validation) for agent evals.
type AgentEvaluator struct {
	engine  *search.Engine
	scanner *MultiScanner
	opts    AgentEvalOptions
}

// NewAgentEvaluator creates an AgentEvaluator with scanners and options.
func NewAgentEvaluator(engine *search.Engine, opts AgentEvalOptions) *AgentEvaluator {
	if opts.Since <= 0 {
		opts.Since = 48 * time.Hour
	}
	if opts.AgentFilter == "" {
		opts.AgentFilter = "all"
	}

	scanner := NewDefaultMultiScanner(opts.BrainDir, opts.ClaudeDir, opts.CursorDir)

	return &AgentEvaluator{
		engine:  engine,
		scanner: scanner,
		opts:    opts,
	}
}

// Evaluate scans agent sessions, applies deterministic filtering, and validates against the vault.
func (e *AgentEvaluator) Evaluate(ctx context.Context) (*AgentEvalReport, error) {
	report := &AgentEvalReport{
		Since:       e.opts.Since.String(),
		AgentFilter: e.opts.AgentFilter,
		ByAgent:     make(map[string]AgentEvalSummary),
		Results:     []AgentEvalResult{},
	}

	// 1. Scan Agnostic Transcripts
	interactions, err := e.scanner.ScanRecent(e.opts.Since, e.opts.AgentFilter)
	if err != nil {
		return nil, fmt.Errorf("failed to scan agent transcripts: %w", err)
	}

	report.ScannedInteractions = len(interactions)

	// Initialize summary maps per agent
	for _, it := range interactions {
		summary := report.ByAgent[it.AgentName]
		summary.ScannedInteractions++
		report.ByAgent[it.AgentName] = summary
	}

	// 2. Phase 1: Deterministic Filter (Deltas Suspeitos)
	deltas := FindSuspiciousDeltas(interactions)
	report.SuspiciousDeltas = len(deltas)

	for _, d := range deltas {
		summary := report.ByAgent[d.Interaction.AgentName]
		summary.SuspiciousDeltas++
		report.ByAgent[d.Interaction.AgentName] = summary
	}

	// 3. Phase 2: Classification Engine (Validation against Vault SQLite FTS5)
	results, err := e.EvaluateDeltas(ctx, deltas)
	if err != nil {
		return nil, fmt.Errorf("evals classification failed: %w", err)
	}

	report.Results = results
	for _, res := range results {
		summary := report.ByAgent[res.AgentName]
		switch res.Classification {
		case ClassificationMissedOpportunity:
			report.MissedOpportunities++
			summary.MissedOpportunities++
		case ClassificationKnowledgeGap:
			report.KnowledgeGaps++
			summary.KnowledgeGaps++
		}
		report.ByAgent[res.AgentName] = summary
	}

	// 4. Persistence if --apply is requested (Vault markdown files)
	if e.opts.Apply && !e.opts.DryRun {
		_ = e.ApplyReport(ctx, report)
	}

	// 5. Always persist telemetry cache for ConnectRPC / Cockpit
	if e.opts.VaultRoot != "" {
		_ = persistTelemetryCache(e.opts.VaultRoot, report)
	}

	return report, nil
}

// FindSuspiciousDeltas performs Phase 1 deterministic filtering on interactions.
func FindSuspiciousDeltas(interactions []Interaction) []SuspiciousDelta {
	var deltas []SuspiciousDelta

	for _, it := range interactions {
		// Tool Negation Check: If the agent already queried the vault, it's not a missed search opportunity
		if HasVaultSearchTool(it.ToolsUsed) {
			continue
		}

		// Agent Question / Clarification Check
		if !questionDetectRegex.MatchString(it.AgentReply) {
			continue
		}

		// User prompt must have substantive content (> 5 characters)
		if len(strings.TrimSpace(it.UserPrompt)) < 5 {
			continue
		}

		query := ExtractCentralQuery(it.UserPrompt, it.AgentReply)
		if query == "" {
			continue
		}

		deltas = append(deltas, SuspiciousDelta{
			Interaction:    it,
			ExtractedQuery: query,
			Reason:         "Agent interrogated user without querying vault tools",
		})
	}

	return deltas
}

var stopWords = map[string]bool{
	"a": true, "as": true, "o": true, "os": true, "um": true, "uma": true, "uns": true, "umas": true,
	"de": true, "do": true, "da": true, "dos": true, "das": true, "em": true, "no": true, "na": true,
	"nos": true, "nas": true, "por": true, "pelo": true, "pela": true, "pelos": true, "pelas": true,
	"para": true, "pra": true, "pro": true, "com": true, "sem": true, "sob": true, "sobre": true,
	"e": true, "ou": true, "se": true, "mas": true, "que": true, "qual": true, "quais": true,
	"como": true, "onde": true, "quando": true, "quem": true, "quanto": true, "quantos": true,
	"é": true, "era": true, "foi": true, "ser": true, "estar": true, "está": true, "estão": true,
	"tem": true, "têm": true, "ter": true, "fica": true, "ficam": true, "são": true,
	"the": true, "an": true, "of": true, "in": true, "on": true, "at": true, "to": true, "for": true,
	"with": true, "from": true, "by": true, "is": true, "are": true, "was": true, "were": true,
	"what": true, "where": true, "when": true, "who": true, "how": true, "which": true,
}

// ExtractSubstantiveTerms filters out stopwords and returns key terms.
func ExtractSubstantiveTerms(query string) []string {
	wordReg := regexp.MustCompile(`[a-zA-Z0-9_\-]+`)
	words := wordReg.FindAllString(query, -1)
	var terms []string
	for _, w := range words {
		lw := strings.ToLower(w)
		if len(lw) >= 3 && !stopWords[lw] {
			terms = append(terms, w)
		}
	}
	return terms
}

// EvaluateDeltas validates suspicious deltas against SQLite FTS5 to classify Missed Opportunities vs. Knowledge Gaps.
func (e *AgentEvaluator) EvaluateDeltas(ctx context.Context, deltas []SuspiciousDelta) ([]AgentEvalResult, error) {
	var results []AgentEvalResult

	for _, d := range deltas {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		query := d.ExtractedQuery
		substantive := ExtractSubstantiveTerms(query)
		var matchedNotes []string

		if e.engine != nil && len(substantive) > 0 {
			searchQuery := strings.Join(substantive, " ")
			// 1. Faceted full-text search
			searchResults, err := e.engine.SearchFaceted(ctx, search.SearchOptions{
				Query: searchQuery,
				Limit: 3,
			})
			if err == nil {
				for _, sr := range searchResults {
					if sr.RelPath != "" {
						// Verify that at least one substantive term is genuinely matched
						srLower := strings.ToLower(sr.RelPath + " " + sr.Title + " " + sr.Snippet)
						matchFound := false
						for _, st := range substantive {
							if strings.Contains(srLower, strings.ToLower(st)) {
								matchFound = true
								break
							}
						}
						if matchFound {
							matchedNotes = append(matchedNotes, sr.RelPath)
						}
					}
				}
			}

			// 2. Fallback to exact or fuzzy entity resolution
			if len(matchedNotes) == 0 {
				if res, err := e.engine.ResolveNote(ctx, query); err == nil && res != nil && res.Found && res.Match != nil && res.Match.RelPath != "" {
					matchedNotes = append(matchedNotes, res.Match.RelPath)
				}
			}
		}

		if len(matchedNotes) > 0 {
			// Scenario A: Information exists in the Vault -> Missed Opportunity (Agent Failure)
			results = append(results, AgentEvalResult{
				Classification:  ClassificationMissedOpportunity,
				AgentName:       d.Interaction.AgentName,
				ConversationID:  d.Interaction.ConversationID,
				Timestamp:       d.Interaction.Timestamp,
				UserPrompt:      d.Interaction.UserPrompt,
				AgentReply:      d.Interaction.AgentReply,
				Doubt:           query,
				MatchedNotes:    matchedNotes,
				SuggestedAction: fmt.Sprintf("Refine system prompt for %s: enforce proactive vault search (search_vault/resolve_note) before asking the user.", d.Interaction.AgentName),
				Details:         fmt.Sprintf("Relevant documentation exists in: %s", strings.Join(matchedNotes, ", ")),
			})
		} else {
			// Scenario B: Information does NOT exist in the Vault -> Knowledge Gap (Vault Failure)
			results = append(results, AgentEvalResult{
				Classification:  ClassificationKnowledgeGap,
				AgentName:       d.Interaction.AgentName,
				ConversationID:  d.Interaction.ConversationID,
				Timestamp:       d.Interaction.Timestamp,
				UserPrompt:      d.Interaction.UserPrompt,
				AgentReply:      d.Interaction.AgentReply,
				Doubt:           query,
				SuggestedAction: fmt.Sprintf("Create note or develop repository Harvester for missing domain knowledge '%s'.", query),
				Details:         fmt.Sprintf("No relevant notes found in Vault for query '%s'.", query),
			})
		}
	}

	return results, nil
}

// ExtractCentralQuery extracts the core technical entity or doubt from the conversation turn.
func ExtractCentralQuery(userPrompt, agentReply string) string {
	cleanPrompt := strings.TrimSpace(userPrompt)

	// 1. Remove markdown tags or code blocks if present
	cleanPrompt = regexp.MustCompile("(?s)```.*?```").ReplaceAllString(cleanPrompt, "")
	cleanPrompt = regexp.MustCompile(`(?s)<.*?>`).ReplaceAllString(cleanPrompt, "")
	cleanPrompt = strings.TrimSpace(cleanPrompt)

	// First line or first sentence
	lines := strings.Split(cleanPrompt, "\n")
	firstLine := strings.TrimSpace(lines[0])

	// Strip common filler preambles
	stripped := conversationalPreambleRegex.ReplaceAllString(firstLine, "")
	stripped = strings.Trim(stripped, "?.!;: ")

	// Strip leading articles
	leadingArticleRegex := regexp.MustCompile(`(?i)^(o|a|os|as|the|an)\s+`)
	stripped = leadingArticleRegex.ReplaceAllString(stripped, "")
	stripped = strings.Trim(stripped, "?.!;: ")

	if len(stripped) >= 3 && len(stripped) <= 80 {
		return stripped
	}

	if len(stripped) > 80 {
		// Truncate cleanly at word boundary
		idx := strings.LastIndexAny(stripped[:80], " ,.-_")
		if idx > 20 {
			return strings.TrimSpace(stripped[:idx])
		}
		return strings.TrimSpace(stripped[:80])
	}

	// Fallback to agent question subject if prompt was too brief
	if len(agentReply) > 0 {
		cleanReply := strings.TrimSpace(agentReply)
		qIdx := strings.Index(cleanReply, "?")
		if qIdx > 0 {
			qSentence := cleanReply[:qIdx]
			qStripped := conversationalPreambleRegex.ReplaceAllString(qSentence, "")
			qStripped = strings.Trim(qStripped, "?.!;: ")
			if len(qStripped) >= 3 && len(qStripped) <= 80 {
				return qStripped
			}
		}
	}

	return cleanPrompt
}

// ApplyReport writes agent eval results into agent learnings and dashboard metrics.
func (e *AgentEvaluator) ApplyReport(ctx context.Context, report *AgentEvalReport) error {
	if e.opts.VaultRoot == "" {
		return fmt.Errorf("vault root not configured, cannot persist agent evals")
	}

	// 1. Record each finding as cognitive feedback in meta/guias/aprendizados-agente.md
	for _, res := range report.Results {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		feedbackType := string(res.Classification)
		err := dream.RecordFeedbackWithAgent(
			e.opts.VaultRoot,
			res.Doubt,
			feedbackType,
			res.SuggestedAction,
			res.Details,
			res.AgentName,
		)
		if err == nil {
			report.AppliedCount++
		}
	}

	// 2. Sync to meta/dashboard-metricas.md
	UpdateDashboardMetricasEvals(e.opts.VaultRoot, report)

	return nil
}

// UpdateDashboardMetricasEvals updates or appends the Agent Evals telemetry section in meta/dashboard-metricas.md.
func UpdateDashboardMetricasEvals(vaultRoot string, report *AgentEvalReport) {
	dashPath := filepath.Join(vaultRoot, "meta", "dashboard-metricas.md")
	data, err := os.ReadFile(dashPath) //nolint:gosec
	if err != nil {
		return
	}

	var sb strings.Builder
	sb.WriteString("## 🤖 3. Telemetria de Agentes & Evals (Missed Opportunities)\n")
	fmt.Fprintf(&sb, "- **Última Avaliação:** %s (Janela: %s)\n", time.Now().Format("2006-01-02 15:04"), report.Since)
	fmt.Fprintf(&sb, "- **Total de Interações Escaneadas:** %d turnos\n", report.ScannedInteractions)
	fmt.Fprintf(&sb, "- **Deltas Suspeitos Identificados:** %d\n", report.SuspiciousDeltas)
	fmt.Fprintf(&sb, "- **Missed Opportunities (Falha de Autonomia do Agente):** %d\n", report.MissedOpportunities)
	fmt.Fprintf(&sb, "- **Knowledge Gaps (Falha de Cobertura do Cofre):** %d\n", report.KnowledgeGaps)

	content := string(data)
	evalsSectionRegex := regexp.MustCompile(`(?s)## 🤖 3\. Telemetria de Agentes & Evals.*?(?:\n## |\z)`)

	if evalsSectionRegex.MatchString(content) {
		content = evalsSectionRegex.ReplaceAllStringFunc(content, func(m string) string {
			if strings.HasSuffix(m, "\n## ") {
				return sb.String() + "\n## "
			}
			return sb.String() + "\n"
		})
	} else {
		content = strings.TrimRight(content, "\n") + "\n\n" + sb.String() + "\n"
	}

	_ = os.WriteFile(dashPath, []byte(content), 0600) //nolint:gosec // intentional
}

// persistTelemetryCache saves the evaluation snapshot and feedback history into .daneel/cache/agent_learnings.json.
func persistTelemetryCache(vaultRoot string, report *AgentEvalReport) error {
	learnings := dream.LoadLearnings(vaultRoot)
	if learnings == nil {
		learnings = dream.DefaultLearnings()
	}

	learnings.LastUpdated = time.Now().Format(time.RFC3339)
	learnings.ScannedTurns = report.ScannedInteractions
	learnings.SuspiciousDeltas = report.SuspiciousDeltas
	learnings.MissedOpportunities = report.MissedOpportunities
	learnings.KnowledgeGaps = report.KnowledgeGaps

	// Synchronize feedback history with report results, avoiding duplicates
	for _, res := range report.Results {
		exists := false
		for _, existing := range learnings.FeedbackHistory {
			if existing.Topic == res.Doubt && (existing.Type == string(res.Classification) || existing.Action == string(res.Classification)) {
				exists = true
				break
			}
		}

		if !exists {
			dateStr := res.Timestamp.Format("2006-01-02")
			if res.Timestamp.IsZero() {
				dateStr = time.Now().Format("2006-01-02")
			}
			timeStr := res.Timestamp.Format(time.RFC3339)
			if res.Timestamp.IsZero() {
				timeStr = time.Now().Format(time.RFC3339)
			}

			learnings.FeedbackHistory = append(learnings.FeedbackHistory, dream.FeedbackEntry{
				Date:      dateStr,
				Timestamp: timeStr,
				Topic:     res.Doubt,
				Type:      string(res.Classification),
				Action:    res.SuggestedAction,
				Note:      res.Details,
				Agent:     res.AgentName,
			})
		}
	}

	cacheDir := config.ResolveVaultCacheDir(vaultRoot)
	_ = os.MkdirAll(cacheDir, 0750)
	cachePath := filepath.Join(cacheDir, "agent_learnings.json")
	data, err := json.MarshalIndent(learnings, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(cachePath, data, 0600); err != nil {
		fallbackDir := config.XDGCacheDir()
		_ = os.MkdirAll(fallbackDir, 0750)
		fallbackPath := filepath.Join(fallbackDir, "agent_learnings.json")
		return os.WriteFile(fallbackPath, data, 0600)
	}
	return nil
}
