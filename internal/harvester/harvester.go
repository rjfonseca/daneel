//nolint:revive // package comment not required
package harvester

import (
	"encoding/json"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/rjfonseca/daneel/internal/config"
)

// HarvestStats reports the summary of the harvesting process.
type HarvestStats struct {
	Platform        string   `json:"platform"`
	TotalProjects   int      `json:"total_projects"`
	SkippedProjects int      `json:"skipped_projects"`
	TotalDocsFound  int      `json:"total_docs_found"`
	DownloadedCount int      `json:"downloaded_count"`
	SkippedCount    int      `json:"skipped_count"`
	OutputDir       string   `json:"output_dir"`
	DurationMs      int64    `json:"duration_ms"`
	ActiveProjects  []string `json:"active_projects,omitempty"`
}

// ShouldHarvest determines whether a remote repository item path should be downloaded.
func ShouldHarvest(path string, cfg config.HarvesterConfig) bool {
	lower := strings.ToLower(filepath.ToSlash(path))

	// 0. Immediate block of toxic directories (node_modules, vendor, dist, etc.)
	ignored := cfg.GetIgnoredDirectories()
	for _, ign := range ignored {
		ignLower := strings.ToLower(ign)
		if strings.HasPrefix(lower, ignLower+"/") ||
			strings.Contains(lower, "/"+ignLower+"/") ||
			lower == ignLower {
			return false
		}
	}

	base := filepath.Base(lower)
	ext := filepath.Ext(lower)

	dirs := cfg.MatchDirectories
	if len(dirs) == 0 {
		dirs = config.DefaultHarvesterConfig().MatchDirectories
	}
	filenames := cfg.MatchFilenames
	if len(filenames) == 0 {
		filenames = config.DefaultHarvesterConfig().MatchFilenames
	}
	extensions := cfg.MatchExtensions
	if len(extensions) == 0 {
		extensions = config.DefaultHarvesterConfig().MatchExtensions
	}

	// 1. Check exact filenames
	for _, fn := range filenames {
		if strings.ToLower(fn) == base {
			return true
		}
	}

	// 2. Check if inside matched directories (e.g. docs/, catalog-info/)
	for _, d := range dirs {
		dLower := strings.ToLower(d)
		if strings.HasPrefix(lower, dLower+"/") || strings.Contains(lower, "/"+dLower+"/") {
			// If download all is enabled, accept any file inside this directory (images, schemas, docs)
			if cfg.DownloadAllInDirs {
				return true
			}
			// Otherwise verify extension
			for _, extMatch := range extensions {
				if strings.ToLower(extMatch) == ext {
					return true
				}
			}
		}
	}

	// 3. Fallback check for extensions
	for _, extMatch := range extensions {
		if strings.ToLower(extMatch) == ext {
			for _, d := range dirs {
				dLower := strings.ToLower(d)
				if strings.Contains(lower, "/"+dLower+"/") || strings.HasPrefix(lower, dLower+"/") {
					return true
				}
			}
		}
	}

	return false
}

func isDocFile(path string) bool {
	return ShouldHarvest(path, config.DefaultHarvesterConfig())
}

// HTTPClient wraps standard http.Client with resilient retries and rate limit handling.
type HTTPClient struct {
	client     *http.Client
	maxRetries int
	baseDelay  time.Duration
}

func newHTTPClient() *HTTPClient {
	return &HTTPClient{
		client:     &http.Client{Timeout: 30 * time.Second},
		maxRetries: 5,
		baseDelay:  200 * time.Millisecond,
	}
}

func (c *HTTPClient) doWithRetry(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	var err error

	for attempt := 1; attempt <= c.maxRetries; attempt++ {
		time.Sleep(c.baseDelay + time.Duration(rand.Intn(100))*time.Millisecond) //nolint:gosec // intentional

		resp, err = c.client.Do(req)
		if err == nil {
			if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNotFound {
				return resp, nil
			}

			// Handle Rate Limiting (429) or Server Errors (5xx)
			if resp.StatusCode == 429 || (resp.StatusCode >= 500 && resp.StatusCode <= 504) {
				_ = resp.Body.Close()                                                                               //nolint:errcheck,gosec // fully read or safe to ignore
				sleepTime := time.Duration(1<<attempt)*time.Second + time.Duration(rand.Intn(500))*time.Millisecond //nolint:gosec // intentional
				if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
					if sec, err := strconv.Atoi(retryAfter); err == nil {
						sleepTime = time.Duration(sec+1) * time.Second
					}
				}
				time.Sleep(sleepTime)
				continue
			}

			return resp, nil
		}

		time.Sleep(time.Duration(1<<attempt)*time.Second + time.Duration(rand.Intn(500))*time.Millisecond) //nolint:gosec // intentional
	}

	return resp, err
}

func loadCache(cachePath string) map[string]string {
	cache := make(map[string]string)
	data, err := os.ReadFile(cachePath) //nolint:gosec // intentional
	if err == nil {
		_ = json.Unmarshal(data, &cache)
	}
	return cache
}

func saveCache(cachePath string, cache map[string]string) {
	_ = os.MkdirAll(filepath.Dir(cachePath), 0750) //nolint:gosec // intentional
	data, err := json.MarshalIndent(cache, "", "  ")
	if err == nil {
		_ = os.WriteFile(cachePath, data, 0600) //nolint:gosec // intentional
	}
}
