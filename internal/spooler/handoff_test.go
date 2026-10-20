package spooler_test

import (
	"strings"
	"testing"
	"time"

	"github.com/rjfonseca/daneel/internal/spooler"
)

func TestSpooler_ResolveHandoffContext(t *testing.T) {
	tempVault := t.TempDir()
	tempSpool := t.TempDir()
	sp := spooler.New(tempSpool)

	targetCWD := "/home/user/work/myproject"

	// 1. When no sessions exist, returns empty string cleanly
	ctxEmpty, err := sp.ResolveHandoffContext(tempVault, targetCWD)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctxEmpty != "" {
		t.Errorf("expected empty string when no session exists, got %q", ctxEmpty)
	}

	// 2. When active session exists in spool
	_, err = sp.AppendEvent(spooler.Event{
		SessionID: "sess-100",
		Event:     "ToolUse",
		Agent:     "codex",
		CWD:       targetCWD,
		Raw:       []byte(`{"command": "go test ./..."}`),
		Timestamp: time.Now(),
	})
	if err != nil {
		t.Fatalf("failed to append event: %v", err)
	}

	ctxSpool, err := sp.ResolveHandoffContext(tempVault, targetCWD)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(ctxSpool, "codex") {
		t.Errorf("expected context to mention 'codex', got %q", ctxSpool)
	}
	if !strings.Contains(ctxSpool, "go test ./...") {
		t.Errorf("expected context to mention 'go test ./...', got %q", ctxSpool)
	}

	// 3. When compiled session exists in vault inbox/sessions
	compiler := spooler.NewCompiler(tempVault, sp)
	_, err = compiler.CompileSession("sess-100")
	if err != nil {
		t.Fatalf("failed to compile session: %v", err)
	}

	ctxCompiled, err := sp.ResolveHandoffContext(tempVault, targetCWD)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(ctxCompiled, "codex") {
		t.Errorf("expected context to mention 'codex', got %q", ctxCompiled)
	}
	if !strings.Contains(ctxCompiled, "go test ./...") {
		t.Errorf("expected context to mention 'go test ./...', got %q", ctxCompiled)
	}
}
