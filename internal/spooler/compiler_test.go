package spooler_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rjfonseca/daneel/internal/spooler"
)

func TestCompiler_CompileSession(t *testing.T) {
	tempCacheDir := t.TempDir()
	tempVaultDir := t.TempDir()

	sp := spooler.New(tempCacheDir)

	// Create a realistic session with 3 events
	startTime := time.Date(2026, 10, 5, 14, 0, 0, 0, time.UTC)
	endTime := startTime.Add(24 * time.Minute)

	p1, _ := json.Marshal(map[string]interface{}{
		"conversationId": "session-compiler-test-001",
		"cwd":            "/home/user/work/myproject",
		"agent":          "antigravity",
	})
	_, err := sp.AppendEvent(spooler.Event{
		SessionID: "session-compiler-test-001",
		Event:     "SessionStart",
		Agent:     "antigravity",
		CWD:       "/home/user/work/myproject",
		Raw:       p1,
		Timestamp: startTime,
	})
	if err != nil {
		t.Fatalf("failed to append SessionStart: %v", err)
	}

	p2, _ := json.Marshal(map[string]interface{}{
		"tool":    "bash",
		"command": "npm test",
		"result":  "PASS 4 tests",
	})
	_, err = sp.AppendEvent(spooler.Event{
		SessionID: "session-compiler-test-001",
		Event:     "PostToolUse",
		Agent:     "antigravity",
		CWD:       "/home/user/work/myproject",
		Raw:       p2,
		Timestamp: startTime.Add(10 * time.Minute),
	})
	if err != nil {
		t.Fatalf("failed to append PostToolUse: %v", err)
	}

	p3, _ := json.Marshal(map[string]interface{}{
		"tool": "write_file",
		"path": "src/index.ts",
	})
	_, err = sp.AppendEvent(spooler.Event{
		SessionID: "session-compiler-test-001",
		Event:     "SessionEnd",
		Agent:     "antigravity",
		CWD:       "/home/user/work/myproject",
		Raw:       p3,
		Timestamp: endTime,
	})
	if err != nil {
		t.Fatalf("failed to append SessionEnd: %v", err)
	}

	// Instantiate compiler
	c := spooler.NewCompiler(tempVaultDir, sp)
	mdPath, err := c.CompileSession("session-compiler-test-001")
	if err != nil {
		t.Fatalf("CompileSession failed: %v", err)
	}

	// 1. Verify file exists in inbox/sessions/
	expectedDir := filepath.Join(tempVaultDir, "inbox", "sessions")
	if !strings.HasPrefix(mdPath, expectedDir) {
		t.Errorf("expected file path to be inside %q, got %q", expectedDir, mdPath)
	}

	contentBytes, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatalf("failed to read generated markdown: %v", err)
	}
	content := string(contentBytes)

	// 2. Verify Frontmatter OKF rules
	if !strings.Contains(content, "type: session_log") {
		t.Errorf("frontmatter missing 'type: session_log'")
	}
	if !strings.Contains(content, `agent: "antigravity"`) && !strings.Contains(content, "agent: antigravity") {
		t.Errorf("frontmatter missing agent name")
	}
	if !strings.Contains(content, "directory: \"/home/user/work/myproject\"") && !strings.Contains(content, "directory: /home/user/work/myproject") {
		t.Errorf("frontmatter missing directory")
	}
	if !strings.Contains(content, "duration_minutes: 24") {
		t.Errorf("frontmatter missing duration_minutes: 24")
	}

	// 3. Verify Untrusted Content Wrapping (Security Invariant)
	if !strings.Contains(content, "<untrusted_content>") {
		t.Errorf("content missing opening <untrusted_content> tag")
	}
	if !strings.Contains(content, "</untrusted_content>") {
		t.Errorf("content missing closing </untrusted_content> tag")
	}

	// 4. Verify tool & command summary
	if !strings.Contains(content, "npm test") {
		t.Errorf("content missing command 'npm test'")
	}
	if !strings.Contains(content, "src/index.ts") {
		t.Errorf("content missing modified file 'src/index.ts'")
	}

	// 5. Verify spool file marked as compiled
	compiledSpool := filepath.Join(tempCacheDir, "session-compiler-test-001.jsonl.compiled")
	if _, err := os.Stat(compiledSpool); err != nil {
		t.Errorf("expected spool file to be archived to %q, but got error: %v", compiledSpool, err)
	}
}

