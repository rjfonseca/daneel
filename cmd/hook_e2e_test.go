package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestHookE2E_FullLifecycle(t *testing.T) {
	tempCacheDir := t.TempDir()
	tempStateDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tempCacheDir)
	t.Setenv("XDG_STATE_HOME", tempStateDir)

	// 1. Session start handoff
	handoff, err := executeHookHandoff("/home/user/work/project")
	if err != nil {
		t.Fatalf("handoff failed: %v", err)
	}
	if handoff != "" {
		t.Errorf("expected clean empty handoff in Phase 1, got %q", handoff)
	}

	// 2. Ingest Antigravity ToolUse event
	agPayload := `{"conversationId": "ag-conv-001", "tool": "view_file", "path": "main.go"}`
	agBuf := bytes.NewBufferString(agPayload)
	sid1, err := executeHookCapture("ToolUse", agBuf, "antigravity", "", "/home/user/work/project", "")
	if err != nil {
		t.Fatalf("capturing antigravity event failed: %v", err)
	}
	if sid1 != "ag-conv-001" {
		t.Errorf("expected session ID 'ag-conv-001', got %q", sid1)
	}

	// 3. Ingest Claude Code PostToolUse event
	claudePayload := `{"sessionId": "claude-session-888", "tool": "Bash", "command": "cargo build"}`
	claudeBuf := bytes.NewBufferString(claudePayload)
	sid2, err := executeHookCapture("PostToolUse", claudeBuf, "claude-code", "", "/home/user/work/project", "")
	if err != nil {
		t.Fatalf("capturing claude-code event failed: %v", err)
	}
	if sid2 != "claude-session-888" {
		t.Errorf("expected session ID 'claude-session-888', got %q", sid2)
	}

	// 4. Verify hook status
	status, err := executeHookStatus()
	if err != nil {
		t.Fatalf("status failed: %v", err)
	}

	if !strings.Contains(status, "Active Sessions: 2") {
		t.Errorf("expected status to report 2 active sessions, got:\n%s", status)
	}
	if !strings.Contains(status, "ag-conv-001") || !strings.Contains(status, "claude-session-888") {
		t.Errorf("expected status to mention both sessions, got:\n%s", status)
	}
}
