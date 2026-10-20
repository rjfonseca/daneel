//nolint:revive // package comment not required
package vault

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
	"gopkg.in/yaml.v3"
)

// Link represents an outgoing link from a note.
type Link struct {
	SourceRelPath string `json:"source_rel_path"`
	TargetRelPath string `json:"target_rel_path"`
	LinkText      string `json:"link_text"`
	IsWikiLink    bool   `json:"is_wikilink"`
}

// Frontmatter represents standard OKF frontmatter metadata.
type Frontmatter struct {
	Type        string                 `yaml:"type"`
	Description string                 `yaml:"description"`
	Tags        interface{}            `yaml:"tags"`
	Aliases     interface{}            `yaml:"aliases"`
	Extra       map[string]interface{} `yaml:",inline"`
}

// Note represents an ingested markdown note in the vault.
type Note struct {
	RelPath     string      `json:"rel_path"`
	AbsPath     string      `json:"abs_path"`
	Title       string      `json:"title"`
	Aliases     []string    `json:"aliases,omitempty"`
	Category    string      `json:"category"`
	Domain      string      `json:"domain"`
	Group       string      `json:"group"`
	Repository  string      `json:"repository"`
	Scope       string      `json:"scope"`
	NoteType    string      `json:"note_type"`
	Description string      `json:"description"`
	Tags        []string    `json:"tags"`
	Headings    []string    `json:"headings"`
	Content     string      `json:"content"`
	Body        string      `json:"body"`
	MTime       time.Time   `json:"mtime"`
	Links       []Link      `json:"links"`
	FM          Frontmatter `json:"frontmatter"`
}

var (
	frontmatterRegex = regexp.MustCompile(`(?s)^---\s*\n(.*?)\n---\s*\n`)
	headingRegex     = regexp.MustCompile(`(?m)^#+\s+(.*)$`)
	h1Regex          = regexp.MustCompile(`(?m)^#\s+(.*)$`)
	mdLinkRegex      = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	wikiLinkRegex    = regexp.MustCompile(`\[\[([^\]|]+)(?:\|([^\]]+))?\]\]`)
	fencedCodeRegex  = regexp.MustCompile("(?s)```.*?```|~~~.*?~~~")
	inlineCodeRegex  = regexp.MustCompile("`[^`\n]+`")
)

// NormalizeText strips accents, converts to lowercase ascii.
func NormalizeText(s string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	result, _, _ := transform.String(t, s)
	return strings.ToLower(result)
}

// ToKebabCase converts a string to a clean kebab-case ascii identifier.
func ToKebabCase(s string) string {
	normStr := NormalizeText(s)
	reg := regexp.MustCompile(`[^a-z0-9]+`)
	clean := reg.ReplaceAllString(normStr, "-")
	return strings.Trim(clean, "-")
}

// DetermineScope categorizes a relative file path into an index scope.
func DetermineScope(relPath string) string {
	cleanPath := filepath.ToSlash(relPath)
	if strings.HasPrefix(cleanPath, "public/content") || strings.HasPrefix(cleanPath, "public/") {
		return "public"
	}
	parts := strings.Split(cleanPath, "/")
	if len(parts) > 0 {
		switch parts[0] {
		case "notes":
			return "tech"
		case "daily":
			return "daily"
		case "meta":
			return "meta"
		case "inbox":
			return "inbox"
		case "sources":
			return "sources"
		}
	}
	return "other"
}

var repoInternalDirs = map[string]bool{
	"docs": true, "doc": true, "documentation": true,
	"specs": true, "spec": true, "plans": true, "conductor": true,
	"runbooks": true, "runbook": true, "adrs": true, "adr": true,
	"tracks": true, "archive": true, "superpowers": true,
	"vendor": true, "node_modules": true, "dist": true, "build": true,
}

var knownSectorSubgroups = map[string]bool{
	"saas": true, "apis": true, "stores": true,
	"platform": true, "infra": true, "core": true,
	"services": true, "tools": true, "apps": true,
	"mobile": true, "web": true, "shared": true,
	"data": true, "analytics": true, "edge": true,
}

