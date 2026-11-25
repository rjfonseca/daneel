package tui

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"connectrpc.com/connect"

	graphv1 "github.com/rjfonseca/daneel/api/v1"
	"github.com/rjfonseca/daneel/api/v1/graphv1connect"
	"github.com/rjfonseca/daneel/internal/config"
	"github.com/rjfonseca/daneel/internal/daemon"
	"github.com/rjfonseca/daneel/internal/ops/overview"
	"github.com/rjfonseca/daneel/internal/routine"
	"github.com/rjfonseca/daneel/internal/search"
	"github.com/rjfonseca/daneel/internal/spooler"
	"github.com/rjfonseca/daneel/internal/telemetry"
)

type tuiSurfaceTransport struct {
	base http.RoundTripper
}

func (t tuiSurfaceTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r2 := r.Clone(r.Context())
	r2.Header.Set("X-Daneel-Surface", "tui")
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(r2)
}

// Backend coordinates TUI data access, preferring the running daemon ConnectRPC API
// with automatic fallback to local vault/SQLite operations when offline.
type Backend struct {
	vaultRoot   string
	cfg         *config.Config
	rpcClient   graphv1connect.GraphServiceClient
	daemonAddr  string
	localEngine *search.Engine
	spool       *spooler.Spooler
}

// ConnectBackend probes the local daemon IPC socket and wires a ConnectRPC thin client when online.
func ConnectBackend(vaultRoot string, cfg *config.Config, localEngine *search.Engine) *Backend {
	b := &Backend{
		vaultRoot:   vaultRoot,
		cfg:         cfg,
		localEngine: localEngine,
		spool:       spooler.New(""),
	}

	probeCtx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	client, err := daemon.NewClientContext(probeCtx)
	if err != nil {
		return b
	}
	defer func() { _ = client.Close() }()

	info, err := client.GetServerInfoContext(probeCtx)
	if err != nil || info == nil || strings.TrimSpace(info.HTTPAddr) == "" {
		return b
	}
	if info.VaultPath != "" && vaultRoot != "" && filepath.Clean(info.VaultPath) != filepath.Clean(vaultRoot) {
		return b
	}

	b.SetRPCClient(newDaemonRPCClient(info.HTTPAddr), info.HTTPAddr)
	return b
}

// NewLocalBackend constructs an offline/test backend without probing the host daemon socket.
func NewLocalBackend(vaultRoot string, cfg *config.Config, localEngine *search.Engine) *Backend {
	return &Backend{
		vaultRoot:   vaultRoot,
		cfg:         cfg,
		localEngine: localEngine,
		spool:       spooler.New(""),
	}
}

func newDaemonRPCClient(baseURL string) graphv1connect.GraphServiceClient {
	httpClient := &http.Client{
		Timeout:   60 * time.Second,
		Transport: tuiSurfaceTransport{base: http.DefaultTransport},
	}
	return graphv1connect.NewGraphServiceClient(httpClient, baseURL)
}

// SetRPCClient injects a ConnectRPC client (used for daemon connections and hermetic tests).
func (b *Backend) SetRPCClient(client graphv1connect.GraphServiceClient, addr string) {
	b.rpcClient = client
	b.daemonAddr = addr
}

// IsDaemonConnected returns true when the TUI is talking to a running daemon HTTP/ConnectRPC endpoint.
func (b *Backend) IsDaemonConnected() bool {
	return b != nil && b.rpcClient != nil
}

// Config returns the backend's loaded configuration (loading lazily from vaultRoot if needed).
func (b *Backend) Config() *config.Config {
	if b.cfg == nil && b.vaultRoot != "" {
		if loaded, err := config.LoadConfig(b.vaultRoot, "", false); err == nil {
			b.cfg = loaded
		}
	}
	return b.cfg
}

// WorkDir returns the configured workspace directory (e.g., ~/Projects).
func (b *Backend) WorkDir() string {
	if cfg := b.Config(); cfg != nil {
		if wd := cfg.GetWorkDir(); wd != "" {
			return wd
		}
	}
	home, _ := os.UserHomeDir()
	projects := filepath.Join(home, "Projects")
	if info, err := os.Stat(projects); err == nil && info.IsDir() {
		return projects
	}
	return filepath.Join(home, "Work")
}

