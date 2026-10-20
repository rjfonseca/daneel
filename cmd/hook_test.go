package cmd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rjfonseca/daneel/internal/daemon"
	"github.com/rjfonseca/daneel/internal/spooler"
)

func TestHookCapture_DaemonOnline(t *testing.T) {
	tempStateDir := hookSocketStateDir(t)
	tempCacheDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tempStateDir)
	t.Setenv("XDG_CACHE_HOME", tempCacheDir)

	spoolDir := filepath.Join(tempCacheDir, "daneel", "spool")
	spoolInstance := spooler.New(spoolDir)

	hub := daemon.NewHub()
	srv := daemon.NewServer(hub)
	srv.SetSpooler(spoolInstance)

	go func() {
		_ = srv.ListenAndServe()
	}()
	defer srv.Stop()

	// Wait for daemon
	for i := 0; i < 50; i++ {
		if daemon.IsDaemonRunning() {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !daemon.IsDaemonRunning() {
		t.Fatal("fixture daemon did not start")
	}

	payload := `{"sessionId": "test-online-session", "tool": "test"}`
	inBuf := bytes.NewBufferString(payload)

	sid, err := executeHookCapture("ToolUse", inBuf, "", "", "", "")
	if err != nil {
		t.Fatalf("executeHookCapture failed with online daemon: %v", err)
	}
	if sid != "test-online-session" {
		t.Errorf("expected sid 'test-online-session', got %q", sid)
	}

	spoolFile := filepath.Join(spoolDir, "test-online-session.jsonl")
	if _, err := os.Stat(spoolFile); err != nil {
		t.Fatalf("expected spool file %q to exist: %v", spoolFile, err)
	}
}

func TestHookCapture_UnresponsiveDaemonFallsBackWithinBudget(t *testing.T) {
	tempStateDir := hookSocketStateDir(t)
	tempCacheDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tempStateDir)
	t.Setenv("XDG_CACHE_HOME", tempCacheDir)

	listener, err := net.Listen("unix", daemon.SocketPath())
	if err != nil {
		t.Fatalf("listen on fixture socket: %v", err)
	}
	defer func() { _ = listener.Close() }()
	accepted := make(chan struct{})
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		close(accepted)
		buf := make([]byte, 4096)
		for {
			if _, readErr := conn.Read(buf); readErr != nil {
				return
			}
		}
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		<-serverDone
	})

	payload := `{"sessionId":"silent-daemon-session","tool":"build"}`
	started := time.Now()
	sid, err := executeHookCapture("PostToolUse", strings.NewReader(payload), "test-agent", "", "/tmp/project", "")
	elapsed := time.Since(started)
	if err != nil {
		t.Fatalf("capture with silent daemon: %v", err)
	}
	if elapsed >= 750*time.Millisecond {
		t.Fatalf("capture waited %s for an unresponsive daemon, want <750ms", elapsed)
	}
	if sid != "silent-daemon-session" {
		t.Fatalf("session ID = %q, want persisted payload session ID", sid)
	}
	data, err := os.ReadFile(filepath.Join(tempCacheDir, "daneel", "spool", sid+".jsonl"))
	if err != nil {
		t.Fatalf("read fallback spool: %v", err)
	}
	if !strings.Contains(string(data), payload) {
		t.Fatalf("fallback spool does not contain payload: %s", data)
	}
}

func TestHookCapture_DaemonRejectionFallsBackToSpool(t *testing.T) {
	setupHookSocketFixture(t)
	listener, err := net.Listen("unix", daemon.SocketPath())
	if err != nil {
		t.Fatalf("listen on fixture socket: %v", err)
	}
	defer func() { _ = listener.Close() }()
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		_, _ = bufio.NewReader(conn).ReadBytes('\n')
		_, _ = conn.Write([]byte(`{"error":"rejected"}` + "\n"))
	}()
	t.Cleanup(func() { _ = listener.Close(); <-serverDone })

	payload := `{"sessionId":"rejected-session","tool":"test"}`
	sid, err := executeHookCapture("PostToolUse", strings.NewReader(payload), "", "", "", "")
	if err != nil {
		t.Fatalf("capture after daemon rejection: %v", err)
	}
	assertHookSpoolContains(t, sid, payload)
}