// NoteTaxonomy captures the 4-tier cosmic hierarchy for any vault document.
type NoteTaxonomy struct {
	Domain     string `json:"domain"`     // Knowledge Domain: e.g. "sources/gitlab", "notes/arquitetura", "daily"
	Group      string `json:"group"`      // Organizational group/subgroup: e.g. "acme/saas/pocs"
	Repository string `json:"repository"` // Repository name: e.g. "my-repo"
	Category   string `json:"category"`   // Canonical category path: e.g. "sources/gitlab/acme/saas/pocs/my-repo"
}

// ResolveTaxonomy resolves the domain, group, repository, and category for a relative note path.
// It prioritizes explicit OKF index.md MOCs on disk, falling back to deterministic path heuristics.
func ResolveTaxonomy(relPath string, vaultRoot string) NoteTaxonomy {
	cleanPath := filepath.ToSlash(filepath.Clean(relPath))
	if cleanPath == "." || cleanPath == "" || cleanPath == "README.md" || !strings.Contains(cleanPath, "/") {
		return NoteTaxonomy{
			Domain:     "root",
			Group:      "",
			Repository: "",
			Category:   "root",
		}
	}

	parts := strings.Split(cleanPath, "/")
	top := parts[0]

	switch top {
	case "notes":
		domain := "notes"
		if len(parts) >= 2 {
			domain = "notes/" + parts[1]
		}
		return NoteTaxonomy{
			Domain:     domain,
			Group:      "",
			Repository: "",
			Category:   domain,
		}
	case "daily", "inbox", "meta", "public":
		return NoteTaxonomy{
			Domain:     top,
			Group:      "",
			Repository: "",
			Category:   top,
		}
	case "sources":
		if len(parts) <= 2 {
			dir := filepath.ToSlash(filepath.Dir(cleanPath))
			return NoteTaxonomy{
				Domain:     "sources",
				Group:      "",
				Repository: "",
				Category:   dir,
			}
		}

		provider := parts[1]
		domain := "sources/" + provider

		// 1. Ancestor index.md MOC inspection (deterministic OKF resolution)
		if vaultRoot != "" && (provider == "gitlab" || provider == "github") {
			curr := filepath.ToSlash(filepath.Dir(cleanPath))
			if strings.EqualFold(filepath.Base(cleanPath), "index.md") {
				curr = filepath.ToSlash(filepath.Dir(cleanPath))
			}

			for strings.HasPrefix(curr, domain) && curr != domain {
				indexFile := filepath.Join(vaultRoot, filepath.FromSlash(curr), "index.md")
				if data, err := os.ReadFile(indexFile); err == nil {
					content := string(data)
					if strings.Contains(content, "type: repository") {
						repoName := filepath.Base(curr)
						parentDir := filepath.ToSlash(filepath.Dir(curr))
						groupRel := strings.TrimPrefix(strings.TrimPrefix(parentDir, domain), "/")
						return NoteTaxonomy{
							Domain:     domain,
							Group:      groupRel,
							Repository: repoName,
							Category:   curr,
						}
					}
					if strings.Contains(content, "type: group") && cleanPath == curr+"/index.md" {
						groupRel := strings.TrimPrefix(strings.TrimPrefix(curr, domain), "/")
						return NoteTaxonomy{
							Domain:     domain,
							Group:      groupRel,
							Repository: "",
							Category:   curr,
						}
					}
				}
				next := filepath.ToSlash(filepath.Dir(curr))
				if next == curr {
					break
				}
				curr = next
			}
		}

		// 2. Fallback heuristics when index.md is absent or vaultRoot == ""
		if provider == "github" || provider == "github.com" {
			if len(parts) >= 4 {
				return NoteTaxonomy{
					Domain:     domain,
					Group:      parts[2],
					Repository: parts[3],
					Category:   strings.Join(parts[:4], "/"),
				}
			}
			return NoteTaxonomy{
				Domain:     domain,
				Group:      parts[2],
				Repository: "",
				Category:   strings.Join(parts[:3], "/"),
			}
		}

		if provider == "gitlab" || provider == "gitlab.com" {
			// A. Check for known internal documentation/specification folders
			for i := 3; i < len(parts); i++ {
				if repoInternalDirs[strings.ToLower(parts[i])] {
					repoDir := strings.Join(parts[:i], "/")
					repoParts := strings.Split(repoDir, "/")
					repoName := repoParts[len(repoParts)-1]
					groupParts := repoParts[2 : len(repoParts)-1]
					return NoteTaxonomy{
						Domain:     domain,
						Group:      strings.Join(groupParts, "/"),
						Repository: repoName,
						Category:   repoDir,
					}
				}
			}

			// B. Single or multi-level group matching with known sector subgroups
			if len(parts) >= 4 && knownSectorSubgroups[strings.ToLower(parts[2])] {
				repoDir := strings.Join(parts[:4], "/")
				repoParts := strings.Split(repoDir, "/")
				repoName := repoParts[len(repoParts)-1]
				groupParts := repoParts[2 : len(repoParts)-1]
				return NoteTaxonomy{
					Domain:     domain,
					Group:      strings.Join(groupParts, "/"),
					Repository: repoName,
					Category:   repoDir,
				}
			}
			if len(parts) >= 5 && knownSectorSubgroups[strings.ToLower(parts[3])] {
				repoDir := strings.Join(parts[:5], "/")
				repoParts := strings.Split(repoDir, "/")
				repoName := repoParts[len(repoParts)-1]
				groupParts := repoParts[2 : len(repoParts)-1]
				return NoteTaxonomy{
					Domain:     domain,
					Group:      strings.Join(groupParts, "/"),
					Repository: repoName,
					Category:   repoDir,
				}
			}

			// C. Direct file in repo
			dir := filepath.ToSlash(filepath.Dir(cleanPath))
			dirParts := strings.Split(dir, "/")
			if len(dirParts) >= 3 {
				repoName := dirParts[len(dirParts)-1]
				groupParts := dirParts[2 : len(dirParts)-1]
				return NoteTaxonomy{
					Domain:     domain,
					Group:      strings.Join(groupParts, "/"),
					Repository: repoName,
					Category:   dir,
				}
			}

			return NoteTaxonomy{
				Domain:     domain,
				Group:      "",
				Repository: "",
				Category:   dir,
			}
		}

		// Other sources (e.g. sources/jira/post-mortems, sources/reunioes)
		dir := filepath.ToSlash(filepath.Dir(cleanPath))
		dirParts := strings.Split(dir, "/")
		if len(dirParts) >= 3 {
			return NoteTaxonomy{
				Domain:     domain,
				Group:      "",
				Repository: "",
				Category:   strings.Join(dirParts[:3], "/"),
			}
		}
		return NoteTaxonomy{
			Domain:     domain,
			Group:      "",
			Repository: "",
			Category:   dir,
		}
	}

	dir := filepath.ToSlash(filepath.Dir(cleanPath))
	return NoteTaxonomy{
		Domain:     "other",
		Group:      "",
		Repository: "",
		Category:   dir,
	}
}