// SearchNotes queries the daemon ConnectRPC search endpoint or falls back to local FTS5.
func (b *Backend) SearchNotes(ctx context.Context, query, scope string, limit int) ([]search.SearchResult, error) {
	if b.rpcClient != nil {
		reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		resp, err := b.rpcClient.SearchNotes(reqCtx, connect.NewRequest(&graphv1.SearchNotesRequest{
			Query: query,
			Limit: int32(limit), //nolint:gosec
			Scope: scope,
		}))
		cancel()
		if err == nil && resp != nil {
			out := make([]search.SearchResult, 0, len(resp.Msg.Results))
			for _, r := range resp.Msg.Results {
				out = append(out, search.SearchResult{
					RelPath:     r.RelPath,
					Title:       r.Title,
					Snippet:     r.Snippet,
					Scope:       r.Scope,
					Type:        r.NoteType,
					Description: r.Description,
				})
			}
			return out, nil
		}
	}

	if b.localEngine == nil {
		return nil, nil
	}
	return b.localEngine.Search(ctx, query, scope, limit)
}

// GetNote loads markdown note content via daemon or direct vault read.
func (b *Backend) GetNote(ctx context.Context, relPath string) (string, error) {
	if b.rpcClient != nil {
		reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		resp, err := b.rpcClient.GetNote(reqCtx, connect.NewRequest(&graphv1.GetNoteRequest{
			RelPath: relPath,
		}))
		cancel()
		if err == nil && resp != nil {
			return resp.Msg.ContentMarkdown, nil
		}
	}

	absPath := filepath.Join(b.vaultRoot, relPath)
	data, err := os.ReadFile(absPath) //nolint:gosec // intentional vault path
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// GetDashboardSummary fetches unified dashboard telemetry via daemon or local overview builder.
func (b *Backend) GetDashboardSummary(ctx context.Context, days int) (*graphv1.GetDashboardSummaryResponse, error) {
	if b.rpcClient != nil {
		reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		resp, err := b.rpcClient.GetDashboardSummary(reqCtx, connect.NewRequest(&graphv1.GetDashboardSummaryRequest{
			Days: int32(days), //nolint:gosec
		}))
		cancel()
		if err == nil && resp != nil {
			return resp.Msg, nil
		}
	}
	return overview.BuildDashboardSummary(ctx, b.vaultRoot, b.Config(), b.localEngine, days), nil
}

// GetWorkspaceOverview fetches workspace repositories via daemon cache or live local scan.
func (b *Backend) GetWorkspaceOverview(ctx context.Context, forceRescan bool) (*graphv1.GetWorkspaceOverviewResponse, error) {
	if b.rpcClient != nil {
		timeout := 10 * time.Second
		if forceRescan {
			timeout = 60 * time.Second
		}
		reqCtx, cancel := context.WithTimeout(ctx, timeout)
		resp, err := b.rpcClient.GetWorkspaceOverview(reqCtx, connect.NewRequest(&graphv1.GetWorkspaceOverviewRequest{
			ForceRescan: forceRescan,
		}))
		cancel()
		if err == nil && resp != nil {
			return resp.Msg, nil
		}
	}
	return overview.BuildWorkspaceOverview(ctx, b.vaultRoot, b.Config(), b.localEngine, forceRescan)
}

// GetPeopleDirectory fetches the lightweight colleague directory via daemon or local scan.
func (b *Backend) GetPeopleDirectory(ctx context.Context) (*graphv1.GetPeopleDirectoryResponse, error) {
	if b.rpcClient != nil {
		reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		resp, err := b.rpcClient.GetPeopleDirectory(reqCtx, connect.NewRequest(&graphv1.GetPeopleDirectoryRequest{}))
		cancel()
		if err == nil && resp != nil {
			return resp.Msg, nil
		}
	}
	return overview.BuildPeopleDirectory(ctx, b.vaultRoot, b.localEngine)
}

// GetPersonBriefing fetches a 1:1 briefing for slug via daemon or local generator.
func (b *Backend) GetPersonBriefing(ctx context.Context, slug string) (*graphv1.GetPersonBriefingResponse, error) {
	if b.rpcClient != nil {
		reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		resp, err := b.rpcClient.GetPersonBriefing(reqCtx, connect.NewRequest(&graphv1.GetPersonBriefingRequest{
			Slug: slug,
		}))
		cancel()
		if err == nil && resp != nil {
			return resp.Msg, nil
		}
	}
	return overview.BuildPersonBriefing(b.vaultRoot, slug)
}

// GetAgentActivity fetches unified agent sessions and spool events via daemon or local store.
func (b *Backend) GetAgentActivity(ctx context.Context, hours int, sessionID string) (*graphv1.GetAgentActivityResponse, error) {
	if b.rpcClient != nil {
		reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		resp, err := b.rpcClient.GetAgentActivity(reqCtx, connect.NewRequest(&graphv1.GetAgentActivityRequest{
			Hours:     int32(hours), //nolint:gosec
			SessionId: sessionID,
		}))
		cancel()
		if err == nil && resp != nil {
			return resp.Msg, nil
		}
	}

	var st routine.Store
	if opened, err := routine.OpenDefaultStore(b.vaultRoot); err == nil && opened != nil {
		st = opened
		defer func() { _ = opened.Close() }()
	}
	spoolDir := ""
	if b.spool != nil {
		spoolDir = b.spool.BaseDir()
	}
	return overview.BuildAgentActivity(ctx, b.vaultRoot, b.Config(), st, spoolDir, hours, sessionID)
}

// GetOperationalTelemetry fetches FinOps & operational telemetry via daemon or local store.
func (b *Backend) GetOperationalTelemetry(ctx context.Context, days int) (*graphv1.GetOperationalTelemetryResponse, error) {
	if days <= 0 {
		days = 7
	}
	if b.rpcClient != nil {
		reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		resp, err := b.rpcClient.GetOperationalTelemetry(reqCtx, connect.NewRequest(&graphv1.GetOperationalTelemetryRequest{
			Days: int32(days), //nolint:gosec
		}))
		cancel()
		if err == nil && resp != nil {
			return resp.Msg, nil
		}
	}

	st, err := routine.OpenDefaultStore(b.vaultRoot)
	if err != nil || st == nil {
		return nil, err
	}
	defer func() { _ = st.Close() }()

	sinceDuration := time.Duration(days) * 24 * time.Hour
	sinceUnix := time.Now().Add(-sinceDuration).Unix()

	syncCtx, cancel := context.WithTimeout(ctx, time.Second)
	_, _ = telemetry.SyncSessionsToStore(syncCtx, st, telemetry.ParserOptions{
		Since:  sinceDuration,
		Config: b.Config(),
	})
	cancel()

	resp := &graphv1.GetOperationalTelemetryResponse{
		WindowDays: int32(days), //nolint:gosec
	}
	if cfg := b.Config(); cfg != nil {
		fin := cfg.GetFinOpsConfig()
		resp.DailyBudgetUsd = fin.DailyBudgetUSD
		resp.MonthlyBudgetUsd = fin.MonthlyBudgetUSD
	}

	if modelAggs, err := st.GetModelUsageAggregates(ctx, sinceUnix); err == nil {
		for _, m := range modelAggs {
			resp.ModelUsage = append(resp.ModelUsage, &graphv1.ModelUsageItem{
				Model:            m.Model,
				Agent:            m.Agent,
				SessionsCount:    int32(m.SessionsCount),  //nolint:gosec
				SubagentsCount:   int32(m.SubagentsCount), //nolint:gosec
				InputTokens:      m.InputTokens,
				CacheReadTokens:  m.CacheReadTokens,
				OutputTokens:     m.OutputTokens,
				EstimatedCostUsd: m.EstimatedCostUSD,
			})
		}
	}
	if rws, err := st.GetRoutineWeightAggregates(ctx, sinceUnix); err == nil {
		for _, rw := range rws {
			resp.RoutineWeights = append(resp.RoutineWeights, &graphv1.RoutineWeightItem{
				RoutineName:       rw.RoutineName,
				TotalRuns:         int32(rw.TotalRuns),   //nolint:gosec
				SuccessRuns:       int32(rw.SuccessRuns), //nolint:gosec
				FailedRuns:        int32(rw.FailedRuns),  //nolint:gosec
				AvgDurationMs:     rw.AvgDurationMs,
				MaxDurationMs:     rw.MaxDurationMs,
				TotalItems:        rw.TotalItems,
				TotalInputTokens:  rw.TotalInputTokens,
				TotalCacheTokens:  rw.TotalCacheTokens,
				TotalOutputTokens: rw.TotalOutputTokens,
				TotalCostUsd:      rw.TotalCostUSD,
				LastRunAt:         rw.LastRunAt,
			})
		}
	}
	if sess, err := st.GetRecentSessionsTelemetry(ctx, sinceUnix, 100); err == nil {
		for _, s := range sess {
			resp.RecentSessions = append(resp.RecentSessions, &graphv1.SessionTelemetryItem{
				SessionId:        s.SessionID,
				ParentSessionId:  s.ParentSessionID,
				Agent:            s.Agent,
				Model:            s.Model,
				Role:             s.Role,
				StartedAt:        s.StartedAt,
				TurnsCount:       int32(s.TurnsCount),     //nolint:gosec
				SubagentsCount:   int32(s.SubagentsCount), //nolint:gosec
				InputTokens:      s.InputTokens,
				CacheReadTokens:  s.CacheReadTokens,
				OutputTokens:     s.OutputTokens,
				EstimatedCostUsd: s.EstimatedCostUSD,
				SkillsUsed:       s.SkillsUsed,
				IsRoutine:        s.IsRoutine,
				RoutineName:      s.RoutineName,
			})
		}
	}
	if cmds, err := st.GetCommandAggregates(ctx, sinceUnix, 100); err == nil {
		for _, c := range cmds {
			resp.CommandMetrics = append(resp.CommandMetrics, &graphv1.CommandMetricItem{
				Surface:       c.Surface,
				Command:       c.Command,
				TotalCalls:    int32(c.TotalCalls), //nolint:gosec
				ErrorCalls:    int32(c.ErrorCalls), //nolint:gosec
				AvgDurationMs: c.AvgDurationMs,
				MaxDurationMs: c.MaxDurationMs,
				LastCalledAt:  c.LastCalledAt,
			})
		}
	}

	var totalIn, totalCache, totalOut int64
	var totalCost float64
	for _, s := range resp.RecentSessions {
		totalIn += s.InputTokens
		totalCache += s.CacheReadTokens
		totalOut += s.OutputTokens
		totalCost += s.EstimatedCostUsd
	}
	for _, rw := range resp.RoutineWeights {
		totalIn += rw.TotalInputTokens
		totalCache += rw.TotalCacheTokens
		totalOut += rw.TotalOutputTokens
		totalCost += rw.TotalCostUsd
	}
	resp.TotalInputTokens = totalIn
	resp.TotalCacheReadTokens = totalCache
	resp.TotalOutputTokens = totalOut
	resp.TotalCostUsd = totalCost
	if totalIn+totalCache > 0 {
		resp.CacheEfficiencyPct = (float64(totalCache) / float64(totalIn+totalCache)) * 100.0
	}
	return resp, nil
}

// ReportActiveContext asynchronously notifies the daemon of the active TUI tab/note.
func (b *Backend) ReportActiveContext(surface, notePath, focusedDomain string) {
	if b == nil || b.rpcClient == nil {
		return
	}
	client := b.rpcClient
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
		defer cancel()
		_, _ = client.ReportActiveContext(ctx, connect.NewRequest(&graphv1.ReportActiveContextRequest{
			Surface:        surface,
			ActiveNotePath: notePath,
			FocusedDomain:  focusedDomain,
		}))
	}()
}