func TestCompileSessionContext_CancelledPreservesSpoolAndAllowsRetry(t *testing.T) {
	for _, tc := range []struct {
		name string
		ctx  func() (context.Context, context.CancelFunc)
	}{
		{"canceled", func() (context.Context, context.CancelFunc) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			return ctx, cancel
		}},
		{"expired deadline", func() (context.Context, context.CancelFunc) {
			return context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sp, compiler, spoolPath, vault := compilerFixture(t, "cancel-session")
			ctx, cancel := tc.ctx()
			defer cancel()
			path, err := compiler.CompileSessionContext(ctx, "cancel-session")
			if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("CompileSessionContext error = %v, want context cancellation", err)
			}
			if path != "" {
				t.Fatalf("canceled compilation returned path %q", path)
			}
			assertRawOnly(t, spoolPath, filepath.Join(vault, "inbox", "sessions"))
			if _, err := os.Stat(spoolPath + ".compiled"); !os.IsNotExist(err) {
				t.Fatalf("canceled session was archived: %v", err)
			}
			if _, err := compiler.CompileSession("cancel-session"); err != nil {
				t.Fatalf("unrestricted retry failed: %v", err)
			}
			if _, err := os.Stat(spoolPath + ".compiled"); err != nil {
				t.Fatalf("successful retry did not archive raw session: %v", err)
			}
			_ = sp
		})
	}
}

