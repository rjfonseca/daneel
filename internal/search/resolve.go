//nolint:revive // package comment not required
package search

import (
	"context"
	"math"
	"path/filepath"
	"strings"

	"github.com/rjfonseca/daneel/internal/vault"
)

// ResolveMatch represents the best matching note from a fuzzy resolution query.
type ResolveMatch struct {
	RelPath string  `json:"rel_path"`
	AbsPath string  `json:"abs_path"`
	Title   string  `json:"title"`
	Type    string  `json:"type"`
	Score   float64 `json:"score"`
}

// ResolveResult represents the overall outcome of a resolve operation.
type ResolveResult struct {
	Query string        `json:"query"`
	Found bool          `json:"found"`
	Match *ResolveMatch `json:"match"`
}

// candidateNote stores lightweight metadata loaded for resolution.
type candidateNote struct {
	relPath  string
	title    string
	noteType string
	aliases  string
}

// ResolveNote resolves any note title, slug, or alias to its canonical vault path in < 2ms.
func (e *Engine) ResolveNote(ctx context.Context, query string) (*ResolveResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	trimmedQuery := strings.TrimSpace(query)
	result := &ResolveResult{
		Query: trimmedQuery,
		Found: false,
		Match: nil,
	}

	if trimmedQuery == "" {
		return result, nil
	}

	// 1. Direct match fast-path: exact path or exact slug match in DB
	exactRows, err := e.db.QueryContext(ctx, `
		SELECT rel_path, title, note_type, COALESCE(aliases, '')
		FROM notes_meta
		WHERE rel_path = ? OR rel_path = ? OR rel_path = ?
		LIMIT 1
	`, trimmedQuery, trimmedQuery+".md", "notes/"+trimmedQuery+".md")
	if err == nil {
		if exactRows.Next() {
			var c candidateNote
			if err := exactRows.Scan(&c.relPath, &c.title, &c.noteType, &c.aliases); err == nil {
				_ = exactRows.Close()
				result.Found = true
				result.Match = &ResolveMatch{
					RelPath: c.relPath,
					AbsPath: filepath.Join(e.vaultRoot, c.relPath),
					Title:   c.title,
					Type:    c.noteType,
					Score:   1.0,
				}
				return result, nil
			}
		}
		_ = exactRows.Close()
	}

	// 2. Fetch candidates from notes_meta
	rows, err := e.db.QueryContext(ctx, `
		SELECT rel_path, title, note_type, COALESCE(aliases, '')
		FROM notes_meta
	`)
	if err != nil {
		return result, err
	}
	defer rows.Close() //nolint:errcheck,gosec

	normQuery := vault.NormalizeText(trimmedQuery)
	normQueryClean := strings.ReplaceAll(normQuery, "-", " ")
	queryTokens := strings.Fields(normQueryClean)
	kebabQuery := vault.ToKebabCase(trimmedQuery)

	var bestMatch *candidateNote
	bestScore := 0.0

	for rows.Next() {
		var c candidateNote
		if err := rows.Scan(&c.relPath, &c.title, &c.noteType, &c.aliases); err != nil {
			continue
		}

		score := scoreNoteCandidate(trimmedQuery, normQuery, normQueryClean, kebabQuery, queryTokens, c)
		if score > bestScore {
			bestScore = score
			candCopy := c
			bestMatch = &candCopy
		}
	}

	if bestScore >= 0.50 && bestMatch != nil {
		result.Found = true
		roundedScore := math.Round(bestScore*100) / 100
		result.Match = &ResolveMatch{
			RelPath: bestMatch.relPath,
			AbsPath: filepath.Join(e.vaultRoot, bestMatch.relPath),
			Title:   bestMatch.title,
			Type:    bestMatch.noteType,
			Score:   roundedScore,
		}
	}

	return result, nil
}

