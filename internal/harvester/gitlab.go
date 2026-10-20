//nolint:revive // package comment not required
package harvester

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rjfonseca/daneel/internal/config"
)

// GitLabHarvester manages downloading documentation from GitLab repositories.
type GitLabHarvester struct {
	vaultRoot string
	token     string
	baseURL   string
	client    *HTTPClient
	cfg       config.HarvesterConfig
}

// NewGitLabHarvester creates a new GitLab harvester.
func NewGitLabHarvester(vaultRoot string, token string, cfg config.HarvesterConfig) *GitLabHarvester {
	if token == "" {
		token = os.Getenv("DANEEL_GITLAB_TOKEN")
		if token == "" {
			token = os.Getenv("GITLAB_TOKEN")
		}
		if token == "" {
			token = os.Getenv("GLAB_TOKEN")
		}
	}

	return &GitLabHarvester{
		vaultRoot: vaultRoot,
		token:     token,
		baseURL:   "https://gitlab.com/api/v4",
		client:    newHTTPClient(),
		cfg:       cfg,
	}
}

// HarvestGroup harvests all markdown/docs from all projects in a GitLab group.
func (h *GitLabHarvester) HarvestGroup(groupPath string, limit int) (*HarvestStats, error) {
	if h.token == "" {
		return nil, fmt.Errorf("GitLab token not configured (set DANEEL_GITLAB_TOKEN or GITLAB_TOKEN)")
	}
	projects, err := h.getGroupProjects(groupPath, limit)
	if err != nil {
		return nil, err
	}
	return h.harvestProjects(projects)
}

// HarvestUser harvests all markdown/docs from all projects of a GitLab personal user.
func (h *GitLabHarvester) HarvestUser(userName string, limit int) (*HarvestStats, error) {
	if h.token == "" {
		return nil, fmt.Errorf("GitLab token not configured (set DANEEL_GITLAB_TOKEN or GITLAB_TOKEN)")
	}
	projects, err := h.getUserProjects(userName, limit)
	if err != nil {
		return nil, err
	}
	return h.harvestProjects(projects)
}

func (h *GitLabHarvester) harvestProjects(projects []glProject) (*HarvestStats, error) {
	start := time.Now()
	outputDir := filepath.Join(h.vaultRoot, "sources", "gitlab")
	cacheFile := filepath.Join(h.vaultRoot, ".agents", "cache", "gitlab_harvest_cache.json")
	cache := loadCache(cacheFile)

	stats := &HarvestStats{
		Platform:      "GitLab",
		OutputDir:     outputDir,
		TotalProjects: len(projects),
	}

	for _, p := range projects {
		projID := p.ID
		projPath := p.PathWithNamespace

		if h.cfg.ShouldSkipArchived() && p.Archived {
			stats.SkippedProjects++
			continue
		}
		if h.cfg.IsGroupExcluded(projPath) {
			stats.SkippedProjects++
			continue
		}
		if h.cfg.IsProjectExcluded(p.Name, projPath) {
			stats.SkippedProjects++
			continue
		}

		stats.ActiveProjects = append(stats.ActiveProjects, strings.ToLower(projPath))

		defaultBranch := p.DefaultBranch
		if defaultBranch == "" {
			defaultBranch = "main"
		}

		projCacheKey := fmt.Sprintf("proj:%d", projID)
		if p.LastActivityAt != "" && cache[projCacheKey] == p.LastActivityAt {
			stats.SkippedProjects++
			continue
		}

		tree, err := h.getProjectTree(projID, defaultBranch)
		if err != nil {
			continue
		}

		var wg sync.WaitGroup
		tasks := make(chan glTreeItem, 100)
		var mu sync.Mutex

		workerCount := 10 // Concurrent downloads per project
		for w := 0; w < workerCount; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for item := range tasks {
					// Download raw blob
					content, err := h.getRawFile(projID, item.Path, defaultBranch)
					if err != nil || len(content) == 0 {
						continue
					}

					destPath := filepath.Join(outputDir, filepath.FromSlash(projPath), filepath.FromSlash(item.Path))
					_ = os.MkdirAll(filepath.Dir(destPath), 0750)
					if err := os.WriteFile(destPath, content, 0600); err == nil {
						mu.Lock()
						cacheKey := fmt.Sprintf("%d:%s", projID, item.Path)
						cache[cacheKey] = item.ID
						stats.DownloadedCount++
						mu.Unlock()
					}
				}
			}()
		}

		for _, item := range tree {
			if item.Type != "blob" || !ShouldHarvest(item.Path, h.cfg) {
				continue
			}

			stats.TotalDocsFound++
			cacheKey := fmt.Sprintf("%d:%s", projID, item.Path)

			mu.Lock()
			cachedID := cache[cacheKey]
			mu.Unlock()

			if cachedID == item.ID {
				stats.SkippedCount++
				continue
			}

			tasks <- item
		}
		close(tasks)
		wg.Wait()

		// Generate or update repository index MOC
		repoDir := filepath.Join(outputDir, filepath.FromSlash(projPath))
		repoParts := strings.Split(filepath.ToSlash(projPath), "/")
		repoName := repoParts[len(repoParts)-1]
		groupPath := strings.Join(repoParts[:len(repoParts)-1], "/")
		_ = EnsureRepositoryIndex(repoDir, repoName, groupPath, "gitlab", p.WebURL, p.Description)

		if p.LastActivityAt != "" {
			cache[projCacheKey] = p.LastActivityAt
		}
	}

	_, _ = reconcilePlatformDir(outputDir, "gitlab")

	saveCache(cacheFile, cache)
	stats.DurationMs = time.Since(start).Milliseconds()
	return stats, nil
}

