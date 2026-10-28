//nolint:revive // package comment not required
package dream

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/rjfonseca/daneel/internal/safefile"
)

// InboxTriageItem describes a triaged draft in inbox/.
type InboxTriageItem struct {
	File              string `json:"file"`
	AbsPath           string `json:"abs_path"`
	LineCount         int    `json:"line_count"`
	Confidence        string `json:"confidence"`
	Score             int    `json:"score"`
	SuggestedCategory string `json:"suggested_category"`
	Snippet           string `json:"snippet"`
}

// EmergingConcept represents a latent concept found in daily notes.
type EmergingConcept struct {
	Title     string `json:"title"`
	SourceURL string `json:"source_url,omitempty"`
	Context   string `json:"context,omitempty"`
}

// DenseCategory reports category density in notes/.
type DenseCategory struct {
	Category   string `json:"category"`
	Count      int    `json:"count"`
	Suggestion string `json:"suggestion"`
}

// TriageInboxItems scans inbox/ and assigns promotion scores.
func TriageInboxItems(vaultRoot string, learnings *AgentLearnings) []InboxTriageItem {
	inboxDir := filepath.Join(vaultRoot, "inbox")
	var triaged []InboxTriageItem

	if _, err := os.Stat(inboxDir); os.IsNotExist(err) {
		return triaged
	}

	entries, err := os.ReadDir(inboxDir)
	if err != nil {
		return triaged
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		absPath := filepath.Join(inboxDir, entry.Name())
		data, err := os.ReadFile(absPath) //nolint:gosec // intentional
		if err != nil {
			continue
		}

		content := string(data)
		lines := strings.Split(content, "\n")
		lineCount := len(lines)

		score := 50
		suggestedCategory := "notes/engenharia"

		// Apply learned keyword heuristics
		lower := strings.ToLower(content)
		for kw, cat := range learnings.CategoryPreferences {
			if strings.Contains(lower, strings.ToLower(kw)) {
				suggestedCategory = cat
				score += 20
				break
			}
		}

		if strings.Contains(content, "# ") {
			score += 15
		}
		if strings.Contains(content, "```") {
			score += 15
		}
		if strings.Contains(content, "http://") || strings.Contains(content, "https://") {
			score += 10
		}

		if lineCount >= 20 {
			score += 15
		} else if lineCount <= 2 {
			score = 25
			suggestedCategory = "inbox"
		}

		if score > 95 {
			score = 95
		}

		confidence := "LOW"
		if score >= 80 {
			confidence = "HIGH"
		} else if score >= 50 {
			confidence = "MEDIUM"
		}

		snippet := ""
		for _, l := range lines {
			lTrim := strings.TrimSpace(l)
			if lTrim != "" && !strings.HasPrefix(lTrim, "#") {
				snippet = lTrim
				break
			}
		}

		triaged = append(triaged, InboxTriageItem{
			File:              entry.Name(),
			AbsPath:           absPath,
			LineCount:         lineCount,
			Confidence:        confidence,
			Score:             score,
			SuggestedCategory: suggestedCategory,
			Snippet:           snippet,
		})
	}

	sort.Slice(triaged, func(i, j int) bool {
		return triaged[i].Score > triaged[j].Score
	})

	return triaged
}

// DiscoverEmergingConcepts parses daily notes for latent concepts.
func DiscoverEmergingConcepts(vaultRoot string, targetDate string) []EmergingConcept {
	dailyFile := filepath.Join(vaultRoot, "daily", targetDate+".md")
	var concepts []EmergingConcept

	data, err := os.ReadFile(dailyFile) //nolint:gosec // intentional
	if err != nil {
		return concepts
	}

	lines := strings.Split(string(data), "\n")
	inConhecimento := false
	urlRegex := regexp.MustCompile(`\[([^\]]+)\]\((https?://[^\)]+)\)`)

	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if strings.HasPrefix(trimmed, "##") {
			if strings.Contains(strings.ToLower(trimmed), "conhecimento") {
				inConhecimento = true
				continue
			} else { //nolint:revive
				inConhecimento = false
			}
		}

		if inConhecimento && (strings.HasPrefix(trimmed, "-") || strings.HasPrefix(trimmed, "*")) {
			clean := strings.Trim(trimmed, "- *\r\t")
			urlMatch := urlRegex.FindStringSubmatch(clean)

			title := clean
			sourceURL := ""
			if len(urlMatch) > 2 {
				title = urlMatch[1]
				sourceURL = urlMatch[2]
			}

			if len(title) > 3 {
				concepts = append(concepts, EmergingConcept{
					Title:     title,
					SourceURL: sourceURL,
					Context:   clean,
				})
			}
		}
	}

	return concepts
}

