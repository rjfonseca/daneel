//nolint:revive // package comment not required
package tracking

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// DailyLog represents parsed activities and time telemetry from daily notes.
type DailyLog struct {
	Date        string   `json:"date"`
	TempoA      float64  `json:"tempo_a_estrategico"`
	TempoB      float64  `json:"tempo_b_impacto_local"`
	TempoC      float64  `json:"tempo_c_glue_work"`
	TempoD      float64  `json:"tempo_d_low_impact"`
	TempoE      float64  `json:"tempo_e_overhead"`
	TotalHours  float64  `json:"total_hours"`
	Activities  []string `json:"activities"`
	Alignments  []string `json:"alignments"`
	Discoveries []string `json:"discoveries"`
}

// GetDailyLog retrieves and parses a single daily note (YYYY-MM-DD).
func GetDailyLog(vaultRoot string, dateStr string) (*DailyLog, error) {
	if dateStr == "" || dateStr == "today" {
		dateStr = time.Now().Format("2006-01-02")
	}

	dailyPath := filepath.Join(vaultRoot, "daily", dateStr+".md")
	data, err := os.ReadFile(dailyPath) //nolint:gosec // intentional
	if err != nil {
		return nil, err
	}

	content := string(data)
	log := &DailyLog{
		Date: dateStr,
	}

	// Parse frontmatter
	fmRegex := regexp.MustCompile(`(?s)^---\s*\n(.*?)\n---\s*\n`)
	if m := fmRegex.FindStringSubmatch(content); len(m) > 1 {
		var rawFM map[string]interface{}
		if err := yaml.Unmarshal([]byte(m[1]), &rawFM); err == nil {
			log.TempoA = toFloat(rawFM["tempo-A_Estrategico"])
			log.TempoB = toFloat(rawFM["tempo-B_ImpactoLocal"])
			log.TempoC = toFloat(rawFM["tempo-C_GlueWork"])
			log.TempoD = toFloat(rawFM["tempo-D_LowImpact"])
			log.TempoE = toFloat(rawFM["tempo-E_Overhead"])
			log.TotalHours = log.TempoA + log.TempoB + log.TempoC + log.TempoD + log.TempoE
		}
	}

	// Line by line parser for sections
	lines := strings.Split(content, "\n")
	currentSection := ""

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.Contains(trimmed, "Log de Atividades") || strings.Contains(trimmed, "Activity Log") || strings.Contains(trimmed, "Activities") {
			currentSection = "activities"
			continue
		}
		if strings.Contains(trimmed, "Alinhamentos e Interações") || strings.Contains(trimmed, "Alignments & Interactions") || strings.Contains(trimmed, "Alignments") {
			currentSection = "alignments"
			continue
		}
		if strings.Contains(trimmed, "Conhecimento & Descobertas") || strings.Contains(trimmed, "Knowledge & Discoveries") || strings.Contains(trimmed, "Discoveries") {
			currentSection = "discoveries"
			continue
		}
		if strings.HasPrefix(trimmed, "## ") && !strings.Contains(trimmed, "Atividades") && !strings.Contains(trimmed, "Activities") && !strings.Contains(trimmed, "Alinhamentos") && !strings.Contains(trimmed, "Alignments") && !strings.Contains(trimmed, "Conhecimento") && !strings.Contains(trimmed, "Knowledge") {
			currentSection = ""
			continue
		}

		if strings.HasPrefix(trimmed, "-") && len(trimmed) > 2 {
			clean := strings.TrimLeft(trimmed, "- *")
			clean = strings.TrimSpace(clean)

			switch currentSection {
			case "activities":
				if len(clean) > 0 {
					log.Activities = append(log.Activities, clean)
				}
			case "alignments":
				if len(clean) > 0 && !strings.Contains(clean, "Links para perfis") {
					log.Alignments = append(log.Alignments, clean)
				}
			case "discoveries":
				if len(clean) > 0 && !strings.Contains(clean, "Links para domínios") {
					log.Discoveries = append(log.Discoveries, clean)
				}
			}
		}
	}

	return log, nil
}

