//nolint:revive // package comment not required
package harvester

import (
	"net/http"
	"net/http/httptest"
	"testing"

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
