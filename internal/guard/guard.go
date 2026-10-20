//nolint:revive // package comment not required
package guard

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// LawType categorizes violations under Isaac Asimov's Laws of Robotics.
type LawType string

const (
	ZerothLaw LawType = "ZEROTH_LAW_ZERO_LEAKAGE" //nolint:revive
	FirstLaw  LawType = "FIRST_LAW_PARTNER_PRIVACY"
	SecondLaw LawType = "SECOND_LAW_GRAPH_INTEGRITY"
	ThirdLaw  LawType = "THIRD_LAW_SCHEMA_STANDARDS"
)

// IssueType categorizes security and compliance violations.
type IssueType string

const (
	LeakLinkOutsidePublic IssueType = "LEAK_LINK_OUTSIDE_PUBLIC" //nolint:revive
	BrokenPublicLink      IssueType = "BROKEN_PUBLIC_LINK"
	UnconvertedWikiLinks  IssueType = "UNCONVERTED_WIKILINKS"
	InternalDataDetected  IssueType = "INTERNAL_DATA_DETECTED"
	MissingFrontmatter    IssueType = "MISSING_FRONTMATTER"
)

// Issue describes a specific violation found during validation.
type Issue struct {
	Law        LawType   `json:"law"`
	Type       IssueType `json:"type"`
	LineNumber int       `json:"line_number,omitempty"`
	Detail     string    `json:"detail"`
	Suggestion string    `json:"suggestion"`
}

// FileReport holds all issues found in a single file.
type FileReport struct {
	RelPath string  `json:"rel_path"`
	AbsPath string  `json:"abs_path"`
	Issues  []Issue `json:"issues"`
}

// AuditReport summarizes the results of the public export guard audit.
type AuditReport struct {
	TotalFilesAudited int          `json:"total_files_audited"`
	TotalViolations   int          `json:"total_violations"`
	Reports           []FileReport `json:"reports"`
	Passed            bool         `json:"passed"`
}

type patternRule struct {
	regex       *regexp.Regexp
	description string
	law         LawType
}

var defaultSecurityPatterns = []patternRule{
	{regexp.MustCompile(`eyJ[a-zA-Z0-9_-]{10,}\.eyJ[a-zA-Z0-9_-]{10,}`), "JWT authentication token detected", ZerothLaw},
	{regexp.MustCompile(`(?i)Authorization:\s*(?:Bearer|eyJ)`), "Authorization header with token/credential", ZerothLaw},
	{regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`), "AWS Access Key ID detected", ZerothLaw},
	{regexp.MustCompile(`\bAIza[0-9A-Za-z\-_]{35}\b`), "Google Cloud API Key detected", ZerothLaw},
	{regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`), "Private key detected (SSH / RSA / TLS)", ZerothLaw},
	{regexp.MustCompile(`\b(?:ghp_[a-zA-Z0-9]{36}|github_pat_[a-zA-Z0-9_]{82})\b`), "GitHub Personal Access Token detected", ZerothLaw},
	{regexp.MustCompile(`\bglpat-[a-zA-Z0-9\-_]{20,}\b`), "GitLab Personal Access Token detected", ZerothLaw},
	{regexp.MustCompile(`\bxox[baprs]-[0-9a-zA-Z]{10,48}\b`), "Slack Token detected", ZerothLaw},
	{regexp.MustCompile(`/home/[a-zA-Z0-9_-]+/(?:dev|work|github|projects)`), "Absolute local development path detected", FirstLaw},
	{regexp.MustCompile(`\[[A-E]\]\s*-\s*(?:Estratégico|Strategic|Impacto Local|Local Impact|Glue Work|Overhead)`), "Private Staff Time Tracking metrics detected", FirstLaw},
	{regexp.MustCompile(`\[(?:[A-Z]+-[0-9]+)\]\(https://[^\)]+\)`), "Corporate ticket reference detected (e.g. PROJ-123)", FirstLaw},
}

