package daemon

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	graphv1 "github.com/rjfonseca/daneel/api/v1"
	"github.com/rjfonseca/daneel/internal/spooler"
)

func TestSocketPaths(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	sockPath := SocketPath()
	pidPath := PIDPath()

	if filepath.Base(sockPath) != "daneel.sock" {
		t.Errorf("unexpected socket path: %s", sockPath)
	}
	if filepath.Base(pidPath) != "daneel.pid" {
		t.Errorf("unexpected pid path: %s", pidPath)
	}
}

func TestHubPubSub(t *testing.T) {
	hub := NewHub()

	ch, unsub := hub.SubscribeUIEvents()

	go func() {
		time.Sleep(10 * time.Millisecond)
		hub.PublishUIEvent(&graphv1.UIEvent{
			Action: graphv1.UIActionType_ACTION_OPEN_NOTE,
		})
	}()

	select {
	case evt := <-ch:
		if evt.Action != graphv1.UIActionType_ACTION_OPEN_NOTE {
			t.Errorf("unexpected event action: %v", evt.Action)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}

	unsub()

	// Should not panic or block
	hub.PublishUIEvent(&graphv1.UIEvent{
		Action: graphv1.UIActionType_ACTION_FOCUS_SECTOR,
	})
	hub.PublishUIEvent(&graphv1.UIEvent{
		Action:     graphv1.UIActionType_ACTION_PIN_NOTE,
		TargetPath: "notes/core.md",
	})
	hub.PublishUIEvent(&graphv1.UIEvent{
		Action:     graphv1.UIActionType_ACTION_UNPIN_NOTE,
		TargetPath: "notes/core.md",
	})
}

func TestHubActiveContext(t *testing.T) {
	hub := NewHub()
	ctx := ActiveContext{
		Surface:       SurfaceWeb,
		FocusedDomain: "notes/arquitetura",
		FocusedSector: "notes/arquitetura",
	}

	hub.SetActiveContext(ctx)

	retrieved := hub.GetActiveContext()
	if retrieved.Surface != SurfaceWeb {
		t.Errorf("expected %s, got %s", SurfaceWeb, retrieved.Surface)
	}
	if retrieved.FocusedDomain != "notes/arquitetura" {
		t.Errorf("expected notes/arquitetura, got %s", retrieved.FocusedDomain)
	}
	if retrieved.FocusedSector != "notes/arquitetura" {
		t.Errorf("expected notes/arquitetura, got %s", retrieved.FocusedSector)
	}
	if retrieved.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be set")
	}
}

