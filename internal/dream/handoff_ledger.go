//nolint:revive // package comment not required
package dream

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/rjfonseca/daneel/internal/safefile"
)

// HandoffEventKind classifies events persisted in the cumulative handoff ledger.
type HandoffEventKind string

const (
	// HandoffKindDreamCycle records an automated or manual Dream Cycle execution.
	HandoffKindDreamCycle HandoffEventKind = "dream_cycle"
	// HandoffKindEveningClosure records an end-of-day closure summary.
	HandoffKindEveningClosure HandoffEventKind = "evening_closure"
	// HandoffKindAgentAction records a standalone cognitive agent action.
	HandoffKindAgentAction HandoffEventKind = "agent_action"

	handoffLedgerRetentionDays = 30
)

// HandoffEvent is a single chronological entry in .daneel/cache/handoff_events.jsonl.
type HandoffEvent struct {
	Kind             HandoffEventKind  `json:"kind"`
	Timestamp        string            `json:"timestamp"`
	Date             string            `json:"date,omitempty"`
	Summary          string            `json:"summary,omitempty"`
	FilesScanned     int               `json:"files_scanned,omitempty"`
	FixedLinks       int               `json:"fixed_links,omitempty"`
	BrokenLinks      int               `json:"broken_links,omitempty"`
	MissingMOCs      []string          `json:"missing_mocs,omitempty"`
	SyncedPeople     []string          `json:"synced_people,omitempty"`
	IngestedMeetings int               `json:"ingested_meetings,omitempty"`
	Syntheses        []string          `json:"syntheses,omitempty"`
	Actions          []string          `json:"actions,omitempty"`
	InboxPending     []string          `json:"inbox_pending,omitempty"`
	Alerts           []string          `json:"alerts,omitempty"`
	ResolvedAlerts   []string          `json:"resolved_alerts,omitempty"`
	Insights         []string          `json:"insights,omitempty"`
	HandoffQuestions []HandoffQuestion `json:"handoff_questions,omitempty"`
}

// HandoffState tracks the high-watermark timestamp of the last morning handoff.
type HandoffState struct {
	LastMorningHandoffAt string `json:"last_morning_handoff_at"`
	LastDailyFile        string `json:"last_daily_file,omitempty"`
}

// AggregatedHandoff summarizes all ledger events inside a (WindowStart, WindowEnd] interval.
type AggregatedHandoff struct {
	WindowStart           string            `json:"window_start"`
	WindowEnd             string            `json:"window_end"`
	DreamCyclesCount      int               `json:"dream_cycles_count"`
	FirstDreamAt          string            `json:"first_dream_at,omitempty"`
	LastDreamAt           string            `json:"last_dream_at,omitempty"`
	FilesScanned          int               `json:"files_scanned"`
	FixedLinksTotal       int               `json:"fixed_links_total"`
	BrokenLinksLatest     int               `json:"broken_links_latest"`
	MissingMOCs           []string          `json:"missing_mocs,omitempty"`
	SyncedPeople          []string          `json:"synced_people,omitempty"`
	IngestedMeetingsTotal int               `json:"ingested_meetings_total"`
	Syntheses             []string          `json:"syntheses,omitempty"`
	EveningClosures       []HandoffEvent    `json:"evening_closures,omitempty"`
	AgentActions          []HandoffEvent    `json:"agent_actions,omitempty"`
	InboxPending          []string          `json:"inbox_pending,omitempty"`
	Alerts                []string          `json:"alerts,omitempty"`
	ResolvedAlerts        []string          `json:"resolved_alerts,omitempty"`
	Insights              []string          `json:"insights,omitempty"`
	HandoffQuestions      []HandoffQuestion `json:"handoff_questions,omitempty"`
	UsedLegacyFallback    bool              `json:"used_legacy_fallback,omitempty"`
}

var windowLineRegex = regexp.MustCompile(`(?i)>\s*\*\*(?:Window|Janela coberta):\*\*\s*` + "`([^`]+)`" + `\s*(?:➔|->)\s*` + "`([^`]+)`")

func handoffLedgerPath(vaultRoot string) string {
	return filepath.Join(vaultRoot, ".daneel", "cache", "handoff_events.jsonl")
}

func handoffStatePath(vaultRoot string) string {
	return filepath.Join(vaultRoot, ".daneel", "cache", "handoff_state.json")
}

