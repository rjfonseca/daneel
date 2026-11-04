// Package daily implements the copilot daily primitives (morning and evening routines).
package daily

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/rjfonseca/daneel/internal/config"
	"github.com/rjfonseca/daneel/internal/dream"
	"github.com/rjfonseca/daneel/internal/ops/backlog"
	"github.com/rjfonseca/daneel/internal/safefile"
)

// MorningReport captures the result of running the morning copilot primitive.
type MorningReport struct {
	Date              string   `json:"date"`
	FilePath          string   `json:"file_path"`
	Created           bool     `json:"created"`
	AutonomousActions []string `json:"autonomous_actions"`
	InboxItems        []string `json:"inbox_items"`
	Alerts            []string `json:"alerts"`
	Suggestions       []string `json:"suggestions"`
	BacklogP0         []string `json:"backlog_p0"`
	BacklogP1         []string `json:"backlog_p1"`
	IngestedMeetings  []string `json:"ingested_meetings,omitempty"`
}

// EveningReport captures the result of running the evening copilot primitive.
type EveningReport struct {
	Date            string             `json:"date"`
	FilePath        string             `json:"file_path"`
	Updated         bool               `json:"updated"`
	HoursByCategory map[string]float64 `json:"hours_by_category"`
	TotalHours      float64            `json:"total_hours"`
}

var (
	catRegex = regexp.MustCompile(`(?i)(?:\[([A-E])\]|\btempo-([A-E])\b)`)
	durRegex = regexp.MustCompile(`(?i)(?:^|[^\w.,])(\d+(?:[.,]\d+)?)\s*h(?:ours?|rs?)?(?:[^\w]|$)`)
)