func TestSocketServerClient(t *testing.T) {
	// Use a short sub-directory to avoid Unix socket path length limit on macOS / Darwin
	tempDir, err := os.MkdirTemp("", "dnl*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()
	t.Setenv("XDG_STATE_HOME", tempDir)
	hub := NewHub()
	server := NewServer(hub)

	go func() {
		_ = server.ListenAndServe()
	}()
	defer server.Stop()

	// wait for server to start
	running := false
	for i := 0; i < 50; i++ {
		if IsDaemonRunning() {
			running = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !running {
		t.Fatal("expected daemon to be running")
	}

	client, err := NewClient()
	if err != nil {
		t.Fatalf("failed to connect client: %v", err)
	}
	defer func() { _ = client.Close() }()

	if err := client.Ping(); err != nil {
		t.Fatalf("ping failed: %v", err)
	}

	err = client.SetActiveContext(ActiveContext{
		Surface: SurfaceTUI,
	})
	if err != nil {
		t.Fatalf("set active context failed: %v", err)
	}

	ctx, err := client.GetActiveContext()
	if err != nil {
		t.Fatalf("get active context failed: %v", err)
	}
	if ctx.Surface != SurfaceTUI {
		t.Errorf("expected %s, got %s", SurfaceTUI, ctx.Surface)
	}

	subCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := client.SubscribeUIEvents(subCtx)
	if err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}

	go func() {
		time.Sleep(50 * time.Millisecond)
		pubClient, _ := NewClient()
		defer func() { _ = pubClient.Close() }()
		_ = pubClient.PublishUIEvent(int32(graphv1.UIActionType_ACTION_OPEN_NOTE), "path/to/note", "{}")
	}()

	select {
	case evt := <-ch:
		if evt.Action != graphv1.UIActionType_ACTION_OPEN_NOTE {
			t.Errorf("unexpected event action: %v", evt.Action)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for UI event")
	}
}

func TestMultiListenerServer(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "dnl_multi*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()
	t.Setenv("XDG_STATE_HOME", tempDir)

	hub := NewHub()
	server := NewServer(hub)

	// Create test HTTP listener
	httpLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen tcp: %v", err)
	}

	httpMux := http.NewServeMux()
	httpMux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	httpSrv := &http.Server{
		Handler:           httpMux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	server.SetHTTPServer(httpSrv)

	go func() {
		_ = server.ListenAndServe()
	}()
	go func() {
		_ = httpSrv.Serve(httpLn)
	}()

	// Wait for socket to be ready
	running := false
	for i := 0; i < 50; i++ {
		if IsDaemonRunning() {
			running = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !running {
		t.Fatal("expected daemon to be running")
	}

	// 1. Verify Unix socket works
	client, err := NewClient()
	if err != nil {
		t.Fatalf("failed to connect to daemon unix socket: %v", err)
	}
	if err := client.Ping(); err != nil {
		_ = client.Close()
		t.Fatalf("daemon ping failed: %v", err)
	}
	_ = client.Close()

	// 2. Verify HTTP endpoint works
	httpAddr := httpLn.Addr().String()
	resp, err := http.Get(fmt.Sprintf("http://%s/healthz", httpAddr))
	if err != nil {
		t.Fatalf("HTTP get failed: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", resp.StatusCode)
	}

	// 3. Clean coordinated Stop
	server.Stop()

	// Verify HTTP server stopped
	_, err = http.Get(fmt.Sprintf("http://%s/healthz", httpAddr))
	if err == nil {
		t.Errorf("expected error connecting to stopped HTTP server")
	}

	// Verify Unix socket stopped
	if IsDaemonRunning() {
		t.Errorf("expected daemon to be stopped")
	}
}

func TestDaemon_HookCaptureAndHubPublish(t *testing.T) {
	tempStateDir, err := os.MkdirTemp("", "dnl_hook_state*")
	if err != nil {
		t.Fatalf("failed to create temp state dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempStateDir) }()
	t.Setenv("XDG_STATE_HOME", tempStateDir)

	tempSpoolDir, err := os.MkdirTemp("", "dnl_hook_spool*")
	if err != nil {
		t.Fatalf("failed to create temp spool dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempSpoolDir) }()

	hub := NewHub()
	server := NewServer(hub)
	spoolInstance := spooler.New(tempSpoolDir)
	server.SetSpooler(spoolInstance)

	go func() {
		_ = server.ListenAndServe()
	}()
	defer server.Stop()

	// Wait for daemon
	for i := 0; i < 50; i++ {
		if IsDaemonRunning() {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	client, err := NewClient()
	if err != nil {
		t.Fatalf("failed to connect to daemon: %v", err)
	}
	defer func() { _ = client.Close() }()

	// Subscribe to UI events on a separate connection
	subClient, err := NewClient()
	if err != nil {
		t.Fatalf("failed to connect subClient: %v", err)
	}
	defer func() { _ = subClient.Close() }()

	subCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	evtCh, err := subClient.SubscribeUIEvents(subCtx)
	if err != nil {
		t.Fatalf("failed to subscribe to ui events: %v", err)
	}

	// Capture hook event
	payload := HookEventPayload{
		SessionID: "sess-daemon-test-1",
		Event:     "PostToolUse",
		Agent:     "antigravity",
		CWD:       "/home/user/myproject",
		Payload:   `{"tool": "bash", "command": "git status"}`,
	}

	sessionID, err := client.CaptureHookEvent(payload)
	if err != nil {
		t.Fatalf("CaptureHookEvent failed: %v", err)
	}
	if sessionID != "sess-daemon-test-1" {
		t.Errorf("expected sessionID 'sess-daemon-test-1', got %q", sessionID)
	}

	// Verify event was saved to spool on disk
	spoolFile := filepath.Join(tempSpoolDir, "sess-daemon-test-1.jsonl")
	if _, err := os.Stat(spoolFile); err != nil {
		t.Fatalf("expected spool file %q to exist: %v", spoolFile, err)
	}

	// Verify event was broadcast through Hub
	select {
	case uiEvt := <-evtCh:
		if uiEvt.TargetPath != "/home/user/myproject" {
			t.Errorf("expected target path '/home/user/myproject', got %q", uiEvt.TargetPath)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for UIEvent on Hub channel")
	}
}

func TestDaemon_HookHandoff(t *testing.T) {
	tempStateDir, err := os.MkdirTemp("", "dnl_handoff_state*")
	if err != nil {
		t.Fatalf("failed to create temp state dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempStateDir) }()
	t.Setenv("XDG_STATE_HOME", tempStateDir)

	tempSpool := t.TempDir()
	sp := spooler.New(tempSpool)

	hub := NewHub()
	server := NewServer(hub)
	server.SetSpooler(sp)
	go func() {
		_ = server.ListenAndServe()
	}()
	defer server.Stop()

	for i := 0; i < 50; i++ {
		if IsDaemonRunning() {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	client, err := NewClient()
	if err != nil {
		t.Fatalf("failed to connect to daemon: %v", err)
	}
	defer func() { _ = client.Close() }()

	targetDir := "/home/user/myproject"

	// 1. Without sessions, handoff returns empty string safely
	handoff, err := client.GetHookHandoff(targetDir)
	if err != nil {
		t.Fatalf("GetHookHandoff returned error: %v", err)
	}
	if handoff != "" {
		t.Errorf("expected empty handoff when no sessions exist, got %q", handoff)
	}

	// 2. With session data in spool, handoff returns continuity context
	_, err = sp.AppendEvent(spooler.Event{
		SessionID: "sess-daemon-test",
		Event:     "ToolUse",
		Agent:     "claude-code",
		CWD:       targetDir,
		Raw:       []byte(`{"command": "make test"}`),
		Timestamp: time.Now(),
	})
	if err != nil {
		t.Fatalf("failed to append event: %v", err)
	}

	handoff, err = client.GetHookHandoff(targetDir)
	if err != nil {
		t.Fatalf("GetHookHandoff returned error: %v", err)
	}
	if !strings.Contains(handoff, "claude-code") || !strings.Contains(handoff, "make test") {
		t.Errorf("expected handoff to contain agent and command, got %q", handoff)
	}
}

func setupClientSocketFixture(t *testing.T) {
	t.Helper()
	// Avoid including long test names in Unix socket paths on macOS.
	dir, err := os.MkdirTemp("", "dnl*")
	if err != nil {
		t.Fatalf("create socket state directory: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	t.Setenv("XDG_STATE_HOME", dir)
}

func TestClientContext_CaptureHookEventCancellationInterruptsRead(t *testing.T) {
	setupClientSocketFixture(t)
	listener, err := net.Listen("unix", SocketPath())
	if err != nil {
		t.Fatalf("listen on fixture socket: %v", err)
	}
	defer func() { _ = listener.Close() }()
	serverDone := make(chan struct{})
	requestRead := make(chan struct{})
	go func() {
		defer close(serverDone)
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		_, _ = bufio.NewReader(conn).ReadBytes('\n')
		close(requestRead)
		buf := make([]byte, 1)
		_, _ = conn.Read(buf)
	}()
	t.Cleanup(func() { _ = listener.Close(); <-serverDone })

	ctx, cancel := context.WithCancel(context.Background())
	client, err := NewClientContext(ctx)
	if err != nil {
		t.Fatalf("connect with context: %v", err)
	}
	result := make(chan error, 1)
	go func() {
		_, requestErr := client.CaptureHookEventContext(ctx, HookEventPayload{SessionID: "canceled-session", Event: "PostToolUse"})
		result <- requestErr
	}()
	<-requestRead
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("request error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("context cancellation did not interrupt the blocked response read")
	}
	_ = client.Close()
}

func TestClientContext_DeadlineInterruptsRead(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer func() { _ = clientConn.Close() }()
	defer func() { _ = serverConn.Close() }()
	client := &Client{conn: clientConn}
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		_, _ = bufio.NewReader(serverConn).ReadBytes('\n')
		buf := make([]byte, 1)
		_, _ = serverConn.Read(buf)
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err := client.sendRequestContext(ctx, "ping", nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("request error = %v, want context.DeadlineExceeded", err)
	}
	if time.Since(started) > time.Second {
		t.Fatal("request exceeded deadline by more than scheduling margin")
	}
	_ = clientConn.Close()
	<-serverDone
}

func TestClientContext_CaptureHookEventSuccessfulResponse(t *testing.T) {
	setupClientSocketFixture(t)
	listener, err := net.Listen("unix", SocketPath())
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
		_, _ = conn.Write([]byte("{\"result\":\"context-session\"}\n"))
	}()
	t.Cleanup(func() { _ = listener.Close(); <-serverDone })

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	client, err := NewClientContext(ctx)
	if err != nil {
		t.Fatalf("connect with context: %v", err)
	}
	defer func() { _ = client.Close() }()
	sid, err := client.CaptureHookEventContext(ctx, HookEventPayload{SessionID: "context-session", Event: "PostToolUse"})
	if err != nil {
		t.Fatalf("capture with context: %v", err)
	}
	if sid != "context-session" {
		t.Fatalf("session ID = %q, want context-session", sid)
	}
}
