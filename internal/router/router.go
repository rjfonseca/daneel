//nolint:revive // package comment not required
package router

import (
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// IntentType represents a classified user intent.
type IntentType string

// MarshalText implements encoding.TextMarshaler for TOON and JSON compatibility.
func (it IntentType) MarshalText() ([]byte, error) {
	return []byte(string(it)), nil
}

// String implements fmt.Stringer.
func (it IntentType) String() string {
	return string(it)
}

const (
	IntentNone           IntentType = "none"
	IntentRoutineMorning IntentType = "routine_morning"
	IntentRoutineEvening IntentType = "routine_evening"
	IntentVaultHealth    IntentType = "vault_health"
	IntentPeopleBriefing IntentType = "people_briefing"
	IntentWorkspace      IntentType = "workspace_status"
)

// RouteMatch holds classification results.
type RouteMatch struct {
	Intent     IntentType        `json:"intent"`
	Confidence float64           `json:"confidence"`
	Params     map[string]string `json:"params,omitempty"`
}

// Router provides CPU-based sub-millisecond intent classification.
type Router struct {
	briefingRegex1 *regexp.Regexp
	briefingRegex2 *regexp.Regexp
	briefingRegex3 *regexp.Regexp
}

// New creates a new CPU intent Router.
func New() *Router {
	return &Router{
		briefingRegex1: regexp.MustCompile(`(?i)^(?:1:1\s+(?:with|com)\s+|1on1\s+(?:with|com)\s+|briefing\s+(?:de\s+|com\s+|para\s+)?)(.+)$`),
		briefingRegex2: regexp.MustCompile(`(?i)^(?:quem\s+[eé]\s+|perfil\s+(?:de\s+)?)(.+)$`),
		briefingRegex3: regexp.MustCompile(`(?i)^prep\s+(.+)$`),
	}
}

// Evaluate classifies an input query into an intent with confidence score.
func (r *Router) Evaluate(input string) RouteMatch {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return RouteMatch{Intent: IntentNone, Confidence: 0.0}
	}

	normInput := normalizeText(trimmed)

	// Check People Briefing regexes first to capture target name
	if match := r.briefingRegex1.FindStringSubmatch(trimmed); len(match) > 1 {
		name := strings.TrimSpace(match[1])
		if !isRoutineKeyword(name) && len(name) > 1 {
			return RouteMatch{
				Intent:     IntentPeopleBriefing,
				Confidence: 0.95,
				Params:     map[string]string{"name": name},
			}
		}
	}
	if match := r.briefingRegex2.FindStringSubmatch(trimmed); len(match) > 1 {
		name := strings.TrimSpace(match[1])
		return RouteMatch{
			Intent:     IntentPeopleBriefing,
			Confidence: 0.95,
			Params:     map[string]string{"name": name},
		}
	}
	if match := r.briefingRegex3.FindStringSubmatch(trimmed); len(match) > 1 {
		name := strings.TrimSpace(match[1])
		return RouteMatch{
			Intent:     IntentPeopleBriefing,
			Confidence: 0.95,
			Params:     map[string]string{"name": name},
		}
	}

	// Exact & Anchor Matches for Routines & System Health
	switch normInput {
	case "briefing matinal", "rotina matinal", "morning routine", "iniciar dia", "comecar dia", "start day", "morning":
		return RouteMatch{Intent: IntentRoutineMorning, Confidence: 0.98}
	case "rotina noturna", "evening routine", "fechar dia", "encerrar dia", "metacognicao", "close day", "evening":
		return RouteMatch{Intent: IntentRoutineEvening, Confidence: 0.98}
	case "status do cofre", "saude do cofre", "vault health", "doctor", "diagnostico", "daneel doctor", "health":
		return RouteMatch{Intent: IntentVaultHealth, Confidence: 0.98}
	case "workspace status", "dirty repos", "status do workspace", "repos sujos", "status dos repos", "workspace":
		return RouteMatch{Intent: IntentWorkspace, Confidence: 0.98}
	}

	// Token overlap analysis for conversational phrasing
	tokens := strings.Fields(normInput)
	tokenSet := make(map[string]bool, len(tokens))
	for _, t := range tokens {
		tokenSet[t] = true
	}

	// Morning indicators
	if tokenSet["matinal"] || (tokenSet["rotina"] && tokenSet["manha"]) || (tokenSet["morning"] && tokenSet["routine"]) {
		return RouteMatch{Intent: IntentRoutineMorning, Confidence: 0.90}
	}

	// Evening indicators
	if (tokenSet["rotina"] && (tokenSet["noturna"] || tokenSet["noite"])) || (tokenSet["evening"] && tokenSet["routine"]) {
		return RouteMatch{Intent: IntentRoutineEvening, Confidence: 0.90}
	}

	// Vault health indicators
	if (tokenSet["status"] || tokenSet["saude"]) && tokenSet["cofre"] {
		return RouteMatch{Intent: IntentVaultHealth, Confidence: 0.90}
	}

	// Workspace indicators
	if (tokenSet["workspace"] && tokenSet["status"]) || (tokenSet["repos"] && tokenSet["sujos"]) || (tokenSet["dirty"] && tokenSet["repos"]) {
		return RouteMatch{Intent: IntentWorkspace, Confidence: 0.90}
	}

	return RouteMatch{
		Intent:     IntentNone,
		Confidence: 0.0,
	}
}

func isRoutineKeyword(s string) bool {
	norm := normalizeText(s)
	return norm == "matinal" || norm == "manha" || norm == "noturna" || norm == "noite"
}

func normalizeText(s string) string {
	lower := strings.ToLower(strings.TrimSpace(s))
	// Strip accents/diacritics
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	res, _, err := transform.String(t, lower)
	if err != nil {
		return lower
	}
	return res
}
