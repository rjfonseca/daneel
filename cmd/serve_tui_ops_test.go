package cmd

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"connectrpc.com/connect"

	graphv1 "github.com/rjfonseca/daneel/api/v1"
	"github.com/rjfonseca/daneel/api/v1/graphv1connect"
	"github.com/rjfonseca/daneel/internal/config"
	"github.com/rjfonseca/daneel/internal/ops/workspace"
	"github.com/rjfonseca/daneel/internal/routine"
	"github.com/rjfonseca/daneel/internal/search"
)

type surfaceHeaderTransport struct {
	base    http.RoundTripper
	surface string
}

func (t surfaceHeaderTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r2 := r.Clone(r.Context())
	if t.surface != "" {
		r2.Header.Set("X-Daneel-Surface", t.surface)
	}
	return t.base.RoundTrip(r2)
}

func TestNewGalaxyHTTPServer_RecordsTUISurfaceTelemetry(t *testing.T) {
	tempVault := t.TempDir()
	if err := os.WriteFile(filepath.Join(tempVault, ".daneel.yaml"), []byte("vault:\n  title: test\n"), 0600); err != nil {
		t.Fatal(err)
	}
	eng, err := search.Open(filepath.Join(tempVault, ".daneel", "cache", "vault_index.db"), tempVault)
	if err != nil {
		t.Fatalf("failed to open search engine: %v", err)
	}
	defer func() { _ = eng.Close() }()

	cfg := &config.Config{VaultPath: tempVault}
	httpSrv, err := NewGalaxyHTTPServer(cfg, eng, 4321, nil)
	if err != nil {
		t.Fatalf("NewGalaxyHTTPServer failed: %v", err)
	}
	cleanupGalaxyHTTPServer(t, httpSrv)

	httpClient := &http.Client{
		Transport: surfaceHeaderTransport{
			base:    localHandlerTransport{handler: httpSrv.Handler, host: "127.0.0.1:4321"},
			surface: "tui",
		},
	}
	client := graphv1connect.NewGraphServiceClient(httpClient, "http://127.0.0.1:4321")
	if _, err := client.GetPeopleDirectory(context.Background(), connect.NewRequest(&graphv1.GetPeopleDirectoryRequest{})); err != nil {
		t.Fatalf("GetPeopleDirectory RPC failed: %v", err)
	}

	store, err := routine.OpenDefaultStore(tempVault)
	if err != nil {
		t.Fatalf("OpenDefaultStore failed: %v", err)
	}
	defer func() { _ = store.Close() }()

	aggs, err := store.GetCommandAggregates(context.Background(), 0, 50)
	if err != nil {
		t.Fatalf("GetCommandAggregates failed: %v", err)
	}

	foundTUI := false
	for _, a := range aggs {
		if a.Surface == "tui" && a.Command == "GetPeopleDirectory" && a.TotalCalls == 1 {
			foundTUI = true
		}
		if a.Surface == "web" && a.Command == "GetPeopleDirectory" {
			t.Fatalf("expected X-Daneel-Surface: tui not to be logged as web, got %+v", a)
		}
	}
	if !foundTUI {
		t.Fatalf("expected tui:GetPeopleDirectory telemetry event, got %+v", aggs)
	}
}