func TestCompileSessionContext_ScannerFailurePreservesRawSession(t *testing.T) {
	sp := spooler.New(t.TempDir())
	vault := t.TempDir()
	spoolPath := filepath.Join(sp.BaseDir(), "scanner-failure.jsonl")
	if err := os.MkdirAll(sp.BaseDir(), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(spoolPath, []byte(strings.Repeat("x", 70*1024)+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	compiler := spooler.NewCompiler(vault, sp)
	if _, err := compiler.CompileSession("scanner-failure"); err == nil || !strings.Contains(err.Error(), "scan") {
		t.Fatalf("CompileSession error = %v, want scanner error", err)
	}
	assertRawOnly(t, spoolPath, filepath.Join(vault, "inbox", "sessions"))
}

func TestCompileSessionContext_PublicationFailurePreservesRawSession(t *testing.T) {
	_, compiler, spoolPath, vault := compilerFixture(t, "publish-failure")
	destDir := filepath.Join(vault, "inbox", "sessions")
	if err := os.MkdirAll(destDir, 0750); err != nil {
		t.Fatal(err)
	}
	finalPath := filepath.Join(destDir, "2026-10-05-14-00-agent.md")
	if err := os.Mkdir(finalPath, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := compiler.CompileSession("publish-failure"); err == nil {
		t.Fatal("CompileSession succeeded while final output path was a directory")
	}
	if _, err := os.Stat(spoolPath); err != nil {
		t.Fatalf("publication failure removed raw session: %v", err)
	}
	if entries, err := os.ReadDir(destDir); err != nil || len(entries) != 1 || entries[0].Name() != filepath.Base(finalPath) {
		t.Fatalf("temporary output was not cleaned up: entries=%v err=%v", entries, err)
	}
}

func TestCompileSessionContext_TemporaryOutputFailurePreservesRawSession(t *testing.T) {
	sp := spooler.New(t.TempDir())
	vault := t.TempDir()
	const sessionID = "temp-output-failure"
	// Exceed the filesystem's filename limit at CreateTemp without relying on
	// platform-specific directories or permissions that privileged users bypass.
	_, err := sp.AppendEvent(spooler.Event{
		SessionID: sessionID,
		Event:     "SessionEnd",
		Agent:     strings.Repeat("a", 256),
		Timestamp: time.Date(2026, 10, 5, 14, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	spoolPath := filepath.Join(sp.BaseDir(), sessionID+".jsonl")
	rawBefore, err := os.ReadFile(spoolPath)
	if err != nil {
		t.Fatal(err)
	}
	compiler := spooler.NewCompiler(vault, sp)
	path, err := compiler.CompileSession(sessionID)
	if err == nil || !strings.Contains(err.Error(), "failed to create temporary compiled markdown") {
		t.Fatalf("CompileSession error = %v, want temporary output creation failure", err)
	}
	if path != "" {
		t.Fatalf("failed compilation returned path %q", path)
	}
	rawAfter, err := os.ReadFile(spoolPath)
	if err != nil || string(rawAfter) != string(rawBefore) {
		t.Fatalf("temporary output failure changed raw session: %v", err)
	}
	if _, err := os.Stat(spoolPath + ".compiled"); !os.IsNotExist(err) {
		t.Fatalf("temporary output failure archived raw session: %v", err)
	}
	if entries, err := os.ReadDir(filepath.Join(vault, "inbox", "sessions")); err != nil || len(entries) != 0 {
		t.Fatalf("temporary output failure left output: entries=%v err=%v", entries, err)
	}
}

func TestCompileSessionContext_ArchiveFailureIsObservableAndRetainsRaw(t *testing.T) {
	_, compiler, spoolPath, vault := compilerFixture(t, "archive-failure")
	if err := os.Mkdir(spoolPath+".compiled", 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := compiler.CompileSession("archive-failure"); err == nil {
		t.Fatal("CompileSession succeeded despite archive failure")
	}
	if _, err := os.Stat(spoolPath); err != nil {
		t.Fatalf("archive failure removed raw session: %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(vault, "inbox", "sessions"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("published output missing after archive failure: entries=%d err=%v", len(entries), err)
	}
}

func compilerFixture(t *testing.T, sessionID string) (*spooler.Spooler, *spooler.Compiler, string, string) {
	t.Helper()
	cache, vault := t.TempDir(), t.TempDir()
	sp := spooler.New(cache)
	_, err := sp.AppendEvent(spooler.Event{SessionID: sessionID, Event: "SessionEnd", Agent: "agent", CWD: "/tmp/project", Raw: []byte(`{"command":"go test"}`), Timestamp: time.Date(2026, 10, 5, 14, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	return sp, spooler.NewCompiler(vault, sp), filepath.Join(cache, sessionID+".jsonl"), vault
}

func assertRawOnly(t *testing.T, spoolPath, sessionsDir string) {
	t.Helper()
	if _, err := os.Stat(spoolPath); err != nil {
		t.Fatalf("raw session missing: %v", err)
	}
	if _, err := os.Stat(spoolPath + ".compiled"); !os.IsNotExist(err) {
		t.Fatalf("unexpected archive: %v", err)
	}
	if entries, err := os.ReadDir(sessionsDir); err == nil && len(entries) > 0 {
		t.Fatalf("unexpected published output: %v", entries)
	}
}

func TestCompiler_CompileAll(t *testing.T) {
	tempCacheDir := t.TempDir()
	tempVaultDir := t.TempDir()

	sp := spooler.New(tempCacheDir)

	for _, sid := range []string{"sess-all-1", "sess-all-2"} {
		_, err := sp.AppendEvent(spooler.Event{
			SessionID: sid,
			Event:     "SessionEnd",
			Agent:     "test-agent",
			CWD:       "/tmp/test",
			Raw:       []byte(`{}`),
			Timestamp: time.Now(),
		})
		if err != nil {
			t.Fatalf("failed to append event: %v", err)
		}
	}

	c := spooler.NewCompiler(tempVaultDir, sp)
	compiledFiles, err := c.CompileAll()
	if err != nil {
		t.Fatalf("CompileAll failed: %v", err)
	}

	if len(compiledFiles) != 2 {
		t.Errorf("expected 2 compiled files, got %d", len(compiledFiles))
	}
}

func TestCompiler_CompileSessionSupportsLargeValidPayload(t *testing.T) {
	sp, compiler, _, _ := compilerFixture(t, "large-payload")
	largePrompt := strings.Repeat("a", 128*1024)
	if _, err := sp.AppendEvent(spooler.Event{SessionID: "large-payload", Event: "PostToolUse", Agent: "agent", Raw: mustJSON(t, map[string]string{"prompt": largePrompt}), Timestamp: time.Date(2026, 10, 5, 14, 1, 0, 0, time.UTC)}); err != nil {
		t.Fatal(err)
	}
	path, err := compiler.CompileSession("large-payload")
	if err != nil {
		t.Fatalf("CompileSession failed for valid payload larger than scanner default: %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), largePrompt) {
		t.Fatal("compiled note did not preserve the large prompt")
	}
}

func TestCompiler_CompileSession_IncludesModelAndTokensInFrontmatter(t *testing.T) {
	tempCacheDir := t.TempDir()
	tempVaultDir := t.TempDir()

	sp := spooler.New(tempCacheDir)
	startTime := time.Date(2026, 10, 5, 14, 0, 0, 0, time.UTC)

	p1, _ := json.Marshal(map[string]interface{}{
		"model":         "gemini-flash",
		"input_tokens":  12000,
		"output_tokens": 1500,
	})
	_, err := sp.AppendEvent(spooler.Event{
		SessionID: "session-finops-frontmatter",
		Event:     "SessionStart",
		Agent:     "antigravity",
		CWD:       "/tmp/test",
		Raw:       p1,
		Timestamp: startTime,
	})
	if err != nil {
		t.Fatalf("failed to append event: %v", err)
	}

	c := spooler.NewCompiler(tempVaultDir, sp)
	mdPath, err := c.CompileSession("session-finops-frontmatter")
	if err != nil {
		t.Fatalf("CompileSession failed: %v", err)
	}

	contentBytes, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatalf("failed to read markdown: %v", err)
	}
	content := string(contentBytes)

	if !strings.Contains(content, `model: "gemini-flash"`) {
		t.Errorf("expected frontmatter to contain model: \"gemini-flash\", got:\n%s", content)
	}
	if !strings.Contains(content, "input_tokens: 12000") {
		t.Errorf("expected frontmatter to contain input_tokens: 12000, got:\n%s", content)
	}
	if !strings.Contains(content, "output_tokens: 1500") {
		t.Errorf("expected frontmatter to contain output_tokens: 1500, got:\n%s", content)
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