// Validator manages security guardrail auditing for public notes.
type Validator struct {
	vaultRoot string
	publicDir string
	patterns  []patternRule
}

// NewValidator creates a new instance with built-in security patterns and optional custom rules.
func NewValidator(vaultRoot string, customPatterns ...patternRule) *Validator {
	rules := make([]patternRule, len(defaultSecurityPatterns)) //nolint:prealloc
	copy(rules, defaultSecurityPatterns)
	rules = append(rules, customPatterns...)

	return &Validator{
		vaultRoot: vaultRoot,
		publicDir: filepath.Join(vaultRoot, "public"),
		patterns:  rules,
	}
}

// BuildCustomPatterns generates pattern rules from custom blocked domains and keywords.
func BuildCustomPatterns(blockedDomains []string, blockedKeywords []string) []patternRule { //nolint:revive
	var rules []patternRule
	for _, domain := range blockedDomains {
		domain = strings.TrimSpace(domain)
		if domain == "" {
			continue
		}
		pattern := fmt.Sprintf(`(?i)(?:https?://)?(?:[a-zA-Z0-9-]+\.)?%s`, regexp.QuoteMeta(domain))
		if re, err := regexp.Compile(pattern); err == nil {
			rules = append(rules, patternRule{
				regex:       re,
				description: fmt.Sprintf("Restricted corporate domain detected (%s)", domain),
				law:         ZerothLaw,
			})
		}
	}

	for _, kw := range blockedKeywords {
		kw = strings.TrimSpace(kw)
		if kw == "" {
			continue
		}
		pattern := fmt.Sprintf(`(?i)\b%s\b`, regexp.QuoteMeta(kw))
		if re, err := regexp.Compile(pattern); err == nil {
			rules = append(rules, patternRule{
				regex:       re,
				description: fmt.Sprintf("Restricted confidential keyword detected ('%s')", kw),
				law:         FirstLaw,
			})
		}
	}
	return rules
}

