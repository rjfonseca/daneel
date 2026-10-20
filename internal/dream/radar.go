//nolint:revive // package comment not required
package dream

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/rjfonseca/daneel/internal/safefile"

	"gopkg.in/yaml.v3"

	"github.com/rjfonseca/daneel/internal/config"
)

// TemporalWindowData holds aggregated sliding-window Staff telemetry.
type TemporalWindowData struct {
	WindowDays      int                `json:"window_days"`
	DaysWithLogs    int                `json:"days_with_logs"`
	TotalHours      float64            `json:"total_hours"`
	HoursByCat      map[string]float64 `json:"hours_by_cat"`
	Percentages     map[string]float64 `json:"percentages"`
	DriftAlerts     []string           `json:"drift_alerts"`
	TopPeople       []string           `json:"top_people"`
	TopSystems      []string           `json:"top_systems"`
	FrictionSignals []FrictionSignal   `json:"friction_signals"`
}

// FrictionSignal reports a technical debt or recurring operational blocker.
type FrictionSignal struct {
	Date string `json:"date"`
	Line string `json:"line"`
}

// StrategicRadar reports initiative traction and high-value architectural insights.
type StrategicRadar struct {
	TractionSummary   []InitiativeTraction `json:"traction_summary"`
	StrategicInsights []string             `json:"strategic_insights"`
}

// InitiativeTraction reports mention counts and traction status for a business theme.
type InitiativeTraction struct {
	Initiative string `json:"initiative"`
	Count      int    `json:"count"`
	Status     string `json:"status"`
}

var frictionKeywords = []string{
	"incidente", "bug", "falha", "bloqueio", "lento", "lentidao",
	"reteste", "gargalo", "defeito", "cve", "vulnerabilidade", "suporte",
}

