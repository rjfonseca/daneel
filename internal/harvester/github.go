//nolint:revive // package comment not required
package harvester

import (
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

// GitHubHarvester manages downloading documentation from GitHub repositories.
type GitHubHarvester struct {
	vaultRoot string
	token     string
	baseURL   string
	client    *HTTPClient
	cfg       config.HarvesterConfig
}

// NewGitHubHarvester creates a new GitHub harvester.
func NewGitHubHarvester(vaultRoot string, token string, cfg config.HarvesterConfig) *GitHubHarvester {
	if token == "" {
		token = os.Getenv("GITHUB_PERSONAL_ACCESS_TOKEN")
		if token == "" {
			token = os.Getenv("GITHUB_TOKEN")
		}
		if token == "" {
			token = os.Getenv("GH_TOKEN")
		}
	}

	return &GitHubHarvester{
		vaultRoot: vaultRoot,
		token:     token,
		baseURL:   "https://api.github.com",
		client:    newHTTPClient(),
		cfg:       cfg,
	}
}

// HarvestOrg harvests all documentation from an organization.
func (h *GitHubHarvester) HarvestOrg(orgName string, limit int) (*HarvestStats, error) {
	repos, err := h.getOrgRepos(orgName, limit)
	if err != nil {
		return nil, err
	}
	return h.harvestRepos(repos)
}

// HarvestUser harvests all documentation from a personal user.
func (h *GitHubHarvester) HarvestUser(userName string, limit int) (*HarvestStats, error) {
	repos, err := h.getUserRepos(userName, limit)
	if err != nil {
		return nil, err
	}
	return h.harvestRepos(repos)
}

func (h *GitHubHarvester) harvestRepos(repos []ghRepo) (*HarvestStats, error) {
	start := time.Now()
	outputDir := filepath.Join(h.vaultRoot, "sources", "github")
	cacheFile := filepath.Join(h.vaultRoot, ".agents", "cache", "github_harvest_cache.json")
	cache := loadCache(cacheFile)

	stats := &HarvestStats{
		Platform:      "GitHub",
		OutputDir:     outputDir,
		TotalProjects: len(repos),
	}

	for _, repo := range repos {
		owner := repo.Owner.Login
		repoName := repo.Name
		fullName := repo.FullName
		if fullName == "" {
			fullName = fmt.Sprintf("%s/%s", owner, repoName)
		}

		if h.cfg.ShouldSkipArchived() && repo.Archived {
			stats.SkippedProjects++
			continue
		}
		if h.cfg.IsGroupExcluded(owner) || h.cfg.IsGroupExcluded(fullName) {
			stats.SkippedProjects++
			continue
		}
		if h.cfg.IsProjectExcluded(repoName, fullName) {
			stats.SkippedProjects++
			continue
		}

		stats.ActiveProjects = append(stats.ActiveProjects, strings.ToLower(fullName))

		defaultBranch := repo.DefaultBranch
		if defaultBranch == "" {
			defaultBranch = "main"
		}

		tree, err := h.getRepoTree(owner, repoName, defaultBranch)
		if err != nil {
			continue
		}

		for _, item := range tree {
			if item.Type != "blob" || !ShouldHarvest(item.Path, h.cfg) {
				continue
			}

			stats.TotalDocsFound++
			cacheKey := fmt.Sprintf("%s/%s:%s", owner, repoName, item.Path)
			if cache[cacheKey] == item.SHA {
				stats.SkippedCount++
				continue
			}

			content, err := h.getRawFile(owner, repoName, defaultBranch, item.Path)
			if err != nil || len(content) == 0 {
				continue
			}

			destPath := filepath.Join(outputDir, owner, repoName, filepath.FromSlash(item.Path))
			_ = os.MkdirAll(filepath.Dir(destPath), 0750)
			if err := os.WriteFile(destPath, content, 0600); err == nil {
				cache[cacheKey] = item.SHA
				stats.DownloadedCount++
			}
		}

		// Generate or update repository index MOC
		repoDir := filepath.Join(outputDir, owner, repoName)
		_ = EnsureRepositoryIndex(repoDir, repoName, owner, "github", repo.HTMLURL, repo.Description)
	}

	_, _ = reconcilePlatformDir(outputDir, "github")

	saveCache(cacheFile, cache)
	stats.DurationMs = time.Since(start).Milliseconds()
	return stats, nil
}

type ghRepo struct {
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	Description   string `json:"description"`
	HTMLURL       string `json:"html_url"`
	DefaultBranch string `json:"default_branch"`
	Archived      bool   `json:"archived"`
	Owner         struct {
		Login string `json:"login"`
	} `json:"owner"`
}

type ghTreeItem struct {
	Path string `json:"path"`
	Type string `json:"type"`
	SHA  string `json:"sha"`
}

func (h *GitHubHarvester) getOrgRepos(orgName string, limit int) ([]ghRepo, error) {
	var repos []ghRepo
	page := 1
	perPage := 100

	for {
		reqURL := fmt.Sprintf("%s/orgs/%s/repos?per_page=%d&page=%d&sort=pushed", h.baseURL, orgName, perPage, page)
		req, err := http.NewRequest("GET", reqURL, nil)
		if err != nil {
			return nil, err
		}
		if h.token != "" {
			req.Header.Set("Authorization", "Bearer "+h.token)
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("User-Agent", "Daneel-Harvester/1.0")

		resp, err := h.client.doWithRetry(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close() //nolint:errcheck,gosec // fully read or safe to ignore
			if page == 1 {
				return nil, fmt.Errorf("failed to fetch repositories for GitHub organization '%s': HTTP %d", orgName, resp.StatusCode)
			}
			break
		}

		data, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close() //nolint:errcheck,gosec // fully read or safe to ignore
		if err != nil {
			break
		}

		var pageRepos []ghRepo
		if err := json.Unmarshal(data, &pageRepos); err != nil {
			break
		}
		if len(pageRepos) == 0 {
			break
		}

		repos = append(repos, pageRepos...)
		if limit > 0 && len(repos) >= limit {
			repos = repos[:limit]
			break
		}
		if len(pageRepos) < perPage {
			break
		}
		page++
	}

	return repos, nil //nolint:nilerr // returning collected items on error is intentional
}

func (h *GitHubHarvester) getUserRepos(userName string, limit int) ([]ghRepo, error) {
	var repos []ghRepo
	page := 1
	perPage := 100

	for {
		reqURL := fmt.Sprintf("%s/users/%s/repos?per_page=%d&page=%d&sort=pushed", h.baseURL, userName, perPage, page)
		req, err := http.NewRequest("GET", reqURL, nil)
		if err != nil {
			return nil, err
		}
		if h.token != "" {
			req.Header.Set("Authorization", "Bearer "+h.token)
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("User-Agent", "Daneel-Harvester/1.0")

		resp, err := h.client.doWithRetry(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close() //nolint:errcheck,gosec // fully read or safe to ignore
			if page == 1 {
				return nil, fmt.Errorf("failed to fetch repositories for GitHub user '%s': HTTP %d", userName, resp.StatusCode)
			}
			break
		}

		data, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close() //nolint:errcheck,gosec // fully read or safe to ignore
		if err != nil {
			break
		}

		var pageRepos []ghRepo
		if err := json.Unmarshal(data, &pageRepos); err != nil {
			break
		}
		if len(pageRepos) == 0 {
			break
		}

		repos = append(repos, pageRepos...)
		if limit > 0 && len(repos) >= limit {
			repos = repos[:limit]
			break
		}
		if len(pageRepos) < perPage {
			break
		}
		page++
	}

	return repos, nil //nolint:nilerr // returning collected items on error is intentional
}

func (h *GitHubHarvester) getRepoTree(owner string, repo string, branch string) ([]ghTreeItem, error) {
	reqURL := fmt.Sprintf("%s/repos/%s/%s/git/trees/%s?recursive=1", h.baseURL, owner, repo, branch)
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	if h.token != "" {
		req.Header.Set("Authorization", "Bearer "+h.token)
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := h.client.doWithRetry(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck,gosec // fully read or safe to ignore

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var payload struct {
		Tree []ghTreeItem `json:"tree"`
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(data, &payload)
	return payload.Tree, nil
}

func (h *GitHubHarvester) getRawFile(owner string, repo string, branch string, path string) ([]byte, error) {
	reqURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", owner, repo, branch, path)
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	if h.token != "" {
		req.Header.Set("Authorization", "Bearer "+h.token)
	}

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
