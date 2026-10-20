//nolint:revive // package comment not required
package auditor

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/rjfonseca/daneel/internal/dream"
)

var (
	correctionRegex = regexp.MustCompile(`(?i)(você demorou|não é aí|o caminho correto|o padrão é|deveria estar em|lembre-se de|nunca faça|não faça|está errado|corrija|não comite|use sempre|sempre use|atenção para|por favor lembre|evite|o certo é|regra:)`)

	imperativeVerbRegex = regexp.MustCompile(`(?i)\b(pesquise|busque|procure|rode|execute|crie|gere|gerar|escreva|implemente|compile|teste|verifique|valide|suba|atualize|analise|extraia|search|run|execute|create|generate|write|implement|build|test|check|verify|update|analyze)\b`)

	userRequestTagRegex = regexp.MustCompile(`(?s)<USER_REQUEST>(.*?)</USER_REQUEST>`)
)

type transcriptLine struct {
	StepIndex int    `json:"step_index"`
	Source    string `json:"source"`
	Type      string `json:"type"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
	Content   string `json:"content"`
	ToolCalls []struct {
		Name string `json:"name"`
	} `json:"tool_calls"`
}

// SessionMiner reads agent transcripts and extracts learnings, skill proposals, and evals.
type SessionMiner struct {
	opts AuditOptions
}

// NewSessionMiner creates a new SessionMiner with the provided options.
func NewSessionMiner(opts AuditOptions) *SessionMiner {
	if opts.Since <= 0 {
		opts.Since = 48 * time.Hour
	}
	if opts.BrainDir == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			opts.BrainDir = filepath.Join(home, ".gemini", "antigravity-cli", "brain")
		}
	}
	return &SessionMiner{opts: opts}
}

// Audit scans recent sessions and produces an AuditReport.
func (m *SessionMiner) Audit(ctx context.Context) (*AuditReport, error) {
	report := &AuditReport{
		Since:          m.opts.Since.String(),
		Learnings:      []ExtractedLearning{},
		SkillProposals: []SkillProposal{},
	}

	if _, err := os.Stat(m.opts.BrainDir); os.IsNotExist(err) {
		return report, nil
	}

	entries, err := os.ReadDir(m.opts.BrainDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read brain directory: %w", err)
	}

	cutoff := time.Now().Add(-m.opts.Since)
	workflowPatterns := make(map[string][]string)

	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		if !entry.IsDir() {
			continue
		}

		convID := entry.Name()
		convDir := filepath.Join(m.opts.BrainDir, convID)

		transcriptPath := filepath.Join(convDir, ".system_generated", "logs", "transcript.jsonl")
		if _, err := os.Stat(transcriptPath); os.IsNotExist(err) {
			transcriptPath = filepath.Join(convDir, "transcript.jsonl")
			if _, err := os.Stat(transcriptPath); os.IsNotExist(err) {
				continue
			}
		}

		turns, err := m.parseTranscript(transcriptPath, convID, cutoff)
		if err != nil || len(turns) == 0 {
			continue
		}

		report.ScannedConversations++
		report.ScannedTurns += len(turns)

		for _, turn := range turns {
			// 1. Check for User Corrections
			if correctionRegex.MatchString(turn.UserPrompt) {
				topic := extractTopic(turn.UserPrompt)
				desc := cleanPrompt(turn.UserPrompt)
				if len(desc) > 200 {
					desc = desc[:200] + "..."
				}

				report.Learnings = append(report.Learnings, ExtractedLearning{
					SignalType:      SignalCorrection,
					Topic:           topic,
					Description:     desc,
					SuggestedAction: "Sincronizar em meta/guias/aprendizados-agente.md",
					ConversationID:  convID,
					Timestamp:       turn.Timestamp,
				})
			}

			// 2. Multi-step Imperative Workflows (Skill Mining)
			verbs := imperativeVerbRegex.FindAllString(turn.UserPrompt, -1)
			if len(verbs) >= 2 {
				cleanText := cleanPrompt(turn.UserPrompt)
				normKey := normalizeWorkflowKey(verbs)
				workflowPatterns[normKey] = append(workflowPatterns[normKey], cleanText)
			}

			// 3. Evals / Knowledge Gaps
			if m.opts.Evals {
				if strings.Contains(turn.AgentReply, "?") && !hasVaultTool(turn.ToolNames) {
					topic := extractTopic(turn.UserPrompt)
					report.Learnings = append(report.Learnings, ExtractedLearning{
						SignalType:      SignalKnowledgeGap,
						Topic:           topic,
						Description:     cleanPrompt(turn.UserPrompt),
						SuggestedAction: "Verificar ausência de documentação no Vault ou criar Harvester",
						ConversationID:  convID,
						Timestamp:       turn.Timestamp,
					})
				}
			}
		}
	}

	// Synthesize Skill Proposals from repetitive workflows
	for normKey, prompts := range workflowPatterns {
		freq := len(prompts)
		if freq >= 2 || m.opts.MineSkills {
			slug := slugify(normKey)
			if slug == "" {
				slug = "custom-workflow"
			}
			skillName := fmt.Sprintf("mined-%s", slug)
			repPrompt := prompts[0]
			if len(repPrompt) > 120 {
				repPrompt = repPrompt[:120] + "..."
			}

			draft := fmt.Sprintf(`---
name: %s
description: %s
---

# 🤖 Proposed Skill: %s

Auto-generated from %d recorded session prompts matching pattern: %s.

## Typical Prompt Trigger
> "%s"

## Recommended Orchestration Steps
1. Parse inputs and context.
2. Execute deterministic hygiene or tooling.
3. Validate artifacts and return concise feedback.
`, skillName, fmt.Sprintf("Automated workflow for %s", normKey), skillName, freq, normKey, repPrompt)

			report.SkillProposals = append(report.SkillProposals, SkillProposal{
				Name:          skillName,
				Title:         fmt.Sprintf("Mined Skill: %s", normKey),
				Description:   fmt.Sprintf("Detected %d times across recent sessions", freq),
				PromptPattern: normKey,
				Frequency:     freq,
				DraftContent:  draft,
			})
		}
	}

	// Sort proposals by frequency descending
	sort.Slice(report.SkillProposals, func(i, j int) bool {
		return report.SkillProposals[i].Frequency > report.SkillProposals[j].Frequency
	})

	// If --apply is requested and not dry-run, apply immediately
	if m.opts.Apply && !m.opts.DryRun {
		_ = m.ApplyReport(ctx, report)
	}

	return report, nil
}

// ApplyReport writes discovered learnings and skill proposals to the vault.
func (m *SessionMiner) ApplyReport(ctx context.Context, report *AuditReport) error {
	if m.opts.VaultRoot == "" {
		return fmt.Errorf("cannot apply learnings: vault root path not configured")
	}

	// 1. Record Learnings into meta/guias/aprendizados-agente.md and JSON cache
	for _, l := range report.Learnings {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		err := dream.RecordFeedback(
			m.opts.VaultRoot,
			l.Topic,
			string(l.SignalType),
			l.SuggestedAction,
			l.Description,
		)
		if err == nil {
			report.AppliedLearnings++
		}
	}

	// 2. Write proposed skills to .agents/skills/propostas/
	proposalsDir := filepath.Join(m.opts.VaultRoot, ".agents", "skills", "propostas")
	for _, skill := range report.SkillProposals {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		targetDir := filepath.Join(proposalsDir, skill.Name)
		_ = os.MkdirAll(targetDir, 0750)
		targetFile := filepath.Join(targetDir, "SKILL.md")
		if err := os.WriteFile(targetFile, []byte(skill.DraftContent), 0600); err == nil {
			report.AppliedSkills++
		}
	}

	return nil
}

func (m *SessionMiner) parseTranscript(filePath string, convID string, cutoff time.Time) ([]SessionTurn, error) {
	file, err := os.Open(filePath) //nolint:gosec
	if err != nil {
		return nil, err
	}
	defer file.Close() //nolint:errcheck

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024) // Support up to 1MB lines

	var turns []SessionTurn
	var pendingUserPrompt string
	var pendingTimestamp time.Time
	var pendingStepIndex int

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var tl transcriptLine
		if err := json.Unmarshal(line, &tl); err != nil {
			continue
		}

		tTime, _ := time.Parse(time.RFC3339, tl.CreatedAt)
		if tTime.IsZero() {
			tTime = time.Now()
		}

		if tTime.Before(cutoff) {
			continue
		}

		if tl.Type == "USER_INPUT" && tl.Source == "USER_EXPLICIT" {
			pendingUserPrompt = extractUserPrompt(tl.Content)
			pendingTimestamp = tTime
			pendingStepIndex = tl.StepIndex
		} else if tl.Type == "PLANNER_RESPONSE" && pendingUserPrompt != "" {
			var tools []string
			for _, tc := range tl.ToolCalls {
				if tc.Name != "" {
					tools = append(tools, tc.Name)
				}
			}

			turns = append(turns, SessionTurn{
				Timestamp:      pendingTimestamp,
				ConversationID: convID,
				StepIndex:      pendingStepIndex,
				UserPrompt:     pendingUserPrompt,
				AgentReply:     tl.Content,
				ToolNames:      tools,
			})
			pendingUserPrompt = ""
		}
	}

	return turns, scanner.Err()
}

func extractUserPrompt(raw string) string {
	if m := userRequestTagRegex.FindStringSubmatch(raw); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	// Fallback to removing XML-like metadata tags
	clean := regexp.MustCompile(`(?s)<[A-Z_]+>.*?</[A-Z_]+>`).ReplaceAllString(raw, "")
	return strings.TrimSpace(clean)
}

func cleanPrompt(p string) string {
	lines := strings.Split(p, "\n")
	var result []string
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return strings.Join(result, " ")
}

func extractTopic(prompt string) string {
	clean := cleanPrompt(prompt)
	if len(clean) > 60 {
		idx := strings.IndexAny(clean[:60], ".!?;:")
		if idx > 10 {
			return clean[:idx]
		}
		return clean[:60] + "..."
	}
	return clean
}

func normalizeWorkflowKey(verbs []string) string {
	lowerVerbs := make([]string, 0, len(verbs))
	seen := make(map[string]bool)
	for _, v := range verbs {
		lv := strings.ToLower(v)
		if !seen[lv] {
			seen[lv] = true
			lowerVerbs = append(lowerVerbs, lv)
		}
	}
	sort.Strings(lowerVerbs)
	return strings.Join(lowerVerbs, "-")
}

func slugify(s string) string {
	reg := regexp.MustCompile(`[^a-zA-Z0-9]+`)
	slug := reg.ReplaceAllString(strings.ToLower(s), "-")
	return strings.Trim(slug, "-")
}

func hasVaultTool(tools []string) bool {
	for _, t := range tools {
		if strings.Contains(t, "search") || strings.Contains(t, "vault") || strings.Contains(t, "grep") {
			return true
		}
	}
	return false
}