func TestHookCapture_LateAcknowledgementPreservesEvent(t *testing.T) {
	tempStateDir := hookSocketStateDir(t)
	tempCacheDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tempStateDir)
	t.Setenv("XDG_CACHE_HOME", tempCacheDir)
	serverSpool := spooler.New(filepath.Join(t.TempDir(), "daemon-spool"))
	listener, err := net.Listen("unix", daemon.SocketPath())
	if err != nil {
		t.Fatalf("listen on fixture socket: %v", err)
	}
	defer func() { _ = listener.Close() }()
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		line, readErr := bufio.NewReader(conn).ReadBytes('\n')
		if readErr != nil {
			return
		}
		var req struct {
			Params daemon.HookEventPayload `json:"params"`
		}
		if json.Unmarshal(line, &req) != nil {
			return
		}
		_, _ = serverSpool.AppendEvent(spooler.Event{SessionID: req.Params.SessionID, Event: req.Params.Event, Agent: req.Params.Agent, CWD: req.Params.CWD, GitRoot: req.Params.GitRoot, Raw: []byte(req.Params.Payload), Timestamp: time.Now()})
		<-time.After(300 * time.Millisecond)
		_, _ = fmt.Fprintf(conn, "{\"result\":%q}\n", req.Params.SessionID)
	}()
	t.Cleanup(func() { _ = listener.Close(); <-serverDone })

	payload := `{"sessionId":"late-ack-session","tool":"test"}`
	sid, err := executeHookCapture("PostToolUse", strings.NewReader(payload), "", "", "", "")
	if err != nil {
		t.Fatalf("capture with delayed acknowledgement: %v", err)
	}
	assertHookSpoolContains(t, sid, payload)
	if _, err := os.Stat(filepath.Join(serverSpool.BaseDir(), sid+".jsonl")); err != nil {
		t.Fatalf("daemon did not persist event before delayed acknowledgement: %v", err)
	}
}

func TestHookCapture_FailsWhenSpoolCannotBeCreated(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	tempCacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tempCacheDir)
	if err := os.WriteFile(filepath.Join(tempCacheDir, "daneel"), []byte("blocked"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := executeHookCapture("PostToolUse", strings.NewReader(`{"sessionId":"no-spool"}`), "", "", "", "")
	if err == nil {
		t.Fatal("capture succeeded without daemon acknowledgement or local spool")
	}
}

func TestHookHandoff_SilentDaemonFallsBackWithinBudget(t *testing.T) {
	setupHookSocketFixture(t)
	listener, err := net.Listen("unix", daemon.SocketPath())
	if err != nil {
		t.Fatalf("listen on fixture socket: %v", err)
	}
	defer func() { _ = listener.Close() }()
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		buf := make([]byte, 512)
		for {
			if _, readErr := conn.Read(buf); readErr != nil {
				return
			}
		}
	}()
	t.Cleanup(func() { _ = listener.Close(); <-serverDone })

	started := time.Now()
	handoff, err := executeHookHandoff("/tmp/project")
	elapsed := time.Since(started)
	if err != nil {
		t.Fatalf("handoff after silent daemon: %v", err)
	}
	if handoff != "" {
		t.Fatalf("handoff = %q, want empty local fallback", handoff)
	}
	if elapsed >= 750*time.Millisecond {
		t.Fatalf("handoff waited %s for an unresponsive daemon, want <750ms", elapsed)
	}
}

func hookSocketStateDir(t *testing.T) string {
	t.Helper()
	// Avoid including long test names in Unix socket paths on macOS.
	dir, err := os.MkdirTemp("", "dnl*")
	if err != nil {
		t.Fatalf("create socket state directory: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

func setupHookSocketFixture(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_STATE_HOME", hookSocketStateDir(t))
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
}

func assertHookSpoolContains(t *testing.T, sid, payload string) {
	t.Helper()
	spoolDir := filepath.Join(os.Getenv("XDG_CACHE_HOME"), "daneel", "spool")
	data, err := os.ReadFile(filepath.Join(spoolDir, sid+".jsonl"))
	if err != nil {
		t.Fatalf("read fallback spool: %v", err)
	}
	if !strings.Contains(string(data), payload) {
		t.Fatalf("spool does not contain payload %q: %s", payload, data)
	}
}

func TestHookCapture_DaemonOfflineFallback(t *testing.T) {
	tempStateDir := t.TempDir()
	tempCacheDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tempStateDir)
	t.Setenv("XDG_CACHE_HOME", tempCacheDir)

	// Explicitly make sure daemon is NOT running
	if daemon.IsDaemonRunning() {
		t.Fatal("daemon should not be running in this test")
	}

	payload := `{"conversationId": "test-offline-session", "action": "test-offline"}`
	inBuf := bytes.NewBufferString(payload)

	sid, err := executeHookCapture("PostToolUse", inBuf, "", "", "", "")
	if err != nil {
		t.Fatalf("executeHookCapture failed with offline daemon: %v", err)
	}
	if sid != "test-offline-session" {
		t.Errorf("expected sid 'test-offline-session', got %q", sid)
	}

	spoolDir := filepath.Join(tempCacheDir, "daneel", "spool")
	spoolFile := filepath.Join(spoolDir, "test-offline-session.jsonl")
	if _, err := os.Stat(spoolFile); err != nil {
		t.Fatalf("expected spool file %q to exist on offline fallback: %v", spoolFile, err)
	}
}

func TestHookHandoff(t *testing.T) {
	tempStateDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tempStateDir)

	out, err := executeHookHandoff("/some/project")
	if err != nil {
		t.Fatalf("executeHookHandoff failed: %v", err)
	}
	if out != "" {
		t.Errorf("expected empty string in Phase 1, got %q", out)
	}
}

func TestHookStatus(t *testing.T) {
	tempCacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tempCacheDir)

	spoolDir := filepath.Join(tempCacheDir, "daneel", "spool")
	sp := spooler.New(spoolDir)

	_, err := sp.AppendEvent(spooler.Event{
		SessionID: "sess-status-test",
		Event:     "SessionStart",
		Agent:     "test",
		Raw:       []byte(`{}`),
		Timestamp: time.Now(),
	})
	if err != nil {
		t.Fatalf("failed to append event: %v", err)
	}

	out, err := executeHookStatus()
	if err != nil {
		t.Fatalf("executeHookStatus failed: %v", err)
	}
	if out == "" {
		t.Errorf("expected status output, got empty string")
	}
}