func AnalyzeTemporalWindow(vaultRoot string, targetDateStr string, days int) (*TemporalWindowData, error) { //nolint:revive
	curDate, err := time.Parse("2006-01-02", targetDateStr)
	if err != nil {
		curDate = time.Now()
	}

	dailyDir := filepath.Join(vaultRoot, "daily")
	totalHours := 0.0
	hoursByCat := make(map[string]float64)
	daysWithLogs := 0

	peopleCounter := make(map[string]int)
	systemsCounter := make(map[string]int)
	var frictionSignals []FrictionSignal

	fKeywords := frictionKeywords
	cfg, err := config.LoadConfig(vaultRoot, "", false)
	var archetypes []config.ArchetypeConfig
	if err == nil {
		if kws := cfg.GetFrictionKeywords(); len(kws) > 0 {
			fKeywords = kws
		}
		archetypes = cfg.GetTaxonomyArchetypes()
	}

	for _, arch := range archetypes {
		hoursByCat[arch.ID] = 0
	}

	for i := 0; i < days; i++ {
		dStr := curDate.AddDate(0, 0, -i).Format("2006-01-02")
		dFile := filepath.Join(dailyDir, dStr+".md")

		data, err := os.ReadFile(dFile) //nolint:gosec // intentional
		if err != nil {
			continue
		}
		content := string(data)
		daysWithLogs++

		// Extract Frontmatter Telemetry
		fmRegex := regexp.MustCompile(`(?s)^---\s*\n(.*?)\n---\s*\n`)
		if m := fmRegex.FindStringSubmatch(content); len(m) > 1 {
			var rawFM map[string]interface{}
			if err := yaml.Unmarshal([]byte(m[1]), &rawFM); err == nil {
				for _, arch := range archetypes {
					key := fmt.Sprintf("tempo-%s", arch.ID)
					t := toFloat(rawFM[key])
					if t == 0 {
						t = toFloat(rawFM[arch.ID])
					}
					hoursByCat[arch.ID] += t
					totalHours += t
				}
			}
		}

		// Count People mentions
		pMatches := regexp.MustCompile(`(?:notes|meta)/pessoas/([^)#]+)\.md`).FindAllStringSubmatch(content, -1)
		for _, pm := range pMatches {
			if len(pm) > 1 {
				peopleCounter[pm[1]]++
			}
		}

		// Count Systems mentions
		sMatches := regexp.MustCompile(`notes/sistemas/([^)#]+)\.md`).FindAllStringSubmatch(content, -1)
		for _, sm := range sMatches {
			if len(sm) > 1 {
				systemsCounter[sm[1]]++
			}
		}

		// Detect Friction Signals
		for _, line := range strings.Split(content, "\n") {
			lLower := strings.ToLower(line)
			for _, kw := range fKeywords {
				if strings.Contains(lLower, kw) && len(strings.TrimSpace(line)) > 15 {
					clean := regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`).ReplaceAllString(line, "$1")
					clean = strings.Trim(clean, "- *\r\n")
					if len(clean) > 120 {
						clean = clean[:120] + "..."
					}
					frictionSignals = append(frictionSignals, FrictionSignal{
						Date: dStr,
						Line: clean,
					})
					break
				}
			}
		}
	}

	percentages := make(map[string]float64)
	if totalHours > 0 {
		for k, v := range hoursByCat {
			percentages[k] = roundFloat((v/totalHours)*100, 1)
		}
	}

	// Dynamic Drift Diagnosis
	var driftAlerts []string
	if totalHours >= 15.0 {
		for _, arch := range archetypes {
			pct := percentages[arch.ID]
			if arch.TargetMin > 0 && pct < arch.TargetMin {
				driftAlerts = append(driftAlerts, fmt.Sprintf("Time in `%s` (%.1f%%) is below the configured minimum target of %.1f%%.", arch.Name, pct, arch.TargetMin))
			}
			if arch.TargetMax > 0 && pct > arch.TargetMax {
				driftAlerts = append(driftAlerts, fmt.Sprintf("Time in `%s` (%.1f%%) exceeds the configured maximum limit of %.1f%%.", arch.Name, pct, arch.TargetMax))
			}
		}
	}

	topPeople := getTopKeys(peopleCounter, 5)
	topSystems := getTopKeys(systemsCounter, 5)

	limitFriction := 4
	if len(frictionSignals) < limitFriction {
		limitFriction = len(frictionSignals)
	}

	return &TemporalWindowData{
		WindowDays:      days,
		DaysWithLogs:    daysWithLogs,
		TotalHours:      roundFloat(totalHours, 1),
		HoursByCat:      hoursByCat,
		Percentages:     percentages,
		DriftAlerts:     driftAlerts,
		TopPeople:       topPeople,
		TopSystems:      topSystems,
		FrictionSignals: frictionSignals[:limitFriction],
	}, nil
}

// CrossSynthesizeTechBusiness builds the strategic radar between technical work and business goals.
func CrossSynthesizeTechBusiness(vaultRoot string, targetDateStr string, temporalData *TemporalWindowData, emergingConcepts []EmergingConcept, learnings *AgentLearnings) *StrategicRadar {
	curDate, err := time.Parse("2006-01-02", targetDateStr)
	if err != nil {
		curDate = time.Now()
	}

	dailyDir := filepath.Join(vaultRoot, "daily")
	var aggregatedText strings.Builder

	for i := 0; i < temporalData.WindowDays; i++ {
		dStr := curDate.AddDate(0, 0, -i).Format("2006-01-02")
		dFile := filepath.Join(dailyDir, dStr+".md")
		if data, err := os.ReadFile(dFile); err == nil { //nolint:gosec // intentional
			aggregatedText.WriteString(" ")
			aggregatedText.WriteString(strings.ToLower(string(data)))
		}
	}

	allText := aggregatedText.String()
	var tractionList []InitiativeTraction //nolint:prealloc

	for initName, keywords := range learnings.BusinessInitiativesKeywords {
		count := 0
		for _, kw := range keywords {
			count += strings.Count(allText, strings.ToLower(kw))
		}

		status := "❄️ Cold (No recent mentions)"
		if count >= 5 {
			status = "🔥 High Traction"
		} else if count > 0 {
			status = "🟢 Active"
		}

		tractionList = append(tractionList, InitiativeTraction{
			Initiative: initName,
			Count:      count,
			Status:     status,
		})
	}

	sort.Slice(tractionList, func(i, j int) bool {
		return tractionList[i].Count > tractionList[j].Count
	})

	cfg, _ := config.LoadConfig(vaultRoot, "", false)
	radarItems := cfg.GetRadarItems("")

	var insights []string
	for _, ec := range emergingConcepts {
		tLower := strings.ToLower(ec.Title)
		matched := false
		for _, item := range radarItems {
			for _, kw := range item.Keywords {
				if strings.Contains(tLower, strings.ToLower(kw)) {
					if item.Ring != "" {
						insights = append(insights, fmt.Sprintf("Concept `%s` aligns with configured %s radar [%s: %s]: *%s*.", ec.Title, item.Type, item.Quadrant, item.Ring, item.Name))
					} else {
						insights = append(insights, fmt.Sprintf("Concept `%s` aligns with configured radar [%s]: *%s*.", ec.Title, item.Type, item.Name))
					}
					matched = true
					break
				}
			}
			if matched {
				break
			}
		}
	}

	return &StrategicRadar{
		TractionSummary:   tractionList,
		StrategicInsights: insights,
	}
}

// SyncPeopleInteractionsFromDaily syncs interactions recorded in daily note to meta/pessoas/*.md.
func SyncPeopleInteractionsFromDaily(vaultRoot string, targetDate string) ([]string, error) {
	dailyFile := filepath.Join(vaultRoot, "daily", targetDate+".md")
	data, err := os.ReadFile(dailyFile) //nolint:gosec // intentional
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	content := string(data)
	personLinks := regexp.MustCompile(`\[([^\]]+)\]\(([^)]*(?:notes|meta)/pessoas/([^)#]+)\.md[^)]*)\)`).FindAllStringSubmatch(content, -1)
	var synced []string

	logLines := strings.Split(content, "\n")

	for _, m := range personLinks {
		if len(m) < 4 {
			continue
		}
		label := m[1]
		slug := m[3]

		personFile := filepath.Join(vaultRoot, "notes", "pessoas", slug+".md")
		if _, err := os.Stat(personFile); os.IsNotExist(err) { //nolint:gosec // intentional
			personFile = filepath.Join(vaultRoot, "meta", "pessoas", slug+".md")
		}
		pData, err := os.ReadFile(personFile) //nolint:gosec // intentional
		if err != nil {
			continue
		}

		pContent := string(pData)
		if strings.Contains(pContent, fmt.Sprintf("- **%s**", targetDate)) || strings.Contains(pContent, fmt.Sprintf("**%s**", targetDate)) {
			continue
		}

		contextSummary := "Interação e alinhamento registrados no diário de bordo."
		for _, l := range logLines {
			if strings.Contains(l, slug) || strings.Contains(l, label) {
				clean := regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`).ReplaceAllString(l, "$1")
				clean = strings.Trim(clean, "- *\r\n")
				if len(clean) > 15 {
					if len(clean) > 120 {
						clean = clean[:120] + "..."
					}
					contextSummary = clean
					break
				}
			}
		}

		entry := fmt.Sprintf("- **%s** - [Diário de Bordo](../../daily/%s.md): %s\n", targetDate, targetDate, contextSummary)
		if strings.Contains(pContent, "## 🤝 Histórico de Interações") {
			pContent = strings.Replace(pContent, "## 🤝 Histórico de Interações\n", "## 🤝 Histórico de Interações\n"+entry, 1)
			if err := safefile.WriteFile(vaultRoot, personFile, []byte(pContent), 0600); err != nil {
				return synced, fmt.Errorf("save person interactions: %w", err)
			}
			synced = append(synced, fmt.Sprintf("%s (%s)", label, slug))
		}
	}

	return synced, nil
}

func getTopKeys(m map[string]int, limit int) []string {
	type pair struct {
		k string
		v int
	}
	var pairs []pair //nolint:prealloc
	for k, v := range m {
		pairs = append(pairs, pair{k, v})
	}
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].v > pairs[j].v
	})

	var result []string
	for i, p := range pairs {
		if i >= limit {
			break
		}
		result = append(result, fmt.Sprintf("%s (%d)", p.k, p.v))
	}
	return result
}