// AppendHandoffEvent appends an event to .daneel/cache/handoff_events.jsonl and prunes entries older than 30 days.
func AppendHandoffEvent(vaultRoot string, ev HandoffEvent) error {
	now := time.Now()
	var ts time.Time
	if strings.TrimSpace(ev.Timestamp) == "" {
		ts = now
		ev.Timestamp = ts.Format(time.RFC3339)
	} else {
		parsed, err := parseFlexibleTime(ev.Timestamp)
		if err != nil {
			ts = now
			ev.Timestamp = ts.Format(time.RFC3339)
		} else {
			ts = parsed
		}
	}
	if strings.TrimSpace(ev.Date) == "" {
		ev.Date = ts.Format("2006-01-02")
	}

	ledgerPath := handoffLedgerPath(vaultRoot)
	if err := os.MkdirAll(filepath.Dir(ledgerPath), 0750); err != nil {
		return fmt.Errorf("create handoff cache dir: %w", err)
	}

	cutoff := now.AddDate(0, 0, -handoffLedgerRetentionDays)
	var keptLines []string

	if data, err := os.ReadFile(ledgerPath); err == nil { //nolint:gosec // intentional vault path
		scanner := bufio.NewScanner(strings.NewReader(string(data)))
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			var existing HandoffEvent
			if err := json.Unmarshal([]byte(line), &existing); err != nil {
				continue
			}
			evTime, err := parseFlexibleTime(existing.Timestamp)
			if err == nil && evTime.Before(cutoff) {
				continue
			}
			keptLines = append(keptLines, line)
		}
	}

	raw, err := json.Marshal(ev)
	if err != nil {
		return fmt.Errorf("marshal handoff event: %w", err)
	}
	if !ts.Before(cutoff) {
		keptLines = append(keptLines, string(raw))
	}

	out := ""
	if len(keptLines) > 0 {
		out = strings.Join(keptLines, "\n") + "\n"
	}
	return safefile.WriteFile(vaultRoot, ledgerPath, []byte(out), 0600)
}

// LoadHandoffState reads the morning handoff high-watermark from .daneel/cache/handoff_state.json.
func LoadHandoffState(vaultRoot string) (*HandoffState, error) {
	data, err := os.ReadFile(handoffStatePath(vaultRoot)) //nolint:gosec // intentional vault path
	if err != nil {
		return nil, err
	}
	var st HandoffState
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, err
	}
	return &st, nil
}

// SaveHandoffState persists the morning handoff high-watermark atomically.
func SaveHandoffState(vaultRoot string, ts time.Time, dailyFile string) error {
	statePath := handoffStatePath(vaultRoot)
	if err := os.MkdirAll(filepath.Dir(statePath), 0750); err != nil {
		return fmt.Errorf("create handoff state dir: %w", err)
	}
	st := HandoffState{
		LastMorningHandoffAt: ts.Format(time.RFC3339),
		LastDailyFile:        dailyFile,
	}
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return safefile.WriteFile(vaultRoot, statePath, b, 0600)
}

// ResolveLastMorningHandoff finds the watermark timestamp of the previous morning handoff.
// Priority: latest timestamp (< now) between handoff_state.json and previous daily/*.md window markers; fallback now - 24h.
func ResolveLastMorningHandoff(vaultRoot, refDate string, now time.Time) time.Time {
	var best time.Time
	hasBest := false

	consider := func(candidate time.Time) {
		if candidate.IsZero() || !candidate.Before(now) {
			return
		}
		if !hasBest || candidate.After(best) {
			best = candidate
			hasBest = true
		}
	}

	if st, err := LoadHandoffState(vaultRoot); err == nil && st.LastMorningHandoffAt != "" {
		if ts, err := parseFlexibleTime(st.LastMorningHandoffAt); err == nil {
			consider(ts)
		}
	}

	dailyDir := filepath.Join(vaultRoot, "daily")
	if entries, err := os.ReadDir(dailyDir); err == nil {
		var names []string
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			dayPart := strings.TrimSuffix(e.Name(), ".md")
			if refDate != "" && dayPart >= refDate {
				continue
			}
			names = append(names, e.Name())
		}
		sort.Sort(sort.Reverse(sort.StringSlice(names)))
		for _, name := range names {
			raw, err := os.ReadFile(filepath.Join(dailyDir, name)) //nolint:gosec // intentional vault path
			if err != nil {
				continue
			}
			if m := windowLineRegex.FindStringSubmatch(string(raw)); len(m) >= 3 {
				if ts, err := parseFlexibleTime(m[2]); err == nil {
					consider(ts)
					break
				}
			}
		}
	}

	if hasBest {
		return best
	}
	return now.Add(-24 * time.Hour)
}

