package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"

	graphv1 "github.com/rjfonseca/daneel/api/v1"
	"github.com/rjfonseca/daneel/internal/spooler"
)

func TestHooksView_Render(t *testing.T) {
	tempCacheDir := t.TempDir()
	_ = os.Setenv("XDG_CACHE_HOME", tempCacheDir)

	spoolDir := filepath.Join(tempCacheDir, "daneel", "spool")
	sp := spooler.New(spoolDir)

	// Append dummy events
	_, err := sp.AppendEvent(spooler.Event{
		SessionID: "session-tui-123",
		Event:     "ToolUse",
		Agent:     "antigravity",
		CWD:       "/home/user/work/daneel",
		Raw:       []byte(`{"tool": "view_file"}`),
		Timestamp: time.Now(),
	})
	if err != nil {
		t.Fatalf("failed to append test event: %v", err)
	}

	styles := NewStyles()
	view := NewHooksView(t.TempDir(), styles)
	view.SetSpooler(sp)
	view.Refresh()
	view.SetSize(120, 40)

	rendered := view.View()

	if !strings.Contains(rendered, "session-tui-123") {
		t.Errorf("expected rendered view to contain 'session-tui-123', got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "antigravity") {
		t.Errorf("expected rendered view to contain agent 'antigravity', got:\n%s", rendered)
	}
}

func TestHooksView_EmptyState(t *testing.T) {
	tempCacheDir := t.TempDir()
	_ = os.Setenv("XDG_CACHE_HOME", tempCacheDir)

	spoolDir := filepath.Join(tempCacheDir, "daneel", "spool")
	sp := spooler.New(spoolDir)

	styles := NewStyles()
	view := NewHooksView(t.TempDir(), styles)
	view.SetSpooler(sp)
	view.Refresh()
	view.SetSize(100, 30)

	rendered := view.View()
	if !strings.Contains(rendered, "No active sessions") && !strings.Contains(rendered, "No events") {
		t.Errorf("expected empty state message in rendered view, got:\n%s", rendered)
	}
}

func TestHooksView_UnifiedSessionsAndLayout(t *testing.T) {
	now := time.Now().Unix()
	view := NewHooksView(t.TempDir(), NewStyles())
	view.SetSize(110, 30)

	view, _ = view.Update(hooksLoadedMsg{
		resp: &graphv1.GetAgentActivityResponse{
			Sessions: []*graphv1.AgentSessionItem{
				{SessionId: "agy-sess-111", Agent: "antigravity", Model: "gemini-pro", UpdatedAt: now - 30, TurnsCount: 12, IsActive: true},
				{SessionId: "claude-sess-222", Agent: "claude-code", Model: "claude-sonnet", UpdatedAt: now - 180, TurnsCount: 4, IsActive: true},
				{SessionId: "codex-sess-333", Agent: "codex", Model: "gpt-codex", UpdatedAt: now - 7200, TurnsCount: 8, IsActive: false},
			},
			Events: []*graphv1.AgentEventItem{
				{Timestamp: "2026-11-25T11:20:00Z", Event: "ToolUse", Agent: "antigravity", Cwd: "/home/user/Projects/daneel"},
			},
		},
	})

	out := view.View()
	for _, want := range []string{"agy", "claude", "codex", "🟢", "⚪", "ToolUse"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in HooksView output, got:\n%s", want, out)
		}
	}
	if gotH := lipgloss.Height(out); gotH > 30 {
		t.Errorf("expected HooksView height <= 30, got %d", gotH)
	}
}