func TestHookCompileCmd(t *testing.T) {
	tempCacheDir := t.TempDir()
	tempVaultDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tempCacheDir)

	spoolDir := filepath.Join(tempCacheDir, "daneel", "spool")
	sp := spooler.New(spoolDir)

	_, err := sp.AppendEvent(spooler.Event{
		SessionID: "sess-compile-cmd-test",
		Event:     "SessionEnd",
		Agent:     "antigravity",
		CWD:       "/home/user/work/app",
		Raw:       []byte(`{"tool": "bash", "command": "go test ./..."}`),
		Timestamp: time.Now(),
	})
	if err != nil {
		t.Fatalf("failed to append event: %v", err)
	}

	compiledPaths, err := executeHookCompile("sess-compile-cmd-test", false, tempVaultDir)
	if err != nil {
		t.Fatalf("executeHookCompile failed: %v", err)
	}

	if len(compiledPaths) != 1 {
		t.Fatalf("expected 1 compiled file, got %d", len(compiledPaths))
	}

	if _, err := os.Stat(compiledPaths[0]); err != nil {
		t.Errorf("expected compiled markdown %q to exist: %v", compiledPaths[0], err)
	}
}

func TestHookCapture_AutoCompileOnSessionEnd(t *testing.T) {
	tempCacheDir := t.TempDir()
	tempVaultDir := t.TempDir()
	tempStateDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tempCacheDir)
	t.Setenv("XDG_STATE_HOME", tempStateDir)

	// Set temporary vault root in cfg
	oldVaultPath := vaultPath
	vaultPath = tempVaultDir
	defer func() { vaultPath = oldVaultPath }()

	payload := `{"conversationId": "sess-auto-compile-test", "agent": "claude-code", "cwd": "/tmp/proj"}`
	inBuf := bytes.NewBufferString(payload)

	sid, err := executeHookCapture("SessionEnd", inBuf, "claude-code", "", "/tmp/proj", "")
	if err != nil {
		t.Fatalf("executeHookCapture failed: %v", err)
	}
	if sid != "sess-auto-compile-test" {
		t.Errorf("expected sid 'sess-auto-compile-test', got %q", sid)
	}

	// Verify that inbox/sessions was populated automatically
	sessionsDir := filepath.Join(tempVaultDir, "inbox", "sessions")
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		t.Fatalf("failed to read inbox/sessions: %v", err)
	}
	if len(entries) == 0 {
		t.Errorf("expected auto-compiled session markdown in %q, but folder was empty", sessionsDir)
	}
}

func TestHookCapture_CompilationDeferralKeepsCaptureSuccessfulAndPrintsRetry(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	blockedVault := filepath.Join(t.TempDir(), "vault-file")
	if err := os.WriteFile(blockedVault, []byte("blocked"), 0600); err != nil {
		t.Fatal(err)
	}
	oldVaultPath := vaultPath
	vaultPath = blockedVault
	defer func() { vaultPath = oldVaultPath }()

	stderr, err := os.CreateTemp(t.TempDir(), "hook-stderr-*")
	if err != nil {
		t.Fatal(err)
	}
	oldStderr := os.Stderr
	os.Stderr = stderr
	sid, captureErr := executeHookCapture("SessionEnd", strings.NewReader(`{"sessionId":"retry-session"}`), "test-agent", "", "/tmp/project", "")
	os.Stderr = oldStderr
	if closeErr := stderr.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	if captureErr != nil {
		t.Fatalf("successful capture returned compilation error: %v", captureErr)
	}
	if sid != "retry-session" {
		t.Fatalf("session ID = %q, want retry-session", sid)
	}
	message, err := os.ReadFile(stderr.Name())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(message), "Session captured; compilation deferred.") || !strings.Contains(string(message), "daneel hook compile --session retry-session") {
		t.Fatalf("stderr missing concise retry diagnostic: %q", message)
	}
	rawPath := filepath.Join(os.Getenv("XDG_CACHE_HOME"), "daneel", "spool", sid+".jsonl")
	if _, err := os.Stat(rawPath); err != nil {
		t.Fatalf("captured raw session missing: %v", err)
	}
	if _, err := os.Stat(rawPath + ".compiled"); !os.IsNotExist(err) {
		t.Fatalf("failed compilation archived raw session: %v", err)
	}
}