type glProject struct {
	ID                int    `json:"id"`
	Name              string `json:"name"`
	Description       string `json:"description"`
	WebURL            string `json:"web_url"`
	PathWithNamespace string `json:"path_with_namespace"`
	DefaultBranch     string `json:"default_branch"`
	LastActivityAt    string `json:"last_activity_at"`
	Archived          bool   `json:"archived"`
}

type glTreeItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
	Path string `json:"path"`
}

func (h *GitLabHarvester) getGroupProjects(groupPath string, limit int) ([]glProject, error) {
	var projects []glProject
	encoded := url.PathEscape(groupPath)
	page := 1
	perPage := 100

	archivedParam := ""
	if h.cfg.ShouldSkipArchived() {
		archivedParam = "&archived=false"
	}

	for {
		reqURL := fmt.Sprintf("%s/groups/%s/projects?include_subgroups=true&per_page=%d&page=%d%s&order_by=last_activity_at&sort=desc",
			h.baseURL, encoded, perPage, page, archivedParam)

		req, err := http.NewRequest("GET", reqURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("PRIVATE-TOKEN", h.token)
		req.Header.Set("User-Agent", "Daneel-Harvester/1.0")

		resp, err := h.client.doWithRetry(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close() //nolint:errcheck,gosec // fully read or safe to ignore
			if page == 1 {
				return nil, fmt.Errorf("failed to fetch projects for GitLab group '%s': HTTP %d", groupPath, resp.StatusCode)
			}
			break
		}

		data, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close() //nolint:errcheck,gosec // fully read or safe to ignore
		if err != nil {
			break
		}

		var pageProjects []glProject
		if err := json.Unmarshal(data, &pageProjects); err != nil {
			break
		}
		if len(pageProjects) == 0 {
			break
		}

		projects = append(projects, pageProjects...)
		if limit > 0 && len(projects) >= limit {
			projects = projects[:limit]
			break
		}

		totalPages := resp.Header.Get("X-Total-Pages")
		if totalPages == "" || fmt.Sprint(page) == totalPages {
			break
		}
		page++
	}

	return projects, nil //nolint:nilerr // returning collected items on error is intentional
}

func (h *GitLabHarvester) getUserProjects(userName string, limit int) ([]glProject, error) {
	var projects []glProject
	encoded := url.PathEscape(userName)
	page := 1
	perPage := 100

	archivedParam := ""
	if h.cfg.ShouldSkipArchived() {
		archivedParam = "&archived=false"
	}

	for {
		reqURL := fmt.Sprintf("%s/users/%s/projects?per_page=%d&page=%d%s&order_by=last_activity_at&sort=desc",
			h.baseURL, encoded, perPage, page, archivedParam)

		req, err := http.NewRequest("GET", reqURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("PRIVATE-TOKEN", h.token)
		req.Header.Set("User-Agent", "Daneel-Harvester/1.0")

		resp, err := h.client.doWithRetry(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close() //nolint:errcheck,gosec // fully read or safe to ignore
			if page == 1 {
				return nil, fmt.Errorf("failed to fetch projects for GitLab user '%s': HTTP %d", userName, resp.StatusCode)
			}
			break
		}

		data, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close() //nolint:errcheck,gosec // fully read or safe to ignore
		if err != nil {
			break
		}

		var pageProjects []glProject
		if err := json.Unmarshal(data, &pageProjects); err != nil {
			break
		}
		if len(pageProjects) == 0 {
			break
		}

		projects = append(projects, pageProjects...)
		if limit > 0 && len(projects) >= limit {
			projects = projects[:limit]
			break
		}

		totalPages := resp.Header.Get("X-Total-Pages")
		if totalPages == "" || fmt.Sprint(page) == totalPages {
			break
		}
		page++
	}

	return projects, nil //nolint:nilerr // returning collected items on error is intentional
}

func (h *GitLabHarvester) getProjectTree(projectID int, ref string) ([]glTreeItem, error) {
	var items []glTreeItem
	page := 1
	perPage := 100

	for {
		reqURL := fmt.Sprintf("%s/projects/%d/repository/tree?recursive=true&per_page=%d&page=%d&ref=%s",
			h.baseURL, projectID, perPage, page, url.QueryEscape(ref))

		req, err := http.NewRequest("GET", reqURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("PRIVATE-TOKEN", h.token)

		resp, err := h.client.doWithRetry(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close() //nolint:errcheck,gosec // fully read or safe to ignore
			break
		}

		data, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close() //nolint:errcheck,gosec // fully read or safe to ignore
		if err != nil {
			break
		}

		var pageItems []glTreeItem
		if err := json.Unmarshal(data, &pageItems); err != nil {
			break
		}
		if len(pageItems) == 0 {
			break
		}

		items = append(items, pageItems...)
		totalPages := resp.Header.Get("X-Total-Pages")
		if totalPages == "" || fmt.Sprint(page) == totalPages {
			break
		}
		page++
	}

	return items, nil //nolint:nilerr // returning collected items on error is intentional
}

func (h *GitLabHarvester) getRawFile(projectID int, filePath string, ref string) ([]byte, error) {
	encodedPath := url.PathEscape(filePath)
	reqURL := fmt.Sprintf("%s/projects/%d/repository/files/%s/raw?ref=%s",
		h.baseURL, projectID, encodedPath, url.QueryEscape(ref))

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("PRIVATE-TOKEN", h.token)

	resp, err := h.client.doWithRetry(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck,gosec // fully read or safe to ignore

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}
