//nolint:revive // package comment not required
package dream

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/rjfonseca/daneel/internal/safefile"

	"github.com/rjfonseca/daneel/internal/config"
)

// AgentLearnings holds learned categories, taxonomy preferences, and feedback history.
type AgentLearnings struct {
	LastUpdated                 string              `json:"last_updated"`
	CategoryPreferences         map[string]string   `json:"category_preferences"`
	StaffTaxonomyRules          map[string][]string `json:"staff_taxonomy_rules"`
	BusinessInitiativesKeywords map[string][]string `json:"business_initiatives_keywords"`
	FeedbackHistory             []FeedbackEntry     `json:"feedback_history"`
	ScannedTurns                int                 `json:"scanned_turns"`
	SuspiciousDeltas            int                 `json:"suspicious_deltas"`
	MissedOpportunities         int                 `json:"missed_opportunities"`
	KnowledgeGaps               int                 `json:"knowledge_gaps"`
}

// FeedbackEntry represents a single calibration or learned user preference.
type FeedbackEntry struct {
	Date      string `json:"date"`
	Timestamp string `json:"timestamp"`
	Topic     string `json:"topic"`
	Type      string `json:"type"`
	Action    string `json:"action"`
	Note      string `json:"note"`
	Agent     string `json:"agent,omitempty"`
}

// DefaultLearnings returns the default cognitive heuristics.
func DefaultLearnings() *AgentLearnings {
	return &AgentLearnings{
		LastUpdated: time.Now().Format(time.RFC3339),
		CategoryPreferences: map[string]string{
			"agent": "notes/ia", "mcp": "notes/ia", "rag": "notes/ia",
			"prompt": "notes/ia", "gateway": "notes/sistemas", "proxy": "notes/sistemas",
			"keycloak": "notes/sistemas", "service": "notes/sistemas", "rfc": "notes/processors",
			"prd": "notes/processors", "sla": "notes/processors", "pdi": "notes/carreira",
			"roadmap": "notes/carreira",
		},
		StaffTaxonomyRules: map[string][]string{
			"A_Estrategico":  {"rfc", "arquitetura", "visao", "longo prazo", "estrategia", "plataforma", "architecture", "vision", "strategy"},
			"B_ImpactoLocal": {"fix", "bug", "pentest", "rate limit", "sdk", "mr", "pr", "poc", "refactor"},
			"C_GlueWork":     {"1:1", "mentoria", "alinhamento", "code review", "facilita", "entrevista", "mentorship", "alignment"},
			"E_Overhead":     {"all-hands", "bimestral", "institucional", "slack", "email", "administrative"},
		},
		BusinessInitiativesKeywords: map[string][]string{
			"Cloud & Infrastructure":     {"cloud", "kubernetes", "infra", "gateway", "service mesh", "terraform"},
			"Identity & Access":          {"identity", "auth", "oauth2", "sso", "rbac", "keycloak"},
			"AI & Agent Platform":        {"agentes", "agents", "mcp", "daneel", "llm platform", "rag"},
			"Resilience & Security":      {"pentest", "rate limit", "resiliencia", "cve", "seguranca", "security"},
			"Developer Experience & SDK": {"sdk", "devkit", "cli", "tooling", "framework"},
		},
		FeedbackHistory: []FeedbackEntry{},
	}
}

// LoadLearnings loads the learnings cache from agent_learnings.json or config defaults.
func LoadLearnings(vaultRoot string) *AgentLearnings {
	cachePath := filepath.Join(config.ResolveVaultCacheDir(vaultRoot), "agent_learnings.json")
	defaults := DefaultLearnings()

	// Load user preferences from vault configuration if available
	if cfg, err := config.LoadConfig(vaultRoot, "", false); err == nil {
		if taxMap := cfg.GetTaxonomyMap(); len(taxMap) > 0 {
			defaults.StaffTaxonomyRules = taxMap
		}
		if initMap := cfg.GetInitiativesMap(); len(initMap) > 0 {
			defaults.BusinessInitiativesKeywords = initMap
		}
		if routingMap := cfg.GetRoutingMap(); len(routingMap) > 0 {
			catPref := make(map[string]string)
			for folder, kws := range routingMap {
				for _, kw := range kws {
					catPref[kw] = folder
				}
			}
			defaults.CategoryPreferences = catPref
		}
	}

	data, err := os.ReadFile(cachePath) //nolint:gosec // intentional
	// Legacy local feedback remains readable after the canonical cache is created.
	legacyPath := filepath.Join(vaultRoot, ".agents", "cache", "agent_learnings.json")
	if err != nil && vaultRoot != "" && cachePath != legacyPath {
		data, err = os.ReadFile(legacyPath)
	}
	if err != nil && vaultRoot != "" && cachePath != filepath.Join(config.XDGCacheDir(), "agent_learnings.json") {
		data, err = os.ReadFile(filepath.Join(config.XDGCacheDir(), "agent_learnings.json"))
	}
	if err != nil {
		return defaults
	}

	var l AgentLearnings
	if err := json.Unmarshal(data, &l); err != nil {
		return defaults
	}

	if len(l.CategoryPreferences) == 0 {
		l.CategoryPreferences = defaults.CategoryPreferences
	}
	if len(l.StaffTaxonomyRules) == 0 {
		l.StaffTaxonomyRules = defaults.StaffTaxonomyRules
	}
	if len(l.BusinessInitiativesKeywords) == 0 {
		l.BusinessInitiativesKeywords = defaults.BusinessInitiativesKeywords
	}

	return &l
}