func scoreNoteCandidate(rawQuery, normQuery, normQueryClean, kebabQuery string, queryTokens []string, c candidateNote) float64 {
	cleanRelPath := filepath.ToSlash(c.relPath)
	base := filepath.Base(cleanRelPath)
	ext := filepath.Ext(base)
	slug := strings.TrimSuffix(base, ext)
	normSlug := vault.NormalizeText(slug)
	slugTokens := strings.Split(normSlug, "-")

	normTitle := vault.NormalizeText(c.title)
	normTitleClean := strings.ReplaceAll(normTitle, "-", " ")
	titleTokens := strings.Fields(normTitleClean)

	// 1. Exact path or slug match
	if cleanRelPath == rawQuery || cleanRelPath == rawQuery+".md" {
		return 1.0
	}
	if normSlug == kebabQuery || normSlug == normQuery {
		return 0.99
	}
	if normTitleClean == normQueryClean {
		return 0.98
	}

	// 2. Check aliases
	var aliasList []string
	if c.aliases != "" {
		for _, a := range strings.Split(c.aliases, "|") {
			aTrim := strings.TrimSpace(a)
			if aTrim != "" {
				aliasList = append(aliasList, aTrim)
				normA := strings.ReplaceAll(vault.NormalizeText(aTrim), "-", " ")
				if normA == normQueryClean {
					return 0.97
				}
			}
		}
	}

	// 3. Token containment in base filename / slug
	if len(queryTokens) > 0 && isSubsequenceOrSubset(queryTokens, slugTokens) {
		ratio := float64(len(queryTokens)) / float64(len(slugTokens))
		if ratio > 1.0 {
			ratio = 1.0 / ratio
		}
		return 0.90 + 0.05*ratio
	}

	// 4. Token containment in title
	if len(queryTokens) > 0 && isSubsequenceOrSubset(queryTokens, titleTokens) {
		ratio := float64(len(queryTokens)) / float64(len(titleTokens))
		if ratio > 1.0 {
			ratio = 1.0 / ratio
		}
		return 0.88 + 0.07*ratio
	}

	// 5. Token containment in any alias
	for _, alias := range aliasList {
		normAlias := strings.ReplaceAll(vault.NormalizeText(alias), "-", " ")
		aliasTokens := strings.Fields(normAlias)
		if len(queryTokens) > 0 && isSubsequenceOrSubset(queryTokens, aliasTokens) {
			return 0.86
		}
	}

	// 6. Substring match
	if strings.Contains(normSlug, normQuery) || strings.Contains(normSlug, kebabQuery) {
		return 0.80
	}
	if strings.Contains(normTitleClean, normQueryClean) {
		return 0.78
	}
	for _, alias := range aliasList {
		normAlias := strings.ReplaceAll(vault.NormalizeText(alias), "-", " ")
		if strings.Contains(normAlias, normQueryClean) {
			return 0.76
		}
	}

	// 7. Fuzzy similarity
	slugSim := stringSimilarity(normQuery, normSlug)
	if slugSim >= 0.70 {
		return slugSim * 0.75
	}
	titleSim := stringSimilarity(normQueryClean, normTitleClean)
	if titleSim >= 0.70 {
		return titleSim * 0.72
	}

	return 0.0
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

func levenshteinDistance(s1, s2 string) int {
	r1, r2 := []rune(s1), []rune(s2)
	l1, l2 := len(r1), len(r2)
	if l1 == 0 {
		return l2
	}
	if l2 == 0 {
		return l1
	}
	v0 := make([]int, l2+1)
	v1 := make([]int, l2+1)
	for i := 0; i <= l2; i++ {
		v0[i] = i
	}
	for i := 0; i < l1; i++ {
		v1[0] = i + 1
		for j := 0; j < l2; j++ {
			cost := 0
			if r1[i] != r2[j] {
				cost = 1
			}
			min := v0[j+1] + 1
			if v1[j]+1 < min {
				min = v1[j] + 1
			}
			if v0[j]+cost < min {
				min = v0[j] + cost
			}
			v1[j+1] = min
		}
		copy(v0, v1)
	}
	return v1[l2]
}

func stringSimilarity(s1, s2 string) float64 {
	if s1 == s2 {
		return 1.0
	}
	r1, r2 := []rune(s1), []rune(s2)
	maxLen := len(r1)
	if len(r2) > maxLen {
		maxLen = len(r2)
	}
	if maxLen == 0 {
		return 1.0
	}
	dist := levenshteinDistance(s1, s2)
	return 1.0 - float64(dist)/float64(maxLen)
}