// RunMorning scaffolds or updates daily/<date>.md with autonomous Dream Cycle handoff data,
// inbox triage items, active P0/P1 priorities, and ingested meetings.
func RunMorning(vaultRoot string, dateStr string, cfg *config.Config) (*MorningReport, error) {
	if dateStr == "" || dateStr == "today" {
		dateStr = time.Now().Format("2006-01-02")
	}

	ownerName := "Partner"
	if cfg != nil && strings.TrimSpace(cfg.Vault.Owner) != "" {
		ownerName = strings.Fields(strings.TrimSpace(cfg.Vault.Owner))[0]
	}

	// 1. Gather Handoff data from .daneel/cache/dream_handoff.json (fallback .daneel/dream_handoff.json)
	var dreamReport dream.FullDreamReport
	handoffPath := filepath.Join(vaultRoot, ".daneel", "cache", "dream_handoff.json")
	data, err := os.ReadFile(handoffPath) //nolint:gosec // intentional
	if err != nil {
		handoffPath = filepath.Join(vaultRoot, ".daneel", "dream_handoff.json")
		data, err = os.ReadFile(handoffPath) //nolint:gosec // intentional
	}
	if err == nil {
		_ = json.Unmarshal(data, &dreamReport)
	}

	// Autonomous actions
	var autonomousActions []string
	if dreamReport.FixedLinksCount > 0 {
		autonomousActions = append(autonomousActions, fmt.Sprintf("%d WikiLinks healed into relative Markdown links", dreamReport.FixedLinksCount))
	}
	if len(dreamReport.MissingMOCs) > 0 {
		autonomousActions = append(autonomousActions, fmt.Sprintf("%d notes synchronized into thematic MOCs", len(dreamReport.MissingMOCs)))
	}
	if len(dreamReport.SyncedPeople) > 0 {
		autonomousActions = append(autonomousActions, fmt.Sprintf("%d colleague timelines synchronized", len(dreamReport.SyncedPeople)))
	}
	if dreamReport.IngestedMeetingsCount > 0 {
		autonomousActions = append(autonomousActions, fmt.Sprintf("%d meeting notes ingested into sources/reunioes/", dreamReport.IngestedMeetingsCount))
	}

	// Alerts
	var alerts []string
	if dreamReport.BrokenLinksCount > 0 {
		alerts = append(alerts, fmt.Sprintf("%d broken links pending review", dreamReport.BrokenLinksCount))
	}
	if dreamReport.TemporalAnalysis != nil && len(dreamReport.TemporalAnalysis.DriftAlerts) > 0 {
		alerts = append(alerts, dreamReport.TemporalAnalysis.DriftAlerts...)
	}

	// Suggestions
	var suggestions []string
	if dreamReport.Metacognition != nil {
		for _, in := range dreamReport.Metacognition.Insights {
			suggestions = append(suggestions, fmt.Sprintf("%s -> %s", in.Observation, in.SuggestedAction))
		}
	}
	for _, q := range dreamReport.HandoffQuestions {
		suggestions = append(suggestions, q.Question)
	}

	// Inbox items
	var inboxItems []string
	inboxDir := filepath.Join(vaultRoot, "inbox")
	if entries, err := os.ReadDir(inboxDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
				inboxItems = append(inboxItems, e.Name())
			}
		}
		sort.Strings(inboxItems)
	}

	// Backlog items
	var backlogP0 []string
	var backlogP1 []string
	if blReport, err := backlog.GetActiveBacklog(vaultRoot); err == nil && blReport != nil {
		for _, item := range blReport.Items {
			if !item.Completed {
				if strings.EqualFold(item.Priority, "P0") {
					backlogP0 = append(backlogP0, item.Raw)
				} else if strings.EqualFold(item.Priority, "P1") {
					backlogP1 = append(backlogP1, item.Raw)
				}
			}
		}
	}
	if len(backlogP0) == 0 && len(backlogP1) == 0 {
		backlogP0 = append(backlogP0, dreamReport.BacklogP0...)
		backlogP1 = append(backlogP1, dreamReport.BacklogP1...)
	}

	// Ingested meetings
	var ingestedMeetings []string
	reunioesDir := filepath.Join(vaultRoot, "sources", "reunioes")
	if entries, err := os.ReadDir(reunioesDir); err == nil {
		prefix := dateStr + "-"
		for _, e := range entries {
			if !e.IsDir() && strings.HasPrefix(e.Name(), prefix) && strings.HasSuffix(e.Name(), ".md") {
				ingestedMeetings = append(ingestedMeetings, filepath.ToSlash(filepath.Join("sources", "reunioes", e.Name())))
			}
		}
		sort.Strings(ingestedMeetings)
	}

	// 2. Build Handoff Block
	handoffBlock := buildHandoffBlock(ownerName, autonomousActions, inboxItems, alerts, suggestions)

	relPath := filepath.ToSlash(filepath.Join("daily", dateStr+".md"))
	absDailyPath := filepath.Join(vaultRoot, "daily", dateStr+".md")

	rep := &MorningReport{
		Date:              dateStr,
		FilePath:          relPath,
		AutonomousActions: autonomousActions,
		InboxItems:        inboxItems,
		Alerts:            alerts,
		Suggestions:       suggestions,
		BacklogP0:         backlogP0,
		BacklogP1:         backlogP1,
		IngestedMeetings:  ingestedMeetings,
	}

	// Check if daily note exists
	existingBytes, err := os.ReadFile(absDailyPath) //nolint:gosec // intentional
	if os.IsNotExist(err) {
		// New file
		rep.Created = true
		content := buildNewDailyContent(dateStr, handoffBlock, backlogP0, backlogP1, ingestedMeetings)
		if err := safefile.WriteFile(vaultRoot, absDailyPath, []byte(content), 0600); err != nil {
			return nil, fmt.Errorf("failed to write daily note: %w", err)
		}
		return rep, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to read daily note: %w", err)
	}

	// File exists
	rep.Created = false
	existingContent := string(existingBytes)
	if !strings.Contains(existingContent, "## 🤝 Handoff do Copiloto") {
		updatedContent := insertHandoffBlock(existingContent, handoffBlock)
		if err := safefile.WriteFile(vaultRoot, absDailyPath, []byte(updatedContent), 0600); err != nil {
			return nil, fmt.Errorf("failed to inject handoff block: %w", err)
		}
	}

	return rep, nil
}

