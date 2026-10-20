//nolint:revive // package comment not required
package search

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rjfonseca/daneel/internal/vault"
)

// PersonResult represents a discovered colleague or leadership profile.
type PersonResult struct {
	Name    string   `json:"name"`
	Role    string   `json:"role"`
	Squad   string   `json:"squad"`
	Email   string   `json:"email,omitempty"`
	RelPath string   `json:"rel_path"`
	AbsPath string   `json:"abs_path,omitempty"`
	Aliases []string `json:"aliases,omitempty"`
	Score   float64  `json:"score,omitempty"`
}

type personRecord struct {
	relPath string
	name    string
	role    string
	squad   string
	email   string
	aliases string
}

// SearchPeople queries person profiles with compound name matching, nickname resolution, and squad/role/email filters.
func (e *Engine) SearchPeople(ctx context.Context, query, squad, role, email string) ([]PersonResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	// 1. Ensure people_meta is hydrated
	e.ensurePeopleHydrated(ctx)

	// 2. Fetch all candidates from people_meta
	rows, err := e.db.QueryContext(ctx, `
		SELECT rel_path, name, role, squad, email, COALESCE(aliases, '')
		FROM people_meta
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck,gosec

	var records []personRecord
	for rows.Next() {
		var r personRecord
		if err := rows.Scan(&r.relPath, &r.name, &r.role, &r.squad, &r.email, &r.aliases); err == nil {
			records = append(records, r)
		}
	}

	normQuery := vault.NormalizeText(strings.TrimSpace(query))
	normQueryClean := strings.ReplaceAll(normQuery, "-", " ")
	qTokens := strings.Fields(normQueryClean)

	normSquad := vault.NormalizeText(strings.TrimSpace(squad))
	normRole := vault.NormalizeText(strings.TrimSpace(role))
	normEmail := strings.ToLower(strings.TrimSpace(email))

	var results []PersonResult

	for _, rec := range records {
		// Apply filters
		if normSquad != "" {
			if !strings.Contains(vault.NormalizeText(rec.squad), normSquad) {
				continue
			}
		}
		if normRole != "" {
			if !strings.Contains(vault.NormalizeText(rec.role), normRole) {
				continue
			}
		}
		if normEmail != "" {
			if !strings.Contains(strings.ToLower(rec.email), normEmail) {
				continue
			}
		}

		score := 1.0
		if normQuery != "" {
			score = scorePerson(normQuery, normQueryClean, qTokens, rec)
			if score < 0.50 {
				continue
			}
		}

		var aliases []string
		if rec.aliases != "" {
			for _, a := range strings.Split(rec.aliases, "|") {
				if trimmed := strings.TrimSpace(a); trimmed != "" {
					aliases = append(aliases, trimmed)
				}
			}
		}

		results = append(results, PersonResult{
			Name:    rec.name,
			Role:    rec.role,
			Squad:   rec.squad,
			Email:   rec.email,
			RelPath: rec.relPath,
			AbsPath: filepath.Join(e.vaultRoot, rec.relPath),
			Aliases: aliases,
			Score:   math.Round(score*100) / 100,
		})
	}

	// Sort by score DESC, then name ASC
	sort.Slice(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		return results[i].Name < results[j].Name
	})

	return results, nil
}

func scorePerson(normQuery, normQueryClean string, qTokens []string, rec personRecord) float64 {
	normName := vault.NormalizeText(rec.name)
	normNameClean := strings.ReplaceAll(normName, "-", " ")
	nameTokens := strings.Fields(normNameClean)

	slug := strings.TrimSuffix(filepath.Base(rec.relPath), ".md")
	normSlug := vault.NormalizeText(slug)
	slugTokens := strings.Split(normSlug, "-")

	var aliasList []string
	if rec.aliases != "" {
		for _, a := range strings.Split(rec.aliases, "|") {
			if trimmed := strings.TrimSpace(a); trimmed != "" {
				aliasList = append(aliasList, trimmed)
			}
		}
	}

	// 1. Exact match on name, slug, or alias
	if normNameClean == normQueryClean || normSlug == normQuery {
		return 1.0
	}
	for _, a := range aliasList {
		if strings.ReplaceAll(vault.NormalizeText(a), "-", " ") == normQueryClean {
			return 0.99
		}
	}

	// 2. Compound name token matching (e.g. "Ada Lovelace" in "Augusta Ada King Noel Byron Lovelace")
	if len(qTokens) > 0 && isSubsequenceOrSubset(qTokens, nameTokens) {
		ratio := float64(len(qTokens)) / float64(len(nameTokens))
		return 0.90 + 0.08*ratio
	}

	// 3. Slug token matching
	if len(qTokens) > 0 && isSubsequenceOrSubset(qTokens, slugTokens) {
		ratio := float64(len(qTokens)) / float64(len(slugTokens))
		return 0.88 + 0.07*ratio
	}

	// 4. Alias token matching (e.g. "Augusta Ada Byron King Lovelace (lovelace)")
	for _, a := range aliasList {
		aTokens := strings.Fields(strings.ReplaceAll(vault.NormalizeText(a), "-", " "))
		if len(qTokens) > 0 && isSubsequenceOrSubset(qTokens, aTokens) {
			return 0.86
		}
	}

	// 5. Email matching
	normRecEmail := strings.ToLower(rec.email)
	if strings.Contains(normRecEmail, normQuery) || strings.Contains(normRecEmail, strings.ReplaceAll(normQuery, " ", ".")) {
		return 0.85
	}

	// 6. Substring match
	if strings.Contains(normNameClean, normQueryClean) || strings.Contains(normSlug, normQuery) {
		return 0.80
	}
	for _, a := range aliasList {
		if strings.Contains(strings.ReplaceAll(vault.NormalizeText(a), "-", " "), normQueryClean) {
			return 0.78
		}
	}

	// 7. Fuzzy token matching (tolerance for minor typos, e.g. "Turin" vs "Turing")
	if len(qTokens) > 0 {
		allTokensFuzzyMatched := true
		for _, qt := range qTokens {
			matched := false
			for _, nt := range nameTokens {
				if levenshteinDistance(qt, nt) <= 1 || stringSimilarity(qt, nt) >= 0.82 {
					matched = true
					break
				}
			}
			if !matched {
				for _, st := range slugTokens {
					if levenshteinDistance(qt, st) <= 1 || stringSimilarity(qt, st) >= 0.82 {
						matched = true
						break
					}
				}
			}
			if !matched {
				allTokensFuzzyMatched = false
				break
			}
		}
		if allTokensFuzzyMatched {
			return 0.75
		}
	}

	return 0.0
}

func (e *Engine) ensurePeopleHydrated(ctx context.Context) {
	var count int
	_ = e.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM people_meta").Scan(&count)
	if count > 0 {
		return
	}

	// Hydrate from vault directories
	dirs := []string{
		filepath.Join(e.vaultRoot, "notes", "pessoas"),
		filepath.Join(e.vaultRoot, "meta", "pessoas"),
	}

	tx, err := e.db.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	defer tx.Rollback() //nolint:errcheck

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO people_meta(rel_path, name, role, squad, email, aliases, mtime)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(rel_path) DO UPDATE SET
			name=excluded.name,
			role=excluded.role,
			squad=excluded.squad,
			email=excluded.email,
			aliases=excluded.aliases,
			mtime=excluded.mtime;
	`)
	if err != nil {
		return
	}
	defer stmt.Close() //nolint:errcheck

	for _, dir := range dirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}
		_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil //nolint:nilerr // continue walking on inaccessible entry
			}
			if info.IsDir() || !strings.HasSuffix(strings.ToLower(info.Name()), ".md") {
				return nil
			}
			note, parseErr := vault.ParseNote(path, e.vaultRoot)
			if parseErr != nil || note == nil {
				return nil //nolint:nilerr // ignore unparseable note during directory walk
			}

			cleanName := cleanPersonName(note.Title)
			role := getExtraString(note.FM.Extra, "role", "cargo")
			squad := getExtraString(note.FM.Extra, "squad", "equipe", "time")
			email := getExtraString(note.FM.Extra, "email")
			aliasesStr := strings.Join(note.Aliases, " | ")
			mtimeUnix := float64(info.ModTime().UnixNano()) / 1e9

			_, _ = stmt.ExecContext(ctx, note.RelPath, cleanName, role, squad, email, aliasesStr, mtimeUnix)
			return nil
		})
	}

	_ = tx.Commit()
}
