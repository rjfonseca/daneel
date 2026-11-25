package tui

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"connectrpc.com/connect"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	graphv1 "github.com/rjfonseca/daneel/api/v1"
	"github.com/rjfonseca/daneel/api/v1/graphv1connect"
)

type stubGraphClient struct {
	graphv1connect.GraphServiceClient
	workspaceResp *graphv1.GetWorkspaceOverviewResponse
}

func (s *stubGraphClient) GetWorkspaceOverview(
	_ context.Context,
	_ *connect.Request[graphv1.GetWorkspaceOverviewRequest],
) (*connect.Response[graphv1.GetWorkspaceOverviewResponse], error) {
	return connect.NewResponse(s.workspaceResp), nil
}

func (s *stubGraphClient) ReportActiveContext(
	_ context.Context,
	_ *connect.Request[graphv1.ReportActiveContextRequest],
) (*connect.Response[graphv1.ReportActiveContextResponse], error) {
	return connect.NewResponse(&graphv1.ReportActiveContextResponse{Success: true}), nil
}

func TestWorkspaceView_CustomWorkDirFiltersAndLayoutHeight(t *testing.T) {
	styles := NewStyles()
	wv := NewWorkspaceView(t.TempDir(), styles)

	backend := NewLocalBackend(t.TempDir(), nil, nil)
	backend.SetRPCClient(&stubGraphClient{
		workspaceResp: &graphv1.GetWorkspaceOverviewResponse{
			WorkDir:    "/home/user/Projects",
			TotalRepos: 3,
			CleanCount: 2,
			DirtyCount: 1,
			FromCache:  true,
			Repos: []*graphv1.WorkspaceRepoItem{
				{
					Name:          "daneel",
					RelPath:       "github.com/rjfonseca/daneel",
					Branch:        "main",
					Stack:         "Go",
					ComponentType: "Service",
					Dirty:         true,
					ModifiedCount: 2,
					Description:   "Knowledge engine CLI and daemon.",
				},
				{
					Name:        "ai-jail",
					RelPath:     "github.com/akitaonrails/ai-jail",
					Branch:      "master",
					Stack:       "Rust",
					Description: "Sandboxing runner.",
				},
				{
					Name:        "web-ui",
					RelPath:     "github.com/acme/web-ui",
					Branch:      "main",
					Stack:       "TypeScript/Node",
					Description: "Frontend.",
				},
			},
		},
	}, "http://127.0.0.1:4321")
	wv.SetBackend(backend)

	// Loading message should reflect real workDir, never hardcoded ~/Work
	loadingOut := wv.View()
	if strings.Contains(loadingOut, "~/Work") {
		t.Fatalf("expected WorkspaceView loading output not to hardcode ~/Work, got: %s", loadingOut)
	}

	cmd := wv.EnsureLoaded()
	if cmd == nil {
		t.Fatal("expected EnsureLoaded() to return async load tea.Cmd")
	}
	msg := cmd()
	wv, _ = wv.Update(msg)

	if wv.workDir != "/home/user/Projects" {
		t.Errorf("expected workDir /home/user/Projects, got %q", wv.workDir)
	}
	if len(wv.filteredRepos) != 3 {
		t.Fatalf("expected 3 repos, got %d", len(wv.filteredRepos))
	}

	// Cycle filter to "dirty"
	wv, _ = wv.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")})
	if wv.filter != "dirty" || len(wv.filteredRepos) != 1 || wv.filteredRepos[0].Name != "daneel" {
		t.Fatalf("expected dirty filter to isolate 'daneel', got filter=%q repos=%+v", wv.filter, wv.filteredRepos)
	}

	// Cycle back to "all" for layout checks
	for wv.filter != "all" {
		wv, _ = wv.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")})
	}

	for _, dims := range [][2]int{{80, 24}, {160, 45}} {
		w, h := dims[0], dims[1]
		wv.SetSize(w, h)
		out := wv.View()
		if gotH := lipgloss.Height(out); gotH > h {
			t.Errorf("layout overflow at %dx%d: rendered height %d > max %d\n%s", w, h, gotH, h, out)
		}
		if !strings.Contains(out, "daneel") {
			t.Errorf("expected repo 'daneel' in rendered output at %dx%d", w, h)
		}
	}

	// Ensure multi-repo overflow stays bounded
	var many []*graphv1.WorkspaceRepoItem
	for i := 0; i < 40; i++ {
		many = append(many, &graphv1.WorkspaceRepoItem{
			Name:    fmt.Sprintf("repo-%02d", i),
			RelPath: fmt.Sprintf("github.com/org/repo-%02d", i),
			Stack:   "Go",
		})
	}
	wv.applyOverviewResponse(&graphv1.GetWorkspaceOverviewResponse{
		WorkDir:    "/home/user/Projects",
		TotalRepos: 40,
		CleanCount: 40,
		Repos:      many,
	})
	wv.SetSize(100, 26)
	if gotH := lipgloss.Height(wv.View()); gotH > 26 {
		t.Errorf("layout overflow with 40 repos: rendered height %d > 26", gotH)
	}
}
