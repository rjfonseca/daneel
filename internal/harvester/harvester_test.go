//nolint:revive // package comment not required
package harvester

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/rjfonseca/daneel/internal/config"
)

func TestIsDocFile(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"README.md", true},
		{"readme.org", true},
		{"architecture.md", true},
		{"docs/superpowers/plans/my-plan.md", true},
		{"docs/img/diagram.svg", true},
		{"docs/img/architecture.png", true},
		{"docs/schema/payload.json", true},
		{"adrs/0001-test.md", true},
		{"catalog-info/component.yaml", true},
		{"catalog-info.yaml", true},
		{"catalog-info.yml", true},
		{"node_modules/chalk/readme.md", false},
		{"sre/palestras/live-coding/node_modules/express/readme.md", false},
		{"vendor/github.com/gin-gonic/gin/readme.md", false},
		{"wp-content/themes/theme/readme.md", false},
		{"dist/docs/readme.md", false},
		{".git/config", false},
		{"coverage/lcov.info", false},
		{"src/main.go", false},
		{"cmd/app.ts", false},
		{"assets/logo.png", false},
	}

	for _, tt := range tests {
		res := isDocFile(tt.path)
		if res != tt.expected {
			t.Errorf("isDocFile(%s) = %v; want %v", tt.path, res, tt.expected)
		}
	}
}

func TestGitHubGetUserRepos(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/octocat/repos" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("page") == "1" {
			_, _ = w.Write([]byte(`[
				{"name": "hello-world", "default_branch": "main", "owner": {"login": "octocat"}},
				{"name": "spoon-knife", "default_branch": "master", "owner": {"login": "octocat"}}
			]`))
		} else {
			_, _ = w.Write([]byte(`[]`))
		}
	}))
	defer ts.Close()

	gh := NewGitHubHarvester(t.TempDir(), "test-token", config.HarvesterConfig{})
	gh.baseURL = ts.URL
	gh.client.baseDelay = 0

	repos, err := gh.getUserRepos("octocat", 0)
	if err != nil {
		t.Fatalf("getUserRepos failed: %v", err)
	}
	if len(repos) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(repos))
	}
	if repos[0].Name != "hello-world" || repos[0].Owner.Login != "octocat" {
		t.Errorf("unexpected repo 0: %+v", repos[0])
	}
}