// RunEvening reconciles explicit time annotations from the daily note body into YAML frontmatter.
func RunEvening(vaultRoot string, dateStr string, cfg *config.Config) (*EveningReport, error) { //nolint:revive
	if dateStr == "" || dateStr == "today" {
		dateStr = time.Now().Format("2006-01-02")
	}

	relPath := filepath.ToSlash(filepath.Join("daily", dateStr+".md"))
	absDailyPath := filepath.Join(vaultRoot, "daily", dateStr+".md")

	data, err := os.ReadFile(absDailyPath) //nolint:gosec // intentional
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("daily log not found for %s", dateStr)
		}
		return nil, fmt.Errorf("failed to read daily log: %w", err)
	}

	content := string(data)
	var fmContent string
	var bodyContent string
	hasFM := false

	if strings.HasPrefix(content, "---") {
		rest := content[3:]
		if idx := strings.Index(rest, "\n---"); idx != -1 {
			fmContent = rest[:idx]
			bodyContent = strings.TrimPrefix(rest[idx+4:], "\n")
			hasFM = true
		} else {
			bodyContent = content
		}
	} else {
		bodyContent = content
	}

	hoursByCat := map[string]float64{
		"A": 0,
		"B": 0,
		"C": 0,
		"D": 0,
		"E": 0,
	}
	var totalHours float64

	lines := strings.Split(bodyContent, "\n")
	for _, line := range lines {
		catMatch := catRegex.FindStringSubmatch(line)
		if len(catMatch) == 0 {
			continue
		}
		cat := catMatch[1]
		if cat == "" {
			cat = catMatch[2]
		}
		cat = strings.ToUpper(cat)

		durMatch := durRegex.FindStringSubmatch(line)
		if len(durMatch) < 2 {
			continue
		}
		durStr := strings.ReplaceAll(durMatch[1], ",", ".")
		h, err := strconv.ParseFloat(durStr, 64)
		if err != nil || h <= 0 {
			continue
		}
		hoursByCat[cat] += h
		totalHours += h
	}

	rep := &EveningReport{
		Date:            dateStr,
		FilePath:        relPath,
		HoursByCategory: hoursByCat,
		TotalHours:      totalHours,
	}

	if totalHours > 0 {
		rep.Updated = true
		newContent := updateFrontmatterTempos(content, hoursByCat)
		if err := safefile.WriteFile(vaultRoot, absDailyPath, []byte(newContent), 0600); err != nil {
			return nil, fmt.Errorf("failed to update daily log frontmatter: %w", err)
		}
	} else {
		rep.Updated = false
		if hasFM {
			var rawFM map[string]interface{}
			if err := yaml.Unmarshal([]byte(fmContent), &rawFM); err == nil {
				readCat := func(fullKey, shortKey string) float64 {
					if v, ok := rawFM[fullKey]; ok {
						return toFloat(v)
					}
					if v, ok := rawFM[shortKey]; ok {
						return toFloat(v)
					}
					return 0
				}
				rep.HoursByCategory["A"] = readCat("tempo-A_Estrategico", "tempo-A")
				rep.HoursByCategory["B"] = readCat("tempo-B_ImpactoLocal", "tempo-B")
				rep.HoursByCategory["C"] = readCat("tempo-C_GlueWork", "tempo-C")
				rep.HoursByCategory["D"] = readCat("tempo-D_LowImpact", "tempo-D")
				rep.HoursByCategory["E"] = readCat("tempo-E_Overhead", "tempo-E")
				rep.TotalHours = rep.HoursByCategory["A"] + rep.HoursByCategory["B"] + rep.HoursByCategory["C"] + rep.HoursByCategory["D"] + rep.HoursByCategory["E"]
			}
		}
	}

	return rep, nil
}

func buildHandoffBlock(ownerName string, autonomousActions, inboxItems, alerts, suggestions []string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "## 🤝 Handoff do Copiloto (Daneel ➔ %s)\n", ownerName)

	sb.WriteString("### 🤖 Decisões Autônomas & Auto-Curas Realizadas\n")
	if len(autonomousActions) > 0 {
		for _, act := range autonomousActions {
			fmt.Fprintf(&sb, "- %s\n", act)
		}
	} else {
		sb.WriteString("- Nenhuma ação corretiva necessária no último ciclo.\n")
	}
	sb.WriteString("\n")

	sb.WriteString("### 📥 Itens do Inbox que Requerem Decisão Humana\n")
	if len(inboxItems) > 0 {
		for _, item := range inboxItems {
			fmt.Fprintf(&sb, "- [ ] `%s`\n", item)
		}
	} else {
		sb.WriteString("- Nenhum item pendente no inbox.\n")
	}
	sb.WriteString("\n")

	sb.WriteString("### ⚠️ Alertas do Positronic Core\n")
	if len(alerts) > 0 {
		for _, alert := range alerts {
			fmt.Fprintf(&sb, "- %s\n", alert)
		}
	} else {
		sb.WriteString("- Estabilidade positrônica nominal (zero alertas).\n")
	}
	sb.WriteString("\n")

	sb.WriteString("### 💡 Sugestões da Metacognição\n")
	if len(suggestions) > 0 {
		for _, sug := range suggestions {
			fmt.Fprintf(&sb, "- %s\n", sug)
		}
	} else {
		sb.WriteString("- Nenhuma sugestão pendente.\n")
	}

	return strings.TrimRight(sb.String(), "\n")
}

