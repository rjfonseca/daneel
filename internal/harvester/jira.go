//nolint:revive // package comment not required
package harvester

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rjfonseca/daneel/internal/config"
)

type JiraHarvester struct { //nolint:revive
	vaultPath string
	token     string
	cfg       config.HarvesterConfig
	client    *HTTPClient
}

func NewJiraHarvester(vaultPath, token string, cfg config.HarvesterConfig) *JiraHarvester { //nolint:revive
	if token == "" {
		token = os.Getenv("DANEEL_JIRA_TOKEN")
	}
	return &JiraHarvester{
		vaultPath: vaultPath,
		token:     token,
		cfg:       cfg,
		client:    newHTTPClient(),
	}
}

type jiraSearchResponse struct {
	Issues []jiraIssue `json:"issues"`
}

type jiraIssue struct {
	Key            string             `json:"key"`
	Fields         jiraIssueField     `json:"fields"`
	RenderedFields *jiraRenderedField `json:"renderedFields"`
}

type jiraIssueField struct {
	Summary  string     `json:"summary"`
	Created  string     `json:"created"`
	Updated  string     `json:"updated"`
	Status   jiraStatus `json:"status"`
	Assignee *jiraUser  `json:"assignee"`
	Reporter *jiraUser  `json:"reporter"`
}

type jiraRenderedField struct {
	Description string                `json:"description"`
	Comment     jiraRenderedCommentPg `json:"comment"`
}

type jiraStatus struct {
	Name string `json:"name"`
}

type jiraUser struct {
	DisplayName string `json:"displayName"`
}

type jiraRenderedCommentPg struct {
	Comments []jiraRenderedComment `json:"comments"`
}

type jiraRenderedComment struct {
	Author  *jiraUser `json:"author"`
	Created string    `json:"created"`
	Body    string    `json:"body"`
}