func TestGitLabGetUserProjects(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/gitlabuser/projects" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Total-Pages", "1")
		_, _ = w.Write([]byte(`[
			{"id": 42, "path_with_namespace": "gitlabuser/my-project", "default_branch": "main", "archived": false}
		]`))
	}))
	defer ts.Close()

	gl := NewGitLabHarvester(t.TempDir(), "test-token", config.HarvesterConfig{})
	gl.baseURL = ts.URL
	gl.client.baseDelay = 0

	projects, err := gl.getUserProjects("gitlabuser", 0)
	if err != nil {
		t.Fatalf("getUserProjects failed: %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}
	if projects[0].PathWithNamespace != "gitlabuser/my-project" {
		t.Errorf("unexpected project: %+v", projects[0])
	}
}

func TestHarvesters_ConcurrentPoolAndRaceSafety(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		// GitLab: /groups/acme/projects
		case strings.HasPrefix(r.URL.Path, "/groups/acme/projects"):
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Total-Pages", "1")
			var projects []map[string]interface{}
			for i := 1; i <= 16; i++ {
				projects = append(projects, map[string]interface{}{
					"id":                  i,
					"name":                fmt.Sprintf("service-%d", i),
					"path_with_namespace": fmt.Sprintf("acme/service-%d", i),
					"default_branch":      "main",
					"archived":            false,
				})
			}
			_ = json.NewEncoder(w).Encode(projects)

		// GitLab: /projects/:id/repository/tree
		case strings.HasPrefix(r.URL.Path, "/projects/") && strings.HasSuffix(r.URL.Path, "/repository/tree"):
			time.Sleep(25 * time.Millisecond)
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Total-Pages", "1")
			tree := []map[string]string{
				{"id": "sha-readme", "name": "README.md", "type": "blob", "path": "README.md"},
				{"id": "sha-arch", "name": "arch.md", "type": "blob", "path": "docs/arch.md"},
			}
			_ = json.NewEncoder(w).Encode(tree)

		// GitLab: /projects/:id/repository/files/:path/raw
		case strings.HasPrefix(r.URL.Path, "/projects/") && strings.HasSuffix(r.URL.Path, "/raw"):
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte("# Doc content\n"))

		// GitHub: /orgs/acme/repos
		case strings.HasPrefix(r.URL.Path, "/orgs/acme/repos"):
			w.Header().Set("Content-Type", "application/json")
			var repos []map[string]interface{}
			for i := 1; i <= 16; i++ {
				repos = append(repos, map[string]interface{}{
					"name":           fmt.Sprintf("repo-%d", i),
					"full_name":      fmt.Sprintf("acme/repo-%d", i),
					"default_branch": "main",
					"archived":       false,
					"owner":          map[string]string{"login": "acme"},
				})
			}
			_ = json.NewEncoder(w).Encode(repos)

		// GitHub: /repos/acme/:repo/git/trees/main
		case strings.HasPrefix(r.URL.Path, "/repos/acme/") && strings.Contains(r.URL.Path, "/git/trees/"):
			time.Sleep(25 * time.Millisecond)
			w.Header().Set("Content-Type", "application/json")
			treeResp := map[string]interface{}{
				"tree": []map[string]string{
					{"path": "README.md", "type": "blob", "sha": "sha-readme"},
					{"path": "docs/arch.md", "type": "blob", "sha": "sha-arch"},
				},
			}
			_ = json.NewEncoder(w).Encode(treeResp)

		// GitHub raw file: /raw/acme/:repo/main/:path
		case strings.HasPrefix(r.URL.Path, "/raw/"):
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte("# Doc content\n"))

		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	// 1. GitLab Harvester Concurrency & Race Safety
	t.Run("GitLab", func(t *testing.T) {
		tempDir := t.TempDir()
		gl := NewGitLabHarvester(tempDir, "test-token", config.HarvesterConfig{})
		gl.baseURL = srv.URL
		gl.client.baseDelay = -100 * time.Millisecond

		start := time.Now()
		stats, err := gl.HarvestGroup("acme", 0)
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("HarvestGroup failed: %v", err)
		}
		if stats.TotalProjects != 16 {
			t.Errorf("expected 16 TotalProjects, got %d", stats.TotalProjects)
		}
		if stats.DownloadedCount != 32 {
			t.Errorf("expected 32 DownloadedCount, got %d", stats.DownloadedCount)
		}
		if !sort.StringsAreSorted(stats.ActiveProjects) {
			t.Errorf("expected ActiveProjects to be sorted, got %v", stats.ActiveProjects)
		}
		if len(stats.ActiveProjects) != 16 {
			t.Errorf("expected 16 ActiveProjects, got %d", len(stats.ActiveProjects))
		}
		if elapsed >= 1000*time.Millisecond {
			t.Errorf("GitLab harvest took %v, expected < 1000ms (concurrent 8 workers)", elapsed)
		}
	})

	// 2. GitHub Harvester Concurrency & Race Safety
	t.Run("GitHub", func(t *testing.T) {
		tempDir := t.TempDir()
		gh := NewGitHubHarvester(tempDir, "test-token", config.HarvesterConfig{})
		gh.baseURL = srv.URL
		gh.client.baseDelay = -100 * time.Millisecond

		start := time.Now()
		stats, err := gh.HarvestOrg("acme", 0)
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("HarvestOrg failed: %v", err)
		}
		if stats.TotalProjects != 16 {
			t.Errorf("expected 16 TotalProjects, got %d", stats.TotalProjects)
		}
		if stats.DownloadedCount != 32 {
			t.Errorf("expected 32 DownloadedCount, got %d", stats.DownloadedCount)
		}
		if !sort.StringsAreSorted(stats.ActiveProjects) {
			t.Errorf("expected ActiveProjects to be sorted, got %v", stats.ActiveProjects)
		}
		if len(stats.ActiveProjects) != 16 {
			t.Errorf("expected 16 ActiveProjects, got %d", len(stats.ActiveProjects))
		}
		if elapsed >= 1000*time.Millisecond {
			t.Errorf("GitHub harvest took %v, expected < 1000ms (concurrent 8 workers)", elapsed)
		}
	})
}