// CheckFile audits a single file against all guardrail rules under the Laws of Robotics.
func (v *Validator) CheckFile(filePath string) (*FileReport, error) {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(absPath) //nolint:gosec // intentional
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	content := string(data)
	relFile, _ := filepath.Rel(v.vaultRoot, absPath)
	relFile = filepath.ToSlash(relFile)

	var issues []Issue
	currDir := filepath.Dir(absPath)

	// 1. Remove code blocks for link parsing to avoid false positives
	codeBlockRegex := regexp.MustCompile("(?s)```.*?```")
	inlineCodeRegex := regexp.MustCompile("`[^`\n]+`")
	contentNoCode := codeBlockRegex.ReplaceAllString(content, "")
	contentNoCode = inlineCodeRegex.ReplaceAllString(contentNoCode, "")

	// 2. Check relative markdown links (Second Law: Graph & Boundary Integrity)
	linkRegex := regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	matches := linkRegex.FindAllStringSubmatch(contentNoCode, -1)

	for _, m := range matches {
		if len(m) < 3 {
			continue
		}
		text := m[1]
		link := m[2]

		if strings.HasPrefix(link, "http://") || strings.HasPrefix(link, "https://") ||
			strings.HasPrefix(link, "#") || strings.HasPrefix(link, "mailto:") ||
			strings.HasPrefix(link, "file://") {
			continue
		}

		cleanLink := strings.Split(link, "#")[0]
		cleanLink = strings.TrimSpace(cleanLink)
		if cleanLink == "" {
			continue
		}

		targetAbs := filepath.Clean(filepath.Join(currDir, cleanLink))

		// Check if target is inside public/
		if !strings.HasPrefix(targetAbs, v.publicDir) {
			issues = append(issues, Issue{
				Law:        ZerothLaw,
				Type:       LeakLinkOutsidePublic,
				Detail:     fmt.Sprintf("Link points outside of public boundary: '[%s](%s)'", text, link),
				Suggestion: "Remove the link or convert it to plain text / canonical relative link within public/.",
			})
		} else if _, err := os.Stat(targetAbs); os.IsNotExist(err) {
			relTarget, _ := filepath.Rel(v.publicDir, targetAbs)
			issues = append(issues, Issue{
				Law:        SecondLaw,
				Type:       BrokenPublicLink,
				Detail:     fmt.Sprintf("Broken relative link within public/: '[%s](%s)'", text, link),
				Suggestion: fmt.Sprintf("Target file '%s' does not exist.", filepath.ToSlash(relTarget)),
			})
		}
	}

	// 3. Check unconverted WikiLinks (Second Law)
	wikiRegex := regexp.MustCompile(`\[\[([^\]]+)\]\]`)
	wikiMatches := wikiRegex.FindAllStringSubmatch(content, -1)
	if len(wikiMatches) > 0 {
		var sample []string
		for i, wm := range wikiMatches {
			if i >= 3 {
				break
			}
			sample = append(sample, wm[1])
		}
		issues = append(issues, Issue{
			Law:        SecondLaw,
			Type:       UnconvertedWikiLinks,
			Detail:     fmt.Sprintf("Contains %d unconverted WikiLinks in [[...]] format (%s)", len(wikiMatches), strings.Join(sample, ", ")),
			Suggestion: "Convert WikiLinks to canonical relative Markdown links or run 'daneel dream' before publishing.",
		})
	}

	// 4. Check Suspicious Patterns, Credentials, and PII (Zeroth & First Laws)
	lines := strings.Split(content, "\n")
	for lineNum, line := range lines {
		for _, rule := range v.patterns {
			if loc := rule.regex.FindString(line); loc != "" {
				issues = append(issues, Issue{
					Law:        rule.law,
					Type:       InternalDataDetected,
					LineNumber: lineNum + 1,
					Detail:     fmt.Sprintf("Line %d: %s ('%s')", lineNum+1, rule.description, loc),
					Suggestion: "Sanitize the line by removing internal tokens or confidential references.",
				})
			}
		}
	}

	// 5. Check Frontmatter OKF (Third Law: Schema Standards)
	fmRegex := regexp.MustCompile(`(?s)^---\s*\n(.*?)\n---\s*\n`)
	fmM := fmRegex.FindStringSubmatch(content)
	if len(fmM) < 2 {
		issues = append(issues, Issue{
			Law:        ThirdLaw,
			Type:       MissingFrontmatter,
			Detail:     "Note is missing YAML frontmatter header",
			Suggestion: "Add frontmatter with 'type' and 'description' properties for Quartz/Digital Garden rendering.",
		})
	} else {
		var rawFM map[string]interface{}
		if err := yaml.Unmarshal([]byte(fmM[1]), &rawFM); err == nil {
			if rawFM["type"] == nil || fmt.Sprint(rawFM["type"]) == "" {
				issues = append(issues, Issue{
					Law:        ThirdLaw,
					Type:       MissingFrontmatter,
					Detail:     "YAML frontmatter does not contain required 'type' property",
					Suggestion: "Add 'type: tech' or appropriate knowledge category to frontmatter.",
				})
			}
		}
	}

	return &FileReport{
		RelPath: relFile,
		AbsPath: absPath,
		Issues:  issues,
	}, nil
}

// AuditPublicVault audits all Markdown notes in public/content/.
func (v *Validator) AuditPublicVault() (*AuditReport, error) {
	contentDir := filepath.Join(v.publicDir, "content")
	report := &AuditReport{
		Reports: []FileReport{},
		Passed:  true,
	}

	if _, err := os.Stat(contentDir); os.IsNotExist(err) {
		return report, nil
	}

	err := filepath.Walk(contentDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(strings.ToLower(info.Name()), ".md") {
			return err
		}

		fileRep, err := v.CheckFile(path)
		if err != nil {
			return err
		}

		report.TotalFilesAudited++
		if len(fileRep.Issues) > 0 {
			report.TotalViolations += len(fileRep.Issues)
			report.Reports = append(report.Reports, *fileRep)
			report.Passed = false
		}
		return nil
	})

	return report, err
}
