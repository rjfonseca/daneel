package spooler

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type cancelOnCheckContext struct {
	context.Context
	checks   int
	cancelAt int
}

func (c *cancelOnCheckContext) Err() error {
	c.checks++
	if c.checks >= c.cancelAt {
		return context.Canceled
	}
	return nil
}

func TestRenderSessionNote_CancellationDuringTimeline(t *testing.T) {
	ctx := &cancelOnCheckContext{Context: context.Background(), cancelAt: 2}
	_, err := renderSessionNote(ctx, "render-session", []SpoolEntry{
		{Timestamp: time.Now().Format(time.RFC3339Nano), Event: "SessionStart", Agent: "agent"},
		{Timestamp: time.Now().Format(time.RFC3339Nano), Event: "SessionEnd", Agent: "agent"},
	}, "agent", "/tmp/project", "", time.Now(), 1, nil, nil, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("renderSessionNote error = %v, want cancellation during event timeline", err)
	}
	if ctx.checks != 2 {
		t.Fatalf("context checks = %d, want cancellation at second check", ctx.checks)
	}
}

func TestCompileSessionContext_CancellationDuringScanPreservesExistingOutput(t *testing.T) {
	cache, vault := t.TempDir(), t.TempDir()
	sp := New(cache)
	started := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)
	_, err := sp.AppendEvent(Event{SessionID: "scan-cancel", Event: "SessionEnd", Agent: "agent", CWD: "/tmp/project", Raw: []byte(`{"command":"test"}`), Timestamp: started})
	if err != nil {
		t.Fatal(err)
	}
	spoolPath := filepath.Join(cache, "scan-cancel.jsonl")
	destDir := filepath.Join(vault, "inbox", "sessions")
	if err := os.MkdirAll(destDir, 0750); err != nil {
		t.Fatal(err)
	}
	finalPath := filepath.Join(destDir, "2026-10-19-00-00-agent.md")
	previous := []byte("previous summary")
	if err := os.WriteFile(finalPath, previous, 0600); err != nil {
		t.Fatal(err)
	}
	ctx := &cancelOnCheckContext{Context: context.Background(), cancelAt: 2}
	if _, err := NewCompiler(vault, sp).CompileSessionContext(ctx, "scan-cancel"); !errors.Is(err, context.Canceled) {
		t.Fatalf("CompileSessionContext error = %v, want cancellation during scan", err)
	}
	content, err := os.ReadFile(finalPath)
	if err != nil || string(content) != string(previous) {
		t.Fatalf("pre-existing output changed: content=%q err=%v", content, err)
	}
	if _, err := os.Stat(spoolPath); err != nil {
		t.Fatalf("canceled scan removed raw session: %v", err)
	}
	if entries, err := os.ReadDir(destDir); err != nil || len(entries) != 1 {
		t.Fatalf("canceled scan left temporary output: entries=%v err=%v", entries, err)
	}
}