// ListDailyLogs returns the most recent daily logs.
func ListDailyLogs(vaultRoot string, limit int) ([]DailyLog, error) {
	dailyDir := filepath.Join(vaultRoot, "daily")
	if _, err := os.Stat(dailyDir); os.IsNotExist(err) {
		return nil, nil
	}

	entries, err := os.ReadDir(dailyDir)
	if err != nil {
		return nil, err
	}

	var dates []string
	datePattern := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}\.md$`)
	for _, e := range entries {
		if !e.IsDir() && datePattern.MatchString(e.Name()) {
			dates = append(dates, strings.TrimSuffix(e.Name(), ".md"))
		}
	}

	sort.Sort(sort.Reverse(sort.StringSlice(dates)))
	if limit > 0 && len(dates) > limit {
		dates = dates[:limit]
	}

	var logs []DailyLog
	for _, d := range dates {
		l, err := GetDailyLog(vaultRoot, d)
		if err == nil && l != nil {
			logs = append(logs, *l)
		}
	}

	return logs, nil
}

// PeriodSummary consolidates aggregated time telemetry for a temporal block (ISO week or month).
type PeriodSummary struct {
	PeriodKey    string  `json:"period_key"`    // e.g. "2026-W39" or "2026-09"
	Label        string  `json:"label"`         // e.g. "Week 39 (Sep 21 - Sep 27)" or "September 2026"
	StartDate    string  `json:"start_date"`    // YYYY-MM-DD
	EndDate      string  `json:"end_date"`      // YYYY-MM-DD
	DaysCount    int     `json:"days_count"`    // Days with logs in period
	TempoA       float64 `json:"tempo_a"`       // Strategic
	TempoB       float64 `json:"tempo_b"`       // Local Impact
	TempoC       float64 `json:"tempo_c"`       // Glue Work
	TempoD       float64 `json:"tempo_d"`       // Low Impact / Bureaucracy
	TempoE       float64 `json:"tempo_e"`       // Overhead
	TotalHours   float64 `json:"total_hours"`   // Sum A+B+C+D+E
	LeverageRate float64 `json:"leverage_rate"` // (A + C) / Total (0.0 to 1.0)
	Diagnosis    string  `json:"diagnosis"`     // Qualitative classification
}

// CalculateLeverageRate computes technical staff leverage: (TempoA + TempoC) / Total.
func CalculateLeverageRate(tempoA, tempoC, total float64) float64 {
	if total <= 0 {
		return 0
	}
	rate := (tempoA + tempoC) / total
	if rate > 1.0 {
		return 1.0
	}
	return rate
}

// ClassifyPeriodDiagnosis evaluates qualitative focus based on time distribution.
func ClassifyPeriodDiagnosis(p PeriodSummary) string {
	if p.TotalHours == 0 {
		return "💤 No Activity Logged"
	}
	stratRatio := p.TempoA / p.TotalHours
	localRatio := p.TempoB / p.TotalHours
	glueRatio := p.TempoC / p.TotalHours
	overheadRatio := p.TempoE / p.TotalHours

	// 1. Healthy Leverage: LeverageRate >= 50% and Strategic A >= 35%
	if p.LeverageRate >= 0.50 && stratRatio >= 0.35 {
		return "🚀 Strategic Multiplication"
	}
	// 2. Firefighting / High Local Reactivity: Local Impact B >= 45%
	if localRatio >= 0.45 {
		return "🔥 Local Reactivity & Firefighting"
	}
	// 3. High Alignment / Glue Work Overload: Glue C >= 35% or Overhead E >= 25%
	if glueRatio >= 0.35 || overheadRatio >= 0.25 {
		return "🤝 High Glue & Alignment Load"
	}
	// 4. Balanced Operational
	return "⚖️ Balanced Operational Focus"
}

// AggregateByWeek groups daily logs by ISO calendar weeks and returns up to numWeeks summaries.
// Returns periods in chronological order (oldest to newest).
func AggregateByWeek(vaultRoot string, numWeeks int) ([]PeriodSummary, error) {
	if numWeeks <= 0 {
		numWeeks = 4
	}

	logs, err := ListDailyLogs(vaultRoot, 0)
	if err != nil {
		return nil, err
	}

	type weekBucket struct {
		year      int
		week      int
		logs      []DailyLog
		startDate time.Time
		endDate   time.Time
	}

	buckets := make(map[string]*weekBucket)
	var keys []string

	for _, l := range logs {
		t, err := time.Parse("2006-01-02", l.Date)
		if err != nil {
			continue
		}
		year, week := t.ISOWeek()
		key := fmt.Sprintf("%04d-W%02d", year, week)

		b, exists := buckets[key]
		if !exists {
			start, end := isoWeekBounds(year, week)
			b = &weekBucket{
				year:      year,
				week:      week,
				startDate: start,
				endDate:   end,
			}
			buckets[key] = b
			keys = append(keys, key)
		}
		b.logs = append(b.logs, l)
	}

	// Sort chronologically ascending
	sort.Strings(keys)

	if len(keys) > numWeeks {
		keys = keys[len(keys)-numWeeks:]
	}

	summaries := make([]PeriodSummary, 0, len(keys))
	for _, k := range keys {
		b := buckets[k]
		summary := PeriodSummary{
			PeriodKey: k,
			Label: fmt.Sprintf("Week %02d (%s - %s)",
				b.week,
				b.startDate.Format("Jan 02"),
				b.endDate.Format("Jan 02"),
			),
			StartDate: b.startDate.Format("2006-01-02"),
			EndDate:   b.endDate.Format("2006-01-02"),
			DaysCount: len(b.logs),
		}

		for _, l := range b.logs {
			summary.TempoA += l.TempoA
			summary.TempoB += l.TempoB
			summary.TempoC += l.TempoC
			summary.TempoD += l.TempoD
			summary.TempoE += l.TempoE
		}
		summary.TotalHours = summary.TempoA + summary.TempoB + summary.TempoC + summary.TempoD + summary.TempoE
		summary.LeverageRate = CalculateLeverageRate(summary.TempoA, summary.TempoC, summary.TotalHours)
		summary.Diagnosis = ClassifyPeriodDiagnosis(summary)

		summaries = append(summaries, summary)
	}

	return summaries, nil
}

// AggregateByMonth groups daily logs by calendar months and returns up to numMonths summaries.
// Returns periods in chronological order (oldest to newest).
func AggregateByMonth(vaultRoot string, numMonths int) ([]PeriodSummary, error) {
	if numMonths <= 0 {
		numMonths = 6
	}

	logs, err := ListDailyLogs(vaultRoot, 0)
	if err != nil {
		return nil, err
	}

	type monthBucket struct {
		yearMonth time.Time
		logs      []DailyLog
		startDate time.Time
		endDate   time.Time
	}

	buckets := make(map[string]*monthBucket)
	var keys []string

	for _, l := range logs {
		t, err := time.Parse("2006-01-02", l.Date)
		if err != nil {
			continue
		}
		key := t.Format("2006-01")

		b, exists := buckets[key]
		if !exists {
			firstDay := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
			lastDay := firstDay.AddDate(0, 1, -1)
			b = &monthBucket{
				yearMonth: firstDay,
				startDate: firstDay,
				endDate:   lastDay,
			}
			buckets[key] = b
			keys = append(keys, key)
		}
		b.logs = append(b.logs, l)
	}

	// Sort chronologically ascending
	sort.Strings(keys)

	if len(keys) > numMonths {
		keys = keys[len(keys)-numMonths:]
	}

	summaries := make([]PeriodSummary, 0, len(keys))
	for _, k := range keys {
		b := buckets[k]
		summary := PeriodSummary{
			PeriodKey: k,
			Label:     b.yearMonth.Format("January 2006"),
			StartDate: b.startDate.Format("2006-01-02"),
			EndDate:   b.endDate.Format("2006-01-02"),
			DaysCount: len(b.logs),
		}

		for _, l := range b.logs {
			summary.TempoA += l.TempoA
			summary.TempoB += l.TempoB
			summary.TempoC += l.TempoC
			summary.TempoD += l.TempoD
			summary.TempoE += l.TempoE
		}
		summary.TotalHours = summary.TempoA + summary.TempoC + summary.TempoB + summary.TempoD + summary.TempoE
		summary.LeverageRate = CalculateLeverageRate(summary.TempoA, summary.TempoC, summary.TotalHours)
		summary.Diagnosis = ClassifyPeriodDiagnosis(summary)

		summaries = append(summaries, summary)
	}

	return summaries, nil
}

func isoWeekBounds(year, week int) (time.Time, time.Time) {
	// Jan 4th is always in ISO week 1
	jan4 := time.Date(year, time.January, 4, 0, 0, 0, 0, time.UTC)
	weekday := jan4.Weekday()
	if weekday == 0 {
		weekday = 7
	}
	mondayWeek1 := jan4.AddDate(0, 0, -int(weekday-1))
	startMonday := mondayWeek1.AddDate(0, 0, (week-1)*7)
	endSunday := startMonday.AddDate(0, 0, 6)
	return startMonday, endSunday
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