func (h *JiraHarvester) Harvest(ctx context.Context) (*HarvestStats, error) { //nolint:revive
	start := time.Now()
	stats := &HarvestStats{
		Platform: "jira",
	}

	if h.cfg.Jira == nil {
		return nil, fmt.Errorf("jira config not found in harvester config")
	}

	domain := h.cfg.Jira.Domain
	email := h.cfg.Jira.Email
	if domain == "" || email == "" || h.token == "" {
		return nil, fmt.Errorf("domain, email or DANEEL_JIRA_TOKEN missing")
	}

	auth := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", email, h.token)))

	cachePath := filepath.Join(os.Getenv("HOME"), ".cache", "daneel", "jira_harvest_cache.json")
	if xdgCache := os.Getenv("XDG_CACHE_HOME"); xdgCache != "" {
		cachePath = filepath.Join(xdgCache, "daneel", "jira_harvest_cache.json")
	}
	cache := loadCache(cachePath)
	defer func() { saveCache(cachePath, cache) }()

	// Validate authentication first
	myselfURL := fmt.Sprintf("https://%s/rest/api/3/myself", domain)
	reqMyself, _ := http.NewRequestWithContext(ctx, "GET", myselfURL, nil)
	reqMyself.Header.Add("Authorization", "Basic "+auth)
	reqMyself.Header.Add("Accept", "application/json")
	respMyself, err := h.client.doWithRetry(reqMyself)
	if err != nil {
		return nil, fmt.Errorf("failed to check authentication: %w", err)
	}
	if respMyself.StatusCode != http.StatusOK {
		respMyself.Body.Close() //nolint:errcheck,gosec // fully read or safe to ignore
		return nil, fmt.Errorf("jira authentication failed (HTTP %d). Check your email in .daneel.yaml and your DANEEL_JIRA_TOKEN", respMyself.StatusCode)
	}
	respMyself.Body.Close() //nolint:errcheck,gosec // fully read or safe to ignore

	mocContent := "# 📌 MOC: Tickets Ativos\n\n"

	for _, query := range h.cfg.Jira.Queries {
		mocContent += fmt.Sprintf("## 🚀 %s\n", query.Name)

		targetDir := filepath.Join(h.vaultPath, query.TargetDir)
		os.MkdirAll(targetDir, 0750) //nolint:errcheck,gosec

		apiURL := fmt.Sprintf("https://%s/rest/api/3/search/jql", domain)
		bodyData := map[string]interface{}{
			"jql":        query.JQL,
			"maxResults": 100,
			"fields":     []string{"summary", "status", "assignee", "reporter", "created", "updated", "comment", "description"},
			"expand":     "renderedFields",
		}
		bodyBytes, _ := json.Marshal(bodyData)
		req, _ := http.NewRequestWithContext(ctx, "POST", apiURL, strings.NewReader(string(bodyBytes)))

		req.Header.Add("Authorization", "Basic "+auth)
		req.Header.Add("Accept", "application/json")
		req.Header.Add("Content-Type", "application/json")

		resp, err := h.client.doWithRetry(req)
		if err != nil {
			fmt.Printf("❌ Failed to query %s: %v\n", query.Name, err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close() //nolint:errcheck,gosec // fully read or safe to ignore
			fmt.Printf("❌ Failed to query %s (HTTP %d): %s\n", query.Name, resp.StatusCode, string(body))
			continue
		}

		var searchRes jiraSearchResponse
		bodyBytes2, _ := io.ReadAll(resp.Body)
		if err := json.Unmarshal(bodyBytes2, &searchRes); err != nil {
			fmt.Printf("❌ JSON decode error for %s: %v\n", query.Name, err)
			fmt.Printf("Body: %s\n", string(bodyBytes2))
			resp.Body.Close() //nolint:errcheck,gosec // fully read or safe to ignore
			continue
		}

		if len(searchRes.Issues) == 0 {
			fmt.Printf("⚠️ No issues found for %s. Jira response: %s\n", query.Name, string(bodyBytes2))
		}
		resp.Body.Close() //nolint:errcheck,gosec // fully read or safe to ignore

		for _, issue := range searchRes.Issues {
			stats.TotalDocsFound++

			if lastUpdated, ok := cache[issue.Key]; ok && lastUpdated == issue.Fields.Updated {
				stats.SkippedCount++
			} else {
				// Process and save
				content := h.formatIssue(issue, domain, query.TargetDir)
				destPath := filepath.Join(targetDir, issue.Key+".md")
				os.WriteFile(destPath, []byte(content), 0600) //nolint:errcheck,gosec

				cache[issue.Key] = issue.Fields.Updated
				stats.DownloadedCount++
			}

			// Add to MOC
			// Make relative path for MOC
			// if targetDir is "sources/jira/meus-ativos" and MOC is in "notes/processors/moc-tickets.md"
			// Wait, the MOC will be placed in notes/processors/moc-tickets.md.
			relPath := fmt.Sprintf("../../%s/%s.md", query.TargetDir, issue.Key)
			mocContent += fmt.Sprintf("- [%s](%s) [%s] — %s\n", issue.Key, relPath, issue.Fields.Status.Name, issue.Fields.Summary)
		}
		mocContent += "\n"
	}

	// Write MOC
	mocDir := filepath.Join(h.vaultPath, "notes", "processors")
	os.MkdirAll(mocDir, 0750) //nolint:errcheck,gosec
	mocFile := filepath.Join(mocDir, "moc-tickets-ativos.md")
	os.WriteFile(mocFile, []byte(mocContent), 0600) //nolint:errcheck,gosec

	stats.DurationMs = time.Since(start).Milliseconds()
	stats.OutputDir = filepath.Join(h.vaultPath, "notes", "processors", "moc-tickets-ativos.md")

	return stats, nil
}

func (h *JiraHarvester) formatIssue(issue jiraIssue, domain, targetDir string) string { //nolint:revive
	created := issue.Fields.Created
	if len(created) > 10 {
		created = created[:10]
	}
	updated := issue.Fields.Updated
	if len(updated) > 10 {
		updated = updated[:10]
	}

	assignee := "Unassigned"
	if issue.Fields.Assignee != nil {
		assignee = issue.Fields.Assignee.DisplayName
	}

	reporter := "Unknown"
	if issue.Fields.Reporter != nil {
		reporter = issue.Fields.Reporter.DisplayName
	}

	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString("type: source\n")
	sb.WriteString("subtipo: ticket-jira\n")
	fmt.Fprintf(&sb, "id: %s\n", issue.Key)
	fmt.Fprintf(&sb, "status: %s\n", issue.Fields.Status.Name)
	fmt.Fprintf(&sb, "assignee: %s\n", assignee)
	fmt.Fprintf(&sb, "reporter: %s\n", reporter)
	fmt.Fprintf(&sb, "created: %s\n", created)
	fmt.Fprintf(&sb, "updated: %s\n", updated)
	sb.WriteString("tags:\n")
	sb.WriteString("  - sources/jira\n")
	sb.WriteString("---\n\n")

	fmt.Fprintf(&sb, "# [%s] %s\n\n", issue.Key, issue.Fields.Summary)
	fmt.Fprintf(&sb, "**Link:** [Visualizar no Jira](https://%s/browse/%s)\n\n", domain, issue.Key)

	sb.WriteString("## Descrição\n")
	if issue.RenderedFields != nil && issue.RenderedFields.Description != "" {
		sb.WriteString(issue.RenderedFields.Description + "\n")
	} else {
		sb.WriteString("Sem descrição.\n")
	}

	if issue.RenderedFields != nil && len(issue.RenderedFields.Comment.Comments) > 0 {
		sb.WriteString("\n## Comentários\n")
		for _, c := range issue.RenderedFields.Comment.Comments {
			cDate := c.Created
			if len(cDate) > 10 {
				cDate = cDate[:10]
			}
			author := "Unknown"
			if c.Author != nil {
				author = c.Author.DisplayName
			}
			body := strings.ReplaceAll(c.Body, "\n", "\n  ")
			fmt.Fprintf(&sb, "- **%s (%s):**\n  %s\n", author, cDate, body)
		}
	}

	return sb.String()
}
