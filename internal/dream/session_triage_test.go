package dream

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTriageSessionLogs(t *testing.T) {
	tempVault := t.TempDir()
	sessionsDir := filepath.Join(tempVault, "inbox", "sessions")
	if err := os.MkdirAll(sessionsDir, 0750); err != nil {
		t.Fatalf("failed to create sessions dir: %v", err)
	}

	sessionContent := `---
type: session_log
description: "Agent session log for antigravity in /home/user/work/project"
agent: "antigravity"
directory: "/home/user/work/project"
duration_minutes: 35
events_count: 12
session_id: "session-xyz-123"
date: "2026-10-05"
---

<untrusted_content>
## Command & Tool Summary
- Executed ` + "`go test ./...`" + `
- Executed ` + "`git commit -m 'feat: add feature'`" + `

## Modified Files
- ` + "`internal/feature.go`" + `

## Event Timeline
- ` + "`[14:00:00]` **SessionStart** (antigravity)" + `
</untrusted_content>
`

	sessionPath := filepath.Join(sessionsDir, "2026-10-05-14-00-antigravity.md")
	if err := os.WriteFile(sessionPath, []byte(sessionContent), 0600); err != nil {
		t.Fatalf("failed to write session file: %v", err)
	}

	triaged := TriageSessionLogs(tempVault)
	if len(triaged) != 1 {
		t.Fatalf("expected 1 triaged session, got %d", len(triaged))
	}

	s := triaged[0]
	if s.Agent != "antigravity" {
		t.Errorf("expected agent 'antigravity', got %q", s.Agent)
	}
	if s.Directory != "/home/user/work/project" {
		t.Errorf("expected directory '/home/user/work/project', got %q", s.Directory)
	}
	if s.DurationMinutes != 35 {
		t.Errorf("expected duration 35, got %d", s.DurationMinutes)
	}
	if len(s.Commands) != 2 {
		t.Errorf("expected 2 commands, got %d", len(s.Commands))
	}
	if len(s.ModifiedFiles) != 1 {
		t.Errorf("expected 1 modified file, got %d", len(s.ModifiedFiles))
	}
}