// DetermineCategory resolves the canonical project or section category for a relative note path.
func DetermineCategory(relPath string) string {
	return ResolveTaxonomy(relPath, "").Category
}

// ParseNote parses a markdown file at absPath into a Note struct.
func ParseNote(absPath string, vaultRoot string) (*Note, error) {
	stat, err := os.Stat(absPath)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(absPath) //nolint:gosec // intentional
	if err != nil {
		return nil, err
	}

	content := string(data)
	relPath, err := filepath.Rel(vaultRoot, absPath)
	if err != nil {
		relPath = absPath
	}
	relPath = filepath.ToSlash(relPath)

	tax := ResolveTaxonomy(relPath, vaultRoot)
	category := tax.Category
	scope := DetermineScope(relPath)

	// Extract frontmatter
	fm := Frontmatter{
		Type: "nota",
	}
	body := content

	fmMatch := frontmatterRegex.FindStringSubmatch(content)
	if len(fmMatch) > 1 {
		yamlContent := fmMatch[1]
		_ = yaml.Unmarshal([]byte(yamlContent), &fm)
		body = content[len(fmMatch[0]):]
	}

	// Format tags
	var tags []string
	switch v := fm.Tags.(type) {
	case []interface{}:
		for _, t := range v {
			if str, ok := t.(string); ok {
				tags = append(tags, strings.TrimSpace(str))
			}
		}
	case string:
		for _, t := range strings.Split(v, ",") {
			trimmed := strings.TrimSpace(t)
			if trimmed != "" {
				tags = append(tags, trimmed)
			}
		}
	}

	// Format aliases
	var aliases []string
	extractStrings := func(val interface{}) []string {
		var list []string
		switch v := val.(type) {
		case []interface{}:
			for _, t := range v {
				if str, ok := t.(string); ok {
					s := strings.TrimSpace(str)
					if s != "" {
						list = append(list, s)
					}
				}
			}
		case []string:
			for _, s := range v {
				s = strings.TrimSpace(s)
				if s != "" {
					list = append(list, s)
				}
			}
		case string:
			for _, t := range strings.Split(v, ",") {
				s := strings.TrimSpace(t)
				if s != "" {
					list = append(list, s)
				}
			}
		}
		return list
	}

	aliases = extractStrings(fm.Aliases)
	if len(aliases) == 0 && fm.Extra != nil {
		aliases = extractStrings(fm.Extra["aliases"])
	}

	// Extract Title
	title := ""
	if h1Match := h1Regex.FindStringSubmatch(content); len(h1Match) > 1 {
		title = strings.TrimSpace(h1Match[1])
	}
	if title == "" {
		base := filepath.Base(relPath)
		ext := filepath.Ext(base)
		nameWithoutExt := strings.TrimSuffix(base, ext)
		title = strings.Title(strings.ReplaceAll(nameWithoutExt, "-", " ")) //nolint:staticcheck
	}

	// Extract Headings
	var headings []string
	for _, m := range headingRegex.FindAllStringSubmatch(content, -1) {
		if len(m) > 1 {
			headings = append(headings, strings.TrimSpace(m[1]))
		}
	}

	// Extract Links
	links := ExtractLinks(content, relPath, vaultRoot)

	return &Note{
		RelPath:     relPath,
		AbsPath:     absPath,
		Title:       title,
		Aliases:     aliases,
		Category:    category,
		Domain:      tax.Domain,
		Group:       tax.Group,
		Repository:  tax.Repository,
		Scope:       scope,
		NoteType:    fm.Type,
		Description: fm.Description,
		Tags:        tags,
		Headings:    headings,
		Content:     content,
		Body:        body,
		MTime:       stat.ModTime(),
		Links:       links,
		FM:          fm,
	}, nil
}