// RecordFeedback saves a feedback entry into JSON cache and appends to meta/guias/aprendizados-agente.md.
func RecordFeedback(vaultRoot string, topic string, feedbackType string, action string, note string) error {
	return RecordFeedbackWithAgent(vaultRoot, topic, feedbackType, action, note, "agent")
}

// RecordFeedbackWithAgent saves a feedback entry with agent identity into JSON cache and appends to markdown.
func RecordFeedbackWithAgent(vaultRoot string, topic string, feedbackType string, action string, note string, agent string) error {
	learnings := LoadLearnings(vaultRoot)
	entry := FeedbackEntry{
		Date:      time.Now().Format("2006-01-02"),
		Timestamp: time.Now().Format(time.RFC3339),
		Topic:     topic,
		Type:      feedbackType,
		Action:    action,
		Note:      note,
		Agent:     agent,
	}

	learnings.FeedbackHistory = append(learnings.FeedbackHistory, entry)
	learnings.LastUpdated = entry.Timestamp

	cachePath := filepath.Join(vaultRoot, ".daneel", "cache", "agent_learnings.json")
	data, err := json.MarshalIndent(learnings, "", "  ")
	if err != nil {
		return err
	}
	if err := safefile.WriteFile(vaultRoot, cachePath, data, 0600); err != nil {
		return err
	}

	// Append to meta/guias/aprendizados-agente.md (or fallback meta/guias/agente-evolucao.md)
	docPath := filepath.Join(vaultRoot, "meta", "guias", "aprendizados-agente.md")
	if _, err := os.Stat(docPath); os.IsNotExist(err) {
		altPath := filepath.Join(vaultRoot, "meta", "guias", "agente-evolucao.md")
		if _, err := os.Stat(altPath); err == nil {
			docPath = altPath
		}
	}
	docData, err := os.ReadFile(docPath) //nolint:gosec // intentional
	if err == nil {
		docContent := string(docData)
		logLine := fmt.Sprintf("- **%s:** [%s] %s ➔ %s. %s\n", entry.Date, strings.ToUpper(feedbackType), topic, action, note)
		historicLogRegex := regexp.MustCompile(`(## 📝 \d+\. Log Histórico de Feedbacks[^\n]*\n\n)`)
		if historicLogRegex.MatchString(docContent) {
			docContent = historicLogRegex.ReplaceAllString(docContent, "${1}"+logLine)
			if err := safefile.WriteFile(vaultRoot, docPath, []byte(docContent), 0600); err != nil {
				return err
			}
		}
	}

	return nil
}

// ArchivedLearningsMonth stores consolidated feedback entries for a single YYYY-MM partition.
type ArchivedLearningsMonth struct {
	Month       string          `json:"month"`
	LastUpdated string          `json:"last_updated"`
	Entries     []FeedbackEntry `json:"entries"`
}

// ArchiveLearningsResult summarizes an archive pass over agent_learnings.json.
type ArchiveLearningsResult struct {
	ArchivedCount    int      `json:"archived_count"`
	RemainingPending int      `json:"remaining_pending"`
	ArchiveFiles     []string `json:"archive_files"`
	DryRun           bool     `json:"dry_run"`
}

// LearningsArchiveDir returns the directory storing monthly archived learnings JSON files.
func LearningsArchiveDir(vaultRoot string) string {
	return filepath.Join(config.ResolveVaultCacheDir(vaultRoot), "learnings_archive")
}

// SaveLearnings persists AgentLearnings to the hot cache file, falling back to XDGCacheDir when needed.
func SaveLearnings(vaultRoot string, learnings *AgentLearnings) error {
	if learnings == nil {
		return nil
	}
	data, err := json.MarshalIndent(learnings, "", "  ")
	if err != nil {
		return err
	}

	cacheDir := config.ResolveVaultCacheDir(vaultRoot)
	cachePath := filepath.Join(cacheDir, "agent_learnings.json")
	if err := os.MkdirAll(cacheDir, 0750); err == nil {
		if vaultRoot != "" && strings.HasPrefix(filepath.Clean(cachePath), filepath.Clean(vaultRoot)+string(filepath.Separator)) {
			if err := safefile.WriteFile(vaultRoot, cachePath, data, 0600); err == nil {
				return nil
			}
		} else if err := os.WriteFile(cachePath, data, 0600); err == nil {
			return nil
		}
	}

	fallbackDir := config.XDGCacheDir()
	if err := os.MkdirAll(fallbackDir, 0750); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(fallbackDir, "agent_learnings.json"), data, 0600)
}