// AggregateHandoffWindow aggregates all ledger events inside (since, until].
// When the JSONL ledger does not exist yet, it falls back to dream_handoff.json for backward compatibility.
func AggregateHandoffWindow(vaultRoot string, since, until time.Time) (*AggregatedHandoff, error) {
	agg := &AggregatedHandoff{
		WindowStart: since.Format(time.RFC3339),
		WindowEnd:   until.Format(time.RFC3339),
	}

	ledgerPath := handoffLedgerPath(vaultRoot)
	data, err := os.ReadFile(ledgerPath) //nolint:gosec // intentional vault path
	if err != nil {
		if os.IsNotExist(err) {
			return aggregateLegacyDreamHandoff(vaultRoot, agg), nil
		}
		return nil, fmt.Errorf("read handoff ledger: %w", err)
	}

	type timedEvent struct {
		ts time.Time
		ev HandoffEvent
	}
	var inWindow []timedEvent

	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var ev HandoffEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			continue
		}
		ts, err := parseFlexibleTime(ev.Timestamp)
		if err != nil {
			continue
		}
		if ts.After(since) && !ts.After(until) {
			inWindow = append(inWindow, timedEvent{ts: ts, ev: ev})
		}
	}

	sort.SliceStable(inWindow, func(i, j int) bool {
		return inWindow[i].ts.Before(inWindow[j].ts)
	})

	mocSeen := map[string]bool{}
	peopleSeen := map[string]bool{}
	synthSeen := map[string]bool{}
	inboxSeen := map[string]bool{}
	resolvedSeen := map[string]bool{}

	var latestDreamAlerts []string
	var latestDreamInsights []string
	var latestDreamQuestions []HandoffQuestion
	var extraAlerts []string
	var extraInsights []string
	var extraQuestions []HandoffQuestion

	for _, item := range inWindow {
		ev := item.ev
		for _, r := range ev.ResolvedAlerts {
			if r != "" && !resolvedSeen[r] {
				resolvedSeen[r] = true
				agg.ResolvedAlerts = append(agg.ResolvedAlerts, r)
			}
		}
		for _, inb := range ev.InboxPending {
			if inb != "" && !inboxSeen[inb] {
				inboxSeen[inb] = true
				agg.InboxPending = append(agg.InboxPending, inb)
			}
		}

		switch ev.Kind {
		case HandoffKindDreamCycle:
			agg.DreamCyclesCount++
			if agg.FirstDreamAt == "" {
				agg.FirstDreamAt = ev.Timestamp
			}
			agg.LastDreamAt = ev.Timestamp
			agg.FilesScanned = ev.FilesScanned
			agg.FixedLinksTotal += ev.FixedLinks
			agg.BrokenLinksLatest = ev.BrokenLinks
			agg.IngestedMeetingsTotal += ev.IngestedMeetings

			for _, m := range ev.MissingMOCs {
				if m != "" && !mocSeen[m] {
					mocSeen[m] = true
					agg.MissingMOCs = append(agg.MissingMOCs, m)
				}
			}
			for _, p := range ev.SyncedPeople {
				if p != "" && !peopleSeen[p] {
					peopleSeen[p] = true
					agg.SyncedPeople = append(agg.SyncedPeople, p)
				}
			}
			for _, s := range ev.Syntheses {
				if s != "" && !synthSeen[s] {
					synthSeen[s] = true
					agg.Syntheses = append(agg.Syntheses, s)
				}
			}

			// Point-in-time state: latest dream cycle supersedes earlier dream alerts/insights/questions
			latestDreamAlerts = append([]string(nil), ev.Alerts...)
			latestDreamInsights = append([]string(nil), ev.Insights...)
			latestDreamQuestions = append([]HandoffQuestion(nil), ev.HandoffQuestions...)

		case HandoffKindEveningClosure:
			agg.EveningClosures = append(agg.EveningClosures, ev)
			extraAlerts = append(extraAlerts, ev.Alerts...)
			extraInsights = append(extraInsights, ev.Insights...)
			extraQuestions = append(extraQuestions, ev.HandoffQuestions...)

		case HandoffKindAgentAction:
			agg.AgentActions = append(agg.AgentActions, ev)
			extraAlerts = append(extraAlerts, ev.Alerts...)
			extraInsights = append(extraInsights, ev.Insights...)
			extraQuestions = append(extraQuestions, ev.HandoffQuestions...)
		}
	}

	alertSeen := map[string]bool{}
	for _, a := range append(latestDreamAlerts, extraAlerts...) {
		if a != "" && !resolvedSeen[a] && !alertSeen[a] {
			alertSeen[a] = true
			agg.Alerts = append(agg.Alerts, a)
		}
	}

	insightSeen := map[string]bool{}
	for _, in := range append(latestDreamInsights, extraInsights...) {
		if in != "" && !insightSeen[in] {
			insightSeen[in] = true
			agg.Insights = append(agg.Insights, in)
		}
	}

	qSeen := map[string]bool{}
	for _, q := range append(latestDreamQuestions, extraQuestions...) {
		if q.Question != "" && !qSeen[q.Question] {
			qSeen[q.Question] = true
			agg.HandoffQuestions = append(agg.HandoffQuestions, q)
		}
	}

	return agg, nil
}