// SyncMOCs ensures all notes in notes/<cat>/*.md are referenced in notes/<cat>/moc-<cat>.md.
func SyncMOCs(vaultRoot string, autoFix bool) ([]string, error) {
	notesDir := filepath.Join(vaultRoot, "notes")
	var missingNotes []string

	if _, err := os.Stat(notesDir); os.IsNotExist(err) {
		return missingNotes, nil
	}

	cats, err := os.ReadDir(notesDir)
	if err != nil {
		return nil, err
	}

	for _, cat := range cats {
		if !cat.IsDir() {
			continue
		}

		catDir := filepath.Join(notesDir, cat.Name())
		files, err := os.ReadDir(catDir)
		if err != nil {
			continue
		}

		var noteFiles []os.DirEntry
		for _, f := range files {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".md") || strings.HasPrefix(f.Name(), "moc-") || f.Name() == "README.md" {
				continue
			}
			noteFiles = append(noteFiles, f)
		}

		if len(noteFiles) == 0 {
			continue
		}

		mocFile := filepath.Join(catDir, fmt.Sprintf("moc-%s.md", cat.Name()))
		mocRelPath := fmt.Sprintf("notes/%s/moc-%s.md", cat.Name(), cat.Name())

		if _, err := os.Stat(mocFile); os.IsNotExist(err) {
			missingNotes = append(missingNotes, mocRelPath)
			if autoFix {
				catTitle := cat.Name()
				if len(catTitle) > 0 {
					catTitle = strings.ToUpper(catTitle[:1]) + catTitle[1:]
				}
				var sb strings.Builder
				fmt.Fprintf(&sb, "---\ntype: moc\ntags:\n  - notes/moc\n---\n# 🗺️ MOC: %s\n\n> **Master Index:** [Master MOC](../moc-indice-geral.md)\n\n## 📚 Indexed Notes\n", catTitle)

				for _, f := range noteFiles {
					noteData, err := os.ReadFile(filepath.Join(catDir, f.Name())) //nolint:gosec // intentional
					title := strings.TrimSuffix(f.Name(), ".md")
					desc := "Technical note."
					if err == nil {
						nc := string(noteData)
						if tm := regexp.MustCompile(`(?m)^#\s+(.*?)$`).FindStringSubmatch(nc); len(tm) > 1 {
							title = strings.TrimSpace(tm[1])
						}
						if dm := regexp.MustCompile(`(?m)^description:\s*["']?(.*?)["']?$`).FindStringSubmatch(nc); len(dm) > 1 {
							desc = strings.TrimSpace(dm[1])
						}
					}
					fmt.Fprintf(&sb, "\n- [%s](%s) — %s\n", title, f.Name(), desc)
				}

				if err := safefile.WriteFile(vaultRoot, mocFile, []byte(sb.String()), 0600); err != nil {
					return missingNotes, err
				}
			}
			continue
		}

		mocData, err := os.ReadFile(mocFile) //nolint:gosec // intentional
		if err != nil {
			continue
		}
		mocContent := string(mocData)

		for _, f := range noteFiles {
			fNameNoExt := strings.TrimSuffix(f.Name(), ".md")
			if !strings.Contains(mocContent, f.Name()) && !strings.Contains(mocContent, fNameNoExt) {
				rel := fmt.Sprintf("notes/%s/%s", cat.Name(), f.Name())
				missingNotes = append(missingNotes, rel)

				if autoFix {
					noteData, err := os.ReadFile(filepath.Join(catDir, f.Name())) //nolint:gosec // intentional
					if err == nil {
						nc := string(noteData)
						title := fNameNoExt
						if tm := regexp.MustCompile(`(?m)^#\s+(.*?)$`).FindStringSubmatch(nc); len(tm) > 1 {
							title = strings.TrimSpace(tm[1])
						}

						desc := "Technical note."
						if dm := regexp.MustCompile(`(?m)^description:\s*["']?(.*?)["']?$`).FindStringSubmatch(nc); len(dm) > 1 {
							desc = strings.TrimSpace(dm[1])
						}

						appendEntry := fmt.Sprintf("\n- [%s](%s) — %s\n", title, f.Name(), desc)
						mocData = append(mocData, []byte(appendEntry)...)
						mocContent = string(mocData)
						if err := safefile.WriteFile(vaultRoot, mocFile, mocData, 0600); err != nil {
							return missingNotes, err
						}
					}
				}
			}
		}
	}

	return missingNotes, nil
}

// AuditGraphDensity checks note count per category and suggests sub-MOCs.
func AuditGraphDensity(vaultRoot string) ([]DenseCategory, error) {
	notesDir := filepath.Join(vaultRoot, "notes")
	var dense []DenseCategory

	if _, err := os.Stat(notesDir); os.IsNotExist(err) {
		return dense, nil
	}

	cats, err := os.ReadDir(notesDir)
	if err != nil {
		return nil, err
	}

	for _, cat := range cats {
		if !cat.IsDir() {
			continue
		}

		catDir := filepath.Join(notesDir, cat.Name())
		files, err := os.ReadDir(catDir)
		if err != nil {
			continue
		}

		count := 0
		for _, f := range files {
			if !f.IsDir() && strings.HasSuffix(f.Name(), ".md") && !strings.HasPrefix(f.Name(), "moc-") {
				count++
			}
		}

		if count >= 25 {
			dense = append(dense, DenseCategory{
				Category:   cat.Name(),
				Count:      count,
				Suggestion: fmt.Sprintf("A categoria `notes/%s/` possui %d notas. Considere criar sub-MOCs ou tópicos temáticos específicos.", cat.Name(), count),
			})
		}
	}

	return dense, nil
}
