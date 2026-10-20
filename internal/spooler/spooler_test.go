package spooler_test

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/rjfonseca/daneel/internal/spooler"
)

func TestSpooler_AppendEvent(t *testing.T) {
	tempDir := t.TempDir()
	s := spooler.New(tempDir)

	payload := map[string]interface{}{
		"conversationId": "session-antigravity-1234",
		"toolCall": map[string]interface{}{
			"tool": "view_file",
			"args": map[string]interface{}{"path": "internal/daemon/socket.go"},
		},
		"cwd": "/home/user/work/project",
	}
	rawJSON, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	event := spooler.Event{
		Event:     "ToolUse",
		Agent:     "antigravity",
		Raw:       rawJSON,
		Timestamp: time.Now(),
	}

	sessionID, err := s.AppendEvent(event)
	if err != nil {
		t.Fatalf("AppendEvent failed: %v", err)
	}
	if sessionID != "session-antigravity-1234" {
		t.Errorf("expected sessionID 'session-antigravity-1234', got '%s'", sessionID)
	}

	// Verify file on disk
	spoolFile := filepath.Join(tempDir, "session-antigravity-1234.jsonl")
	data, err := os.ReadFile(spoolFile)
	if err != nil {
		t.Fatalf("failed to read spool file: %v", err)
	}

	var entry spooler.SpoolEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Fatalf("failed to unmarshal spool entry: %v", err)
	}

	if entry.SessionID != "session-antigravity-1234" {
		t.Errorf("expected entry.SessionID 'session-antigravity-1234', got '%s'", entry.SessionID)
	}
	if entry.Agent != "antigravity" {
		t.Errorf("expected entry.Agent 'antigravity', got '%s'", entry.Agent)
	}
	if entry.Event != "ToolUse" {
		t.Errorf("expected entry.Event 'ToolUse', got '%s'", entry.Event)
	}
	if entry.CWD != "/home/user/work/project" {
		t.Errorf("expected entry.CWD '/home/user/work/project', got '%s'", entry.CWD)
	}
}

func TestSpooler_SessionResolution(t *testing.T) {
	tempDir := t.TempDir()
	s := spooler.New(tempDir)

	tests := []struct {
		name        string
		payload     string
		explicitSID string
		wantSID     string
	}{
		{
			name:        "Explicit session ID flag has highest priority",
			payload:     `{"conversationId": "from-body"}`,
			explicitSID: "explicit-session-999",
			wantSID:     "explicit-session-999",
		},
		{
			name:        "Antigravity protojson conversationId",
			payload:     `{"conversationId": "antigravity-conv-abc"}`,
			explicitSID: "",
			wantSID:     "antigravity-conv-abc",
		},
		{
			name:        "Claude Code camelCase sessionId",
			payload:     `{"sessionId": "claude-session-xyz"}`,
			explicitSID: "",
			wantSID:     "claude-session-xyz",
		},
		{
			name:        "Generic snake_case session_id",
			payload:     `{"session_id": "generic-session-123"}`,
			explicitSID: "",
			wantSID:     "generic-session-123",
		},
		{
			name:        "Nested info.id",
			payload:     `{"info": {"id": "nested-info-id"}}`,
			explicitSID: "",
			wantSID:     "nested-info-id",
		},
		{
			name:        "Fallback deterministic hash when no ID is provided",
			payload:     `{"message": "no session anywhere", "cwd": "/tmp/test"}`,
			explicitSID: "",
			wantSID:     "", // non-empty deterministic fallback
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sid := s.ResolveSessionID(tt.explicitSID, []byte(tt.payload), "/tmp/test")
			if tt.wantSID != "" && sid != tt.wantSID {
				t.Errorf("got session ID %q, want %q", sid, tt.wantSID)
			}
			if tt.wantSID == "" && sid == "" {
				t.Errorf("expected non-empty fallback session ID, got empty string")
			}
		})
	}
}

func TestSpooler_DirectoryAndGitResolution(t *testing.T) {
	tempDir := t.TempDir()
	s := spooler.New(tempDir)

	// Create a dummy git repo directory
	repoDir := filepath.Join(tempDir, "myrepo")
	// Include a backslash to exercise JSON path escaping on every host.
	subDir := filepath.Join(repoDir, "cmd", `sub\project`)
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0750); err != nil {
		t.Fatalf("failed to create fake .git: %v", err)
	}
	if err := os.MkdirAll(subDir, 0750); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	payload, err := json.Marshal(map[string][]string{"workspacePaths": {subDir}})
	if err != nil {
		t.Fatalf("failed to marshal workspace paths: %v", err)
	}
	resolvedCWD, gitRoot := s.ResolveCWDAndGit("", payload)

	if resolvedCWD != subDir {
		t.Errorf("expected CWD %q, got %q", subDir, resolvedCWD)
	}
	if gitRoot != repoDir {
		t.Errorf("expected GitRoot %q, got %q", repoDir, gitRoot)
	}
}

func TestSpooler_ConcurrentAppends(t *testing.T) {
	tempDir := t.TempDir()
	s := spooler.New(tempDir)
	sessionID := "concurrent-session"

	const goroutines = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			payload := map[string]interface{}{
				"idx":  idx,
				"time": time.Now().Format(time.RFC3339Nano),
			}
			raw, _ := json.Marshal(payload)
			evt := spooler.Event{
				SessionID: sessionID,
				Event:     "ToolUse",
				Agent:     "bench-agent",
				Raw:       raw,
				Timestamp: time.Now(),
			}
			if _, err := s.AppendEvent(evt); err != nil {
				t.Errorf("goroutine %d failed to append: %v", idx, err)
			}
		}(i)
	}

	wg.Wait()

	spoolFile := filepath.Join(tempDir, sessionID+".jsonl")
	file, err := os.Open(spoolFile)
	if err != nil {
		t.Fatalf("failed to open spool file: %v", err)
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	lines := 0
	for scanner.Scan() {
		lines++
		var entry spooler.SpoolEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			t.Fatalf("corrupted JSONL on line %d: %v", lines, err)
		}
	}

	if err := scanner.Err(); err != nil {
		t.Fatalf("scanner error: %v", err)
	}

	if lines != goroutines {
		t.Errorf("expected exactly %d lines in spool file, got %d", goroutines, lines)
	}
}

func TestSpooler_ListSessions(t *testing.T) {
	tempDir := t.TempDir()
	s := spooler.New(tempDir)

	for _, sid := range []string{"sess-1", "sess-2"} {
		for i := 0; i < 3; i++ {
			evt := spooler.Event{
				SessionID: sid,
				Event:     "SessionStart",
				Agent:     "test-agent",
				Raw:       []byte(`{}`),
				Timestamp: time.Now(),
			}
			if _, err := s.AppendEvent(evt); err != nil {
				t.Fatalf("failed to append: %v", err)
			}
		}
	}

	sessions, err := s.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions failed: %v", err)
	}

	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}

	for _, sess := range sessions {
		if sess.EventCount != 3 {
			t.Errorf("session %s: expected 3 events, got %d", sess.SessionID, sess.EventCount)
		}
	}
}