func aggregateLegacyDreamHandoff(vaultRoot string, agg *AggregatedHandoff) *AggregatedHandoff {
	handoffPath := filepath.Join(vaultRoot, ".daneel", "cache", "dream_handoff.json")
	data, err := os.ReadFile(handoffPath) //nolint:gosec // intentional vault path
	if err != nil {
		handoffPath = filepath.Join(vaultRoot, ".daneel", "dream_handoff.json")
		data, err = os.ReadFile(handoffPath) //nolint:gosec // intentional vault path
	}
	if err != nil {
		return agg
	}

	var dreamReport FullDreamReport
	if err := json.Unmarshal(data, &dreamReport); err != nil {
		return agg
	}

	agg.UsedLegacyFallback = true
	agg.DreamCyclesCount = 1
	agg.FilesScanned = dreamReport.TotalFilesScanned
	agg.FixedLinksTotal = dreamReport.FixedLinksCount
	agg.BrokenLinksLatest = dreamReport.BrokenLinksCount
	agg.MissingMOCs = append([]string(nil), dreamReport.MissingMOCs...)
	agg.SyncedPeople = append([]string(nil), dreamReport.SyncedPeople...)
	agg.IngestedMeetingsTotal = dreamReport.IngestedMeetingsCount
	agg.HandoffQuestions = append([]HandoffQuestion(nil), dreamReport.HandoffQuestions...)

	if dreamReport.BrokenLinksCount > 0 {
		agg.Alerts = append(agg.Alerts, fmt.Sprintf("%d broken links pending review", dreamReport.BrokenLinksCount))
	}
	if dreamReport.TemporalAnalysis != nil && len(dreamReport.TemporalAnalysis.DriftAlerts) > 0 {
		agg.Alerts = append(agg.Alerts, dreamReport.TemporalAnalysis.DriftAlerts...)
	}
	if dreamReport.Metacognition != nil {
		for _, in := range dreamReport.Metacognition.Insights {
			agg.Insights = append(agg.Insights, fmt.Sprintf("%s -> %s", in.Observation, in.SuggestedAction))
		}
		for _, s := range dreamReport.Metacognition.Syntheses {
			if s.CreatedPath != "" {
				agg.Syntheses = append(agg.Syntheses, s.CreatedPath)
			} else if s.Slug != "" {
				agg.Syntheses = append(agg.Syntheses, s.Slug)
			}
		}
	}

	return agg
}