func TestGraphServer_TUIUnifiedRPCs(t *testing.T) {
	tempVault := t.TempDir()
	workDir := filepath.Join(t.TempDir(), "Projects")
	if err := os.MkdirAll(filepath.Join(tempVault, "daily"), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tempVault, "notes", "carreira"), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tempVault, "notes", "pessoas"), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(workDir, 0750); err != nil {
		t.Fatal(err)
	}

	today := time.Now().Format("2006-01-02")
	dailyContent := "---\ntempo-A_Estrategico: 4.0\ntempo-B_ImpactoLocal: 4.0\n---\n# Daily\n- Item de bloqueio relevante no fluxo\n"
	if err := os.WriteFile(filepath.Join(tempVault, "daily", today+".md"), []byte(dailyContent), 0600); err != nil {
		t.Fatal(err)
	}

	sumarioContent := "## 📌 Foco Atual & Pendências (Backlog Ativo)\n### 🔴 P0\n- [x] Done task: already shipped\n- [ ] Open task: pending work\n"
	if err := os.WriteFile(filepath.Join(tempVault, "notes", "carreira", "sumario-semanal-atividades-impactos-2026.md"), []byte(sumarioContent), 0600); err != nil {
		t.Fatal(err)
	}

	personContent := "---\ntype: person\nrole: Staff Engineer\nsquad: Platform\n---\n# 👤 Alice Silva\n"
	if err := os.WriteFile(filepath.Join(tempVault, "notes", "pessoas", "alice-silva.md"), []byte(personContent), 0600); err != nil {
		t.Fatal(err)
	}

	eng, err := search.Open(filepath.Join(tempVault, ".daneel", "cache", "vault_index.db"), tempVault)
	if err != nil {
		t.Fatalf("search.Open failed: %v", err)
	}
	defer func() { _ = eng.Close() }()

	if err := eng.SaveWorkspaceRepos(context.Background(), []workspace.RepoInfo{
		{Name: "daneel", RelPath: "github.com/rjfonseca/daneel", Branch: "main", Stack: "Go"},
	}); err != nil {
		t.Fatalf("SaveWorkspaceRepos failed: %v", err)
	}

	st, err := routine.OpenDefaultStore(tempVault)
	if err != nil {
		t.Fatalf("OpenDefaultStore failed: %v", err)
	}
	defer func() { _ = st.Close() }()

	now := time.Now().Unix()
	_ = st.UpsertSessionTelemetry(context.Background(), routine.SessionTelemetryRecord{
		SessionID: "history", Agent: "claude-code", Model: "claude-sonnet", StartedAt: now, UpdatedAt: now, TurnsCount: 99,
	})
	_ = st.UpsertSessionTelemetry(context.Background(), routine.SessionTelemetryRecord{
		SessionID: "codex-live-1", Agent: "codex", Model: "gpt-codex", StartedAt: now - 120, UpdatedAt: now - 30, TurnsCount: 5,
	})

	cfg := &config.Config{
		VaultPath: tempVault,
		Workspace: config.WorkspaceConfig{WorkDir: workDir},
	}
	srv := &GraphServer{
		engine:       eng,
		vaultDir:     tempVault,
		cfg:          cfg,
		routineStore: st,
	}

	dashResp, err := srv.GetDashboardSummary(context.Background(), connect.NewRequest(&graphv1.GetDashboardSummaryRequest{Days: 7}))
	if err != nil {
		t.Fatalf("GetDashboardSummary failed: %v", err)
	}
	if dashResp.Msg.TotalHours != 8.0 {
		t.Errorf("expected 8.0 total hours, got %.1f", dashResp.Msg.TotalHours)
	}
	if len(dashResp.Msg.Taxonomy) == 0 || dashResp.Msg.Taxonomy[0].Percentage != 50.0 {
		t.Errorf("expected first taxonomy percentage 50.0, got %+v", dashResp.Msg.Taxonomy)
	}
	if len(dashResp.Msg.BacklogItems) != 2 {
		t.Errorf("expected 2 backlog items, got %d", len(dashResp.Msg.BacklogItems))
	}

	wsResp, err := srv.GetWorkspaceOverview(context.Background(), connect.NewRequest(&graphv1.GetWorkspaceOverviewRequest{}))
	if err != nil {
		t.Fatalf("GetWorkspaceOverview failed: %v", err)
	}
	if !wsResp.Msg.FromCache || wsResp.Msg.WorkDir != workDir || wsResp.Msg.TotalRepos != 1 {
		t.Errorf("unexpected cached workspace response: %+v", wsResp.Msg)
	}

	peopleResp, err := srv.GetPeopleDirectory(context.Background(), connect.NewRequest(&graphv1.GetPeopleDirectoryRequest{}))
	if err != nil {
		t.Fatalf("GetPeopleDirectory failed: %v", err)
	}
	if len(peopleResp.Msg.People) != 1 || peopleResp.Msg.People[0].Slug != "alice-silva" {
		t.Fatalf("unexpected people directory: %+v", peopleResp.Msg.People)
	}

	briefResp, err := srv.GetPersonBriefing(context.Background(), connect.NewRequest(&graphv1.GetPersonBriefingRequest{Slug: "alice-silva"}))
	if err != nil {
		t.Fatalf("GetPersonBriefing failed: %v", err)
	}
	if briefResp.Msg.Cargo != "Staff Engineer" {
		t.Errorf("expected cargo Staff Engineer, got %q", briefResp.Msg.Cargo)
	}

	actResp, err := srv.GetAgentActivity(context.Background(), connect.NewRequest(&graphv1.GetAgentActivityRequest{Hours: 24}))
	if err != nil {
		t.Fatalf("GetAgentActivity failed: %v", err)
	}
	for _, s := range actResp.Msg.Sessions {
		if s.SessionId == "history" {
			t.Fatalf("synthetic 'history' session must be filtered out")
		}
	}
	foundCodex := false
	for _, s := range actResp.Msg.Sessions {
		if s.SessionId == "codex-live-1" && s.IsActive {
			foundCodex = true
		}
	}
	if !foundCodex {
		t.Fatalf("expected codex-live-1 active session, got %+v", actResp.Msg.Sessions)
	}
}
