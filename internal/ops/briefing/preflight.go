//nolint:revive // package comment not required
package briefing

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/rjfonseca/daneel/internal/ops/backlog"
	"github.com/rjfonseca/daneel/internal/vault"
)

// PersonProfile represents the structured profile of a team member.
type PersonProfile struct {
	Name                string   `json:"name"`
	Slug                string   `json:"slug"`
	RelPath             string   `json:"rel_path"`
	Cargo               string   `json:"cargo"`
	Squad               string   `json:"squad"`
	Relacao             string   `json:"relacao"`
	RecentInteractions  []string `json:"recent_interactions"`
	Topics              []string `json:"topics"`
	PendingTasks        []string `json:"pending_tasks"`
	RecentDailyMentions []string `json:"recent_daily_mentions"`
}

// PreflightService orchestrates 1:1 briefing generation.
type PreflightService struct {
	vaultRoot string
}

// NewPreflightService creates a new instance.
func NewPreflightService(vaultRoot string) *PreflightService {
	return &PreflightService{vaultRoot: vaultRoot}
}

// GenerateBriefing generates a complete preflight briefing for a person or query.
func (s *PreflightService) GenerateBriefing(query string) (*PersonProfile, error) {
	var searchDirs []string
	pNotes := filepath.Join(s.vaultRoot, "notes", "pessoas")
	if _, err := os.Stat(pNotes); err == nil {
		searchDirs = append(searchDirs, pNotes)
	}
	pMeta := filepath.Join(s.vaultRoot, "meta", "pessoas")
	if _, err := os.Stat(pMeta); err == nil {
		searchDirs = append(searchDirs, pMeta)
	}
	if len(searchDirs) == 0 {
		return nil, fmt.Errorf("people directory not found in notes/pessoas or meta/pessoas")
	}

	trimmedQuery := strings.TrimSpace(query)
	normQuery := vault.NormalizeText(trimmedQuery)
	normQueryClean := strings.ReplaceAll(normQuery, "-", " ")
	qTokens := strings.Fields(normQueryClean)
	kebabQuery := vault.ToKebabCase(trimmedQuery)

	type candidate struct {
		path  string
		slug  string
		score float64
	}

	var candidates []candidate

	for _, dir := range searchDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".md") {
				continue
			}
			filePath := filepath.Join(dir, entry.Name())
			slug := strings.TrimSuffix(entry.Name(), ".md")
			normSlug := vault.NormalizeText(slug)
			slugTokens := strings.Split(normSlug, "-")

			note, err := vault.ParseNote(filePath, s.vaultRoot)
			if err != nil || note == nil {
				continue
			}

			normTitle := vault.NormalizeText(note.Title)
			normTitleClean := strings.ReplaceAll(normTitle, "-", " ")
			normTitleClean = strings.TrimPrefix(normTitleClean, "👤 ")
			titleTokens := strings.Fields(normTitleClean)

			score := 0.0
			// 1. Exact match on slug or title
			if normSlug == kebabQuery || normSlug == normQuery {
				score = 1.0
			} else if normTitleClean == normQueryClean {
				score = 0.99
			} else {
				// Check aliases
				matchedAlias := false
				for _, a := range note.Aliases {
					normA := strings.ReplaceAll(vault.NormalizeText(a), "-", " ")
					if normA == normQueryClean {
						score = 0.98
						matchedAlias = true
						break
					}
				}
				if !matchedAlias {
					// Compound token match in name
					if len(qTokens) > 0 && isSubsequenceOrSubset(qTokens, titleTokens) {
						score = 0.95
					} else if len(qTokens) > 0 && isSubsequenceOrSubset(qTokens, slugTokens) {
						score = 0.92
					} else {
						// Compound token match in aliases
						for _, a := range note.Aliases {
							aTokens := strings.Fields(strings.ReplaceAll(vault.NormalizeText(a), "-", " "))
							if len(qTokens) > 0 && isSubsequenceOrSubset(qTokens, aTokens) {
								score = 0.90
								break
							}
						}
					}
					// Substring match
					if score == 0.0 {
						if strings.Contains(normTitleClean, normQueryClean) || strings.Contains(normSlug, normQuery) {
							score = 0.80
						}
						for _, a := range note.Aliases {
							if strings.Contains(strings.ReplaceAll(vault.NormalizeText(a), "-", " "), normQueryClean) {
								score = 0.78
								break
							}
						}
					}
				}
			}

			if score >= 0.50 {
				candidates = append(candidates, candidate{
					path:  filePath,
					slug:  slug,
					score: score,
				})
			}
		}
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no person profile found matching query: '%s'", query)
	}

	// Pick highest scoring candidate
	bestMatch := candidates[0]
	for _, c := range candidates[1:] {
		if c.score > bestMatch.score {
			bestMatch = c
		}
	}

	matchedFile := bestMatch.path
	matchedSlug := bestMatch.slug

	relPath, _ := filepath.Rel(s.vaultRoot, matchedFile)
	relPath = filepath.ToSlash(relPath)

	contentBytes, err := os.ReadFile(matchedFile) //nolint:gosec // intentional
	if err != nil {
		return nil, err
	}
	content := string(contentBytes)

	// Extract Title
	name := strings.Title(strings.ReplaceAll(matchedSlug, "-", " ")) //nolint:staticcheck
	h1Regex := regexp.MustCompile(`(?m)^#\s+(.*?)$`)
	if m := h1Regex.FindStringSubmatch(content); len(m) > 1 {
		name = strings.TrimSpace(m[1])
		name = strings.TrimPrefix(name, "👤")
		name = strings.TrimSpace(name)
	}

	// Extract Cargo, Squad, Relacao
	cargo := "Engineer / Leadership"
	if m := regexp.MustCompile(`(?i)\*\*(?:Cargo|Role)\s*/\s*(?:Papel|Title):\*\*\s*(.*?)$`).FindStringSubmatch(content); len(m) > 1 {
		cargo = strings.TrimSpace(m[1])
	} else if m := regexp.MustCompile(`(?m)^role:\s*(.*?)$`).FindStringSubmatch(content); len(m) > 1 {
		cargo = strings.TrimSpace(m[1])
	}

	squad := "Engineering"
	if m := regexp.MustCompile(`(?i)\*\*(?:Squad|Tribe)\s*/\s*(?:Tribo|Team):\*\*\s*(.*?)$`).FindStringSubmatch(content); len(m) > 1 {
		squad = strings.TrimSpace(m[1])
	} else if m := regexp.MustCompile(`(?m)^squad:\s*(.*?)$`).FindStringSubmatch(content); len(m) > 1 {
		squad = strings.TrimSpace(m[1])
	}

	relacao := "Peer"
	if m := regexp.MustCompile(`(?i)\*\*(?:Realção|Relationship):\*\*\s*(.*?)$`).FindStringSubmatch(content); len(m) > 1 {
		relacao = strings.TrimSpace(m[1])
	}

	// Extract Interactions and Topics line by line
	var interactions []string
	var topics []string
	inInteractions := false
	inTopics := false

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.Contains(trimmed, "Histórico de Interações") || strings.Contains(trimmed, "Interactions") {
			inInteractions = true
			inTopics = false
			continue
		}
		if strings.Contains(trimmed, "Tópicos de Interesse") || strings.Contains(trimmed, "Topics") {
			inTopics = true
			inInteractions = false
			continue
		}
		if strings.HasPrefix(trimmed, "## ") && !strings.Contains(trimmed, "Histórico") && !strings.Contains(trimmed, "Interactions") && !strings.Contains(trimmed, "Tópicos") && !strings.Contains(trimmed, "Topics") {
			inInteractions = false
			inTopics = false
			continue
		}

		if inInteractions && strings.HasPrefix(trimmed, "-") {
			clean := regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`).ReplaceAllString(trimmed, "$1")
			clean = strings.TrimLeft(clean, "- *")
			if len(clean) > 0 && !strings.Contains(clean, "Nenhum registro") {
				interactions = append(interactions, clean)
			}
		}

		if inTopics && strings.HasPrefix(trimmed, "-") {
			clean := strings.TrimLeft(trimmed, "- *")
			if len(clean) > 0 {
				topics = append(topics, clean)
			}
		}
	}

	// Find pending tasks
	pending := s.findPendingTasks(name, matchedSlug, squad)

	// Scan recent daily mentions
	mentions := s.scanRecentDailyMentions(name, matchedSlug, 30)

	profile := &PersonProfile{
		Name:                name,
		Slug:                matchedSlug,
		RelPath:             relPath,
		Cargo:               cargo,
		Squad:               squad,
		Relacao:             relacao,
		RecentInteractions:  interactions,
		Topics:              topics,
		PendingTasks:        pending,
		RecentDailyMentions: mentions,
	}

	return profile, nil
}

func (s *PreflightService) findPendingTasks(name string, slug string, squad string) []string {
	var pending []string
	rep, err := backlog.GetActiveBacklog(s.vaultRoot)
	if err != nil || rep == nil {
		return pending
	}

	parts := strings.Fields(name)
	firstName := ""
	if len(parts) > 0 {
		firstName = strings.ToLower(parts[0])
	}
	cleanSlug := strings.ToLower(slug)
	squadLower := strings.ToLower(squad)

	for _, item := range rep.Items {
		if item.Completed {
			continue
		}
		fullText := strings.ToLower(item.Title + " " + item.Description + " " + item.Raw)
		if (firstName != "" && len(firstName) > 3 && strings.Contains(fullText, firstName)) ||
			strings.Contains(fullText, cleanSlug) ||
			(squadLower != "" && len(squadLower) > 4 && strings.Contains(fullText, squadLower)) {
			pending = append(pending, item.Title)
		}
	}

	return pending
}

func (s *PreflightService) scanRecentDailyMentions(name string, slug string, days int) []string {
	var mentions []string
	dailyDir := filepath.Join(s.vaultRoot, "daily")
	if _, err := os.Stat(dailyDir); os.IsNotExist(err) {
		return mentions
	}

	now := time.Now()
	for i := 0; i < days; i++ {
		dStr := now.AddDate(0, 0, -i).Format("2006-01-02")
		dFile := filepath.Join(dailyDir, dStr+".md")
		data, err := os.ReadFile(dFile) //nolint:gosec // intentional
		if err != nil {
			continue
		}

		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			lineLower := strings.ToLower(line)
			if strings.Contains(lineLower, slug) ||
				strings.Contains(lineLower, fmt.Sprintf("[[%s]]", slug)) ||
				(len(name) > 3 && strings.Contains(line, name)) {
				cleanLine := regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`).ReplaceAllString(line, "$1")
				cleanLine = strings.Trim(cleanLine, "- * \r\n") //nolint:staticcheck
				if len(cleanLine) > 15 {
					mentions = append(mentions, fmt.Sprintf("[%s] %s", dStr, cleanLine))
					if len(mentions) >= 5 {
						return mentions
					}
				}
			}
		}
	}

	return mentions
}

func isSubsequenceOrSubset(needles, haystack []string) bool {
	if len(needles) == 0 {
		return false
	}
	hayMap := make(map[string]bool, len(haystack))
	for _, h := range haystack {
		hayMap[h] = true
	}
	for _, n := range needles {
		if !hayMap[n] {
			return false
		}
	}
	return true
}
