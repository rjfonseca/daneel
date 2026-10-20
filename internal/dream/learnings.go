//nolint:revive // package comment not required
package dream

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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