func buildNewDailyContent(dateStr, handoffBlock string, p0, p1, meetings []string) string {
	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString("type: daily\n")
	fmt.Fprintf(&sb, "date: %s\n", dateStr)
	sb.WriteString("tempo-A_Estrategico: 0\n")
	sb.WriteString("tempo-B_ImpactoLocal: 0\n")
	sb.WriteString("tempo-C_GlueWork: 0\n")
	sb.WriteString("tempo-D_LowImpact: 0\n")
	sb.WriteString("tempo-E_Overhead: 0\n")
	sb.WriteString("tags: [daily]\n")
	sb.WriteString("---\n\n")

	fmt.Fprintf(&sb, "# 📅 Diário de Bordo — %s\n\n", dateStr)
	sb.WriteString(handoffBlock)
	sb.WriteString("\n\n")

	sb.WriteString("## 🎯 Foco & Prioridades do Dia\n")
	for _, item := range p0 {
		fmt.Fprintf(&sb, "- [ ] %s\n", item)
	}
	for _, item := range p1 {
		fmt.Fprintf(&sb, "- [ ] %s\n", item)
	}
	sb.WriteString("\n")

	sb.WriteString("## 🗓️ Reuniões & Alinhamentos\n")
	for _, m := range meetings {
		fmt.Fprintf(&sb, "- [ ] [%s](../%s)\n", m, m)
	}
	sb.WriteString("\n")

	sb.WriteString("## ⏱️ Notas de Tempo & Log de Atividades\n\n")
	sb.WriteString("## 🧠 Conhecimento & Descobertas\n")

	return sb.String()
}

func insertHandoffBlock(content, handoffBlock string) string {
	lines := strings.Split(content, "\n")
	h1Idx := -1
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "# ") {
			h1Idx = i
			break
		}
	}

	if h1Idx != -1 {
		result := make([]string, 0, len(lines)+3)
		result = append(result, lines[:h1Idx+1]...)
		result = append(result, "", handoffBlock, "")
		result = append(result, lines[h1Idx+1:]...)
		return strings.Join(result, "\n")
	}

	if strings.HasPrefix(content, "---") {
		rest := content[3:]
		if idx := strings.Index(rest, "\n---"); idx != -1 {
			endFM := 3 + idx + 4
			return content[:endFM] + "\n\n" + handoffBlock + "\n" + content[endFM:]
		}
	}

	return handoffBlock + "\n\n" + content
}

func updateFrontmatterTempos(content string, hours map[string]float64) string {
	keys := map[string]string{
		"A": "tempo-A_Estrategico",
		"B": "tempo-B_ImpactoLocal",
		"C": "tempo-C_GlueWork",
		"D": "tempo-D_LowImpact",
		"E": "tempo-E_Overhead",
	}
	order := []string{"A", "B", "C", "D", "E"}
	updated := map[string]bool{}

	if strings.HasPrefix(content, "---") {
		rest := content[3:]
		idx := strings.Index(rest, "\n---")
		if idx != -1 {
			fmText := rest[:idx]
			closingAndBody := rest[idx:]

			lines := strings.Split(fmText, "\n")
			for i, line := range lines {
				trimmed := strings.TrimSpace(line)
				for cat, key := range keys {
					re := regexp.MustCompile(fmt.Sprintf(`^(%s|tempo-%s)\s*:.*$`, regexp.QuoteMeta(key), cat))
					if re.MatchString(trimmed) {
						lines[i] = fmt.Sprintf("%s: %s", key, formatHourValue(hours[cat]))
						updated[cat] = true
						break
					}
				}
			}

			for _, cat := range order {
				if !updated[cat] {
					lines = append(lines, fmt.Sprintf("%s: %s", keys[cat], formatHourValue(hours[cat])))
				}
			}

			return "---" + strings.Join(lines, "\n") + closingAndBody
		}
	}

	var sb strings.Builder
	sb.WriteString("---\n")
	for _, cat := range order {
		fmt.Fprintf(&sb, "%s: %s\n", keys[cat], formatHourValue(hours[cat]))
	}
	sb.WriteString("---\n\n")
	sb.WriteString(content)
	return sb.String()
}

func formatHourValue(h float64) string {
	return strconv.FormatFloat(h, 'f', -1, 64)
}

func toFloat(val interface{}) float64 {
	switch v := val.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case string:
		f, _ := strconv.ParseFloat(v, 64)
		return f
	}
	return 0
}
