package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