// BuildDreamHandoffEvent converts a FullDreamReport into a compact HandoffEvent for the JSONL ledger.
func BuildDreamHandoffEvent(date string, report *FullDreamReport) HandoffEvent {
	ev := HandoffEvent{
		Kind:      HandoffKindDreamCycle,
		Timestamp: time.Now().Format(time.RFC3339),
		Date:      date,
	}
	if report == nil {
		return ev
	}
	if report.GeneratedAt != "" {
		if _, err := parseFlexibleTime(report.GeneratedAt); err == nil {
			ev.Timestamp = report.GeneratedAt
		}
	}
	ev.FilesScanned = report.TotalFilesScanned
	ev.FixedLinks = report.FixedLinksCount
	ev.BrokenLinks = report.BrokenLinksCount
	ev.MissingMOCs = append([]string(nil), report.MissingMOCs...)
	ev.SyncedPeople = append([]string(nil), report.SyncedPeople...)
	ev.IngestedMeetings = report.IngestedMeetingsCount
	ev.HandoffQuestions = append([]HandoffQuestion(nil), report.HandoffQuestions...)

	if report.BrokenLinksCount > 0 {
		ev.Alerts = append(ev.Alerts, fmt.Sprintf("%d broken links pending review", report.BrokenLinksCount))
	}
	if report.TemporalAnalysis != nil && len(report.TemporalAnalysis.DriftAlerts) > 0 {
		ev.Alerts = append(ev.Alerts, report.TemporalAnalysis.DriftAlerts...)
	}
	if report.Metacognition != nil {
		for _, in := range report.Metacognition.Insights {
			ev.Insights = append(ev.Insights, fmt.Sprintf("%s -> %s", in.Observation, in.SuggestedAction))
		}
		for _, s := range report.Metacognition.Syntheses {
			if s.CreatedPath != "" {
				ev.Syntheses = append(ev.Syntheses, s.CreatedPath)
			} else if s.Slug != "" {
				ev.Syntheses = append(ev.Syntheses, s.Slug)
			}
		}
	}
	return ev
}

// AppendCognitiveEvolutionLog appends a chronological cognitive closure or action entry to meta/guias/agente-evolucao.md.
func AppendCognitiveEvolutionLog(vaultRoot string, ev HandoffEvent) error {
	logPath := filepath.Join(vaultRoot, "meta", "guias", "agente-evolucao.md")
	if err := os.MkdirAll(filepath.Dir(logPath), 0750); err != nil {
		return fmt.Errorf("create evolution log dir: %w", err)
	}

	var existingContent string
	if data, err := os.ReadFile(logPath); err == nil { //nolint:gosec // intentional vault path
		existingContent = string(data)
	} else {
		existingContent = `# 🧬 Agent Evolution Log

Chronological log of Dream Cycle executions, cognitive closures, and skill calibrations.

---
`
	}

	ts, err := parseFlexibleTime(ev.Timestamp)
	if err != nil {
		ts = time.Now()
	}
	header := fmt.Sprintf("## [%s] Cognitive Closure", ts.Format("2006-01-02 15:04"))
	if strings.TrimSpace(ev.Summary) != "" {
		header += " — " + strings.TrimSpace(ev.Summary)
	}

	var sb strings.Builder
	sb.WriteString("\n" + header + "\n")
	fmt.Fprintf(&sb, "- **Kind:** `%s`\n", ev.Kind)
	if len(ev.Actions) > 0 {
		sb.WriteString("- **Autonomous Actions:**\n")
		for _, act := range ev.Actions {
			fmt.Fprintf(&sb, "  - %s\n", act)
		}
	}
	if len(ev.InboxPending) > 0 {
		sb.WriteString("- **Inbox Pending Human Decision:**\n")
		for _, inb := range ev.InboxPending {
			fmt.Fprintf(&sb, "  - %s\n", inb)
		}
	}
	if len(ev.Alerts) > 0 {
		sb.WriteString("- **Active Alerts:**\n")
		for _, a := range ev.Alerts {
			fmt.Fprintf(&sb, "  - %s\n", a)
		}
	}
	if len(ev.ResolvedAlerts) > 0 {
		sb.WriteString("- **Resolved Alerts:**\n")
		for _, r := range ev.ResolvedAlerts {
			fmt.Fprintf(&sb, "  - %s\n", r)
		}
	}
	sb.WriteString("\n---\n")

	return safefile.WriteFile(vaultRoot, logPath, []byte(existingContent+sb.String()), 0600)
}

func parseFlexibleTime(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if layout == time.RFC3339Nano || layout == time.RFC3339 {
			if t, err := time.Parse(layout, raw); err == nil {
				return t, nil
			}
		} else {
			if t, err := time.ParseInLocation(layout, raw, time.Local); err == nil {
				return t, nil
			}
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized timestamp %q", raw)
}