// LoadArchivedFeedbackEntries loads all feedback entries previously archived in monthly JSON files.
func LoadArchivedFeedbackEntries(vaultRoot string) []FeedbackEntry {
	archiveDir := LearningsArchiveDir(vaultRoot)
	entries, err := os.ReadDir(archiveDir)
	if err != nil {
		return nil
	}

	var all []FeedbackEntry
	for _, de := range entries {
		if de.IsDir() || !strings.HasSuffix(de.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(archiveDir, de.Name())) //nolint:gosec // intentional
		if err != nil {
			continue
		}
		var month ArchivedLearningsMonth
		if err := json.Unmarshal(data, &month); err != nil {
			continue
		}
		all = append(all, month.Entries...)
	}
	return all
}

func feedbackEntryMonth(entry FeedbackEntry) string {
	if len(entry.Date) >= 7 && entry.Date[4] == '-' {
		return entry.Date[:7]
	}
	if len(entry.Timestamp) >= 7 && entry.Timestamp[4] == '-' {
		return entry.Timestamp[:7]
	}
	return time.Now().Format("2006-01")
}

func feedbackDedupKey(entry FeedbackEntry) string {
	return entry.Date + "\x00" + entry.Topic + "\x00" + entry.Type + "\x00" + entry.Action
}

// ArchiveFeedbackHistory moves pending feedback_history entries from the hot agent_learnings.json
// cache into monthly files under learnings_archive/YYYY-MM.json, keeping the hot file small.
func ArchiveFeedbackHistory(vaultRoot string, dryRun bool) (*ArchiveLearningsResult, error) {
	learnings := LoadLearnings(vaultRoot)
	if len(learnings.FeedbackHistory) == 0 {
		return &ArchiveLearningsResult{
			ArchivedCount:    0,
			RemainingPending: 0,
			ArchiveFiles:     []string{},
			DryRun:           dryRun,
		}, nil
	}

	byMonth := make(map[string][]FeedbackEntry)
	for _, entry := range learnings.FeedbackHistory {
		m := feedbackEntryMonth(entry)
		byMonth[m] = append(byMonth[m], entry)
	}

	months := make([]string, 0, len(byMonth))
	for m := range byMonth {
		months = append(months, m)
	}
	sort.Strings(months)

	archiveDir := LearningsArchiveDir(vaultRoot)
	archiveFiles := make([]string, 0, len(months))
	nowStr := time.Now().Format(time.RFC3339)

	for _, m := range months {
		targetPath := filepath.Join(archiveDir, m+".json")
		relPath := targetPath
		if vaultRoot != "" {
			if rel, err := filepath.Rel(vaultRoot, targetPath); err == nil {
				relPath = rel
			}
		}
		archiveFiles = append(archiveFiles, relPath)

		if dryRun {
			continue
		}

		if err := os.MkdirAll(archiveDir, 0750); err != nil {
			return nil, err
		}

		existing := ArchivedLearningsMonth{
			Month:   m,
			Entries: []FeedbackEntry{},
		}
		if data, err := os.ReadFile(targetPath); err == nil { //nolint:gosec // intentional
			_ = json.Unmarshal(data, &existing)
		}
		seen := make(map[string]bool, len(existing.Entries))
		for _, e := range existing.Entries {
			seen[feedbackDedupKey(e)] = true
		}
		for _, e := range byMonth[m] {
			k := feedbackDedupKey(e)
			if !seen[k] {
				existing.Entries = append(existing.Entries, e)
				seen[k] = true
			}
		}
		existing.Month = m
		existing.LastUpdated = nowStr

		raw, err := json.MarshalIndent(existing, "", "  ")
		if err != nil {
			return nil, err
		}
		if err := safefile.WriteFile(vaultRoot, targetPath, raw, 0600); err != nil {
			return nil, err
		}
	}

	archivedCount := len(learnings.FeedbackHistory)
	remaining := archivedCount
	if !dryRun {
		learnings.FeedbackHistory = []FeedbackEntry{}
		learnings.LastUpdated = nowStr
		if err := SaveLearnings(vaultRoot, learnings); err != nil {
			return nil, err
		}
		remaining = 0
	}

	return &ArchiveLearningsResult{
		ArchivedCount:    archivedCount,
		RemainingPending: remaining,
		ArchiveFiles:     archiveFiles,
		DryRun:           dryRun,
	}, nil
}