// ExtractLinks extracts relative markdown links and wikilinks from content.
func ExtractLinks(content string, sourceRelPath string, vaultRoot string) []Link {
	var links []Link
	sourceDirRel := filepath.Dir(sourceRelPath)

	// Strip code blocks and inline code to avoid false positives in code snippets/examples
	cleanContent := fencedCodeRegex.ReplaceAllString(content, "")
	cleanContent = inlineCodeRegex.ReplaceAllString(cleanContent, "")

	// 1. Markdown links [Text](path.md)
	for _, match := range mdLinkRegex.FindAllStringSubmatch(cleanContent, -1) {
		if len(match) < 3 {
			continue
		}
		text := match[1]
		target := match[2]

		if strings.HasPrefix(target, "http://") ||
			strings.HasPrefix(target, "https://") ||
			strings.HasPrefix(target, "#") ||
			strings.HasPrefix(target, "mailto:") {
			continue
		}

		cleanTarget := strings.Split(target, "#")[0]
		if cleanTarget == "" {
			continue
		}

		targetRel := filepath.Clean(filepath.Join(sourceDirRel, cleanTarget))
		targetRel = filepath.ToSlash(targetRel)

		links = append(links, Link{
			SourceRelPath: sourceRelPath,
			TargetRelPath: targetRel,
			LinkText:      text,
			IsWikiLink:    false,
		})
	}

	// 2. Obsidian WikiLinks [[Target|Text]]
	for _, match := range wikiLinkRegex.FindAllStringSubmatch(cleanContent, -1) {
		if len(match) < 2 {
			continue
		}
		targetName := strings.TrimSpace(match[1])
		label := targetName
		if len(match) > 2 && match[2] != "" {
			label = strings.TrimSpace(match[2])
		}

		links = append(links, Link{
			SourceRelPath: sourceRelPath,
			TargetRelPath: targetName,
			LinkText:      label,
			IsWikiLink:    true,
		})
	}

	return links
}
