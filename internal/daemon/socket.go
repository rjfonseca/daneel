package daemon

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	graphv1 "github.com/rjfonseca/daneel/api/v1"
	"github.com/rjfonseca/daneel/internal/spooler"
)

func getStateDir() string {
	stateHome := os.Getenv("XDG_STATE_HOME")
	if stateHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "/tmp"
		}
		stateHome = filepath.Join(home, ".local", "state")
	}
	dir := filepath.Join(stateHome, "daneel")
	_ = os.MkdirAll(dir, 0700) //nolint:gosec // standard state dir
	return dir
}

// ShortSocketPath creates a safe, short path for Unix domain sockets (max 104-108 chars).
func ShortSocketPath(dir string) string {
	_ = os.MkdirAll(dir, 0700)
	return filepath.Join(dir, "d.sock")
}

// SocketPath returns the standard path for the daneel unix domain socket.
func SocketPath() string {
	return filepath.Join(getStateDir(), "daneel.sock")
}

// PIDPath returns the standard path for the daneel daemon PID file.
func PIDPath() string {
	return filepath.Join(getStateDir(), "daneel.pid")
}

// IsDaemonRunning checks if the Unix socket is alive by trying to connect to it.
func IsDaemonRunning() bool {
	conn, err := net.DialTimeout("unix", SocketPath(), 100*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// Server is a Unix domain socket server and optional coordinated HTTP server.
type Server struct {
	listener net.Listener
	hub      *Hub
	httpSrv  *http.Server
	spooler  *spooler.Spooler
	vaultDir string
}

// NewServer creates a new IPC Server.
func NewServer(hub *Hub) *Server {
	return &Server{hub: hub}
}

// SetHTTPServer attaches an HTTP server to the daemon server for coordinated shutdown.
func (s *Server) SetHTTPServer(srv *http.Server) {
	s.httpSrv = srv
}

// SetSpooler attaches a spooler to the daemon server.
func (s *Server) SetSpooler(sp *spooler.Spooler) {
	s.spooler = sp
}

// SetVaultDir sets the vault directory for the daemon server.
func (s *Server) SetVaultDir(dir string) {
	s.vaultDir = dir
}

// ListenAndServe starts the Unix socket server. It cleans up any existing socket file.
func (s *Server) ListenAndServe() error {
	sockPath := SocketPath()
	if err := os.Remove(sockPath); err != nil && !os.IsNotExist(err) {
		return err
	}

	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		return err
	}
	s.listener = ln
	defer func() { _ = s.listener.Close() }()

	for {
		conn, err := ln.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			continue
		}
		go s.handleConnection(conn)
	}
}

// Stop cleanly terminates the server listener and any attached HTTP server.
func (s *Server) Stop() {
	if s.httpSrv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = s.httpSrv.Shutdown(ctx)
	}
	if s.listener != nil {
		_ = s.listener.Close()
	}
	_ = os.Remove(SocketPath())
}

func (s *Server) handleConnection(conn net.Conn) {
	defer func() { _ = conn.Close() }()

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		line := scanner.Text()
		var req Request
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			s.sendResponse(conn, Response{Error: "invalid request"})
			continue
		}

		s.dispatch(conn, req)
	}
}

func (s *Server) dispatch(conn net.Conn, req Request) {
	var resp Response

	switch req.Method {
	case "ping":
		resp.Result = "pong"
		s.sendResponse(conn, resp)
	case "get_active_context":
		resp.Result = s.hub.GetActiveContext()
		s.sendResponse(conn, resp)
	case "set_active_context":
		b, _ := json.Marshal(req.Params)
		var ctx ActiveContext
		if err := json.Unmarshal(b, &ctx); err != nil {
			resp.Error = "invalid params"
		} else {
			s.hub.SetActiveContext(ctx)
			resp.Result = "ok"
		}
		s.sendResponse(conn, resp)
	case "publish_ui_event":
		b, _ := json.Marshal(req.Params)
		var payload UIEventPayload
		if err := json.Unmarshal(b, &payload); err != nil {
			resp.Error = "invalid params"
		} else {
			evt := &graphv1.UIEvent{
				Action:      graphv1.UIActionType(payload.Action),
				TargetPath:  payload.TargetPath,
				PayloadJson: payload.PayloadJSON,
				Timestamp:   time.Now().UnixMilli(),
			}
			s.hub.PublishUIEvent(evt)
			resp.Result = "ok"
		}
		s.sendResponse(conn, resp)
	case "subscribe_ui_events":
		// Streaming endpoint: register subscriber BEFORE acknowledging to avoid race conditions
		ch, unsub := s.hub.SubscribeUIEvents()
		defer unsub()

		resp.Result = "subscribed"
		s.sendResponse(conn, resp) // ack subscription

		for evt := range ch {
			b, _ := json.Marshal(evt)
			if _, err := conn.Write(append(b, '\n')); err != nil {
				return // client disconnected
			}
		}
	case "exec_task":
		// Handle generic task execution - triggering galaxy reload immediately for now
		s.hub.BroadcastGalaxyReload()
		resp.Result = "ok"
		s.sendResponse(conn, resp)
	case "capture_hook_event":
		b, _ := json.Marshal(req.Params)
		var payload HookEventPayload
		if err := json.Unmarshal(b, &payload); err != nil {
			resp.Error = "invalid params"
		} else {
			if s.spooler == nil {
				s.spooler = spooler.New("")
			}
			sid, err := s.spooler.AppendEvent(spooler.Event{
				SessionID: payload.SessionID,
				Event:     payload.Event,
				Agent:     payload.Agent,
				CWD:       payload.CWD,
				GitRoot:   payload.GitRoot,
				Raw:       []byte(payload.Payload),
				Timestamp: time.Now(),
			})
			if err != nil {
				resp.Error = err.Error()
			} else {
				resp.Result = sid
				// Publish UIEvent to Hub for Web and TUI ambient awareness
				s.hub.PublishUIEvent(&graphv1.UIEvent{
					Action:      graphv1.UIActionType_ACTION_FLASH_NOTIFICATION,
					TargetPath:  payload.CWD,
					PayloadJson: payload.Payload,
					Timestamp:   time.Now().UnixMilli(),
				})
			}
		}
		s.sendResponse(conn, resp)
	case "get_hook_handoff":
		cwd := ""
		b, _ := json.Marshal(req.Params)
		var p map[string]interface{}
		if err := json.Unmarshal(b, &p); err == nil {
			if c, ok := p["cwd"].(string); ok {
				cwd = c
			}
		}
		if s.spooler != nil {
			handoff, _ := s.spooler.ResolveHandoffContext(s.vaultDir, cwd)
			resp.Result = handoff
		} else {
			resp.Result = ""
		}
		s.sendResponse(conn, resp)
	default:
		resp.Error = "unknown method"
		s.sendResponse(conn, resp)
	}
}

func (s *Server) sendResponse(conn net.Conn, resp Response) {
	b, err := json.Marshal(resp)
	if err == nil {
		_, _ = conn.Write(append(b, '\n'))
	}
}

// Client represents a connection to the daemon.
type Client struct {
	conn net.Conn
}

// NewClient connects to the running daemon.
func NewClient() (*Client, error) {
	return NewClientContext(context.Background())
}

// NewClientContext connects to the daemon while honoring the caller's cancellation.
// The normal one-second connection timeout remains in effect when the context has no earlier deadline.
func NewClientContext(ctx context.Context) (*Client, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	dialer := net.Dialer{Timeout: time.Second}
	conn, err := dialer.DialContext(ctx, "unix", SocketPath())
	if err != nil {
		return nil, err
	}
	return &Client{conn: conn}, nil
}

// Close closes the client connection.
func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) sendRequest(method string, params interface{}) (Response, error) {
	return c.sendRequestContext(context.Background(), method, params)
}

func (c *Client) sendRequestContext(ctx context.Context, method string, params interface{}) (Response, error) {
	if err := ctx.Err(); err != nil {
		return Response{}, err
	}
	req := Request{Method: method, Params: params}
	b, _ := json.Marshal(req)

	deadline := time.Now().Add(5 * time.Second)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	if err := c.conn.SetDeadline(deadline); err != nil {
		return Response{}, err
	}
	watchDone := make(chan struct{})
	watchExited := make(chan struct{})
	go func() {
		defer close(watchExited)
		select {
		case <-ctx.Done():
			_ = c.conn.SetDeadline(time.Now())
		case <-watchDone:
		}
	}()
	defer func() {
		close(watchDone)
		<-watchExited
		_ = c.conn.SetDeadline(time.Time{})
	}()

	if _, err := c.conn.Write(append(b, '\n')); err != nil {
		if ctxErr := contextIOError(ctx); ctxErr != nil {
			return Response{}, ctxErr
		}
		return Response{}, err
	}

	reader := bufio.NewReader(c.conn)
	line, err := reader.ReadBytes('\n')
	if err != nil {
		if ctxErr := contextIOError(ctx); ctxErr != nil {
			return Response{}, ctxErr
		}
		return Response{}, err
	}

	var resp Response
	if err := json.Unmarshal(line, &resp); err != nil {
		return Response{}, err
	}
	if resp.Error != "" {
		return Response{}, errors.New(resp.Error)
	}

	return resp, nil
}

func contextIOError(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if deadline, ok := ctx.Deadline(); ok && !time.Now().Before(deadline) {
		return context.DeadlineExceeded
	}
	return nil
}

// Ping checks if daemon is responsive.
func (c *Client) Ping() error {
	resp, err := c.sendRequest("ping", nil)
	if err != nil {
		return err
	}
	if resp.Result != "pong" {
		return fmt.Errorf("unexpected ping response: %v", resp.Result)
	}
	return nil
}

// GetActiveContext retrieves current active context from daemon.
func (c *Client) GetActiveContext() (*ActiveContext, error) {
	resp, err := c.sendRequest("get_active_context", nil)
	if err != nil {
		return nil, err
	}
	b, _ := json.Marshal(resp.Result)
	var ctx ActiveContext
	if err := json.Unmarshal(b, &ctx); err != nil {
		return nil, err
	}
	return &ctx, nil
}

// SetActiveContext updates the active context on daemon.
func (c *Client) SetActiveContext(ctx ActiveContext) error {
	_, err := c.sendRequest("set_active_context", ctx)
	return err
}

// PublishUIEvent publishes a UI event to the daemon.
func (c *Client) PublishUIEvent(action int32, targetPath, payload string) error {
	_, err := c.sendRequest("publish_ui_event", UIEventPayload{
		Action:      action,
		TargetPath:  targetPath,
		PayloadJSON: payload,
	})
	return err
}

// SubscribeUIEvents subscribes to UI events stream.
func (c *Client) SubscribeUIEvents(ctx context.Context) (<-chan *graphv1.UIEvent, error) {
	req := Request{Method: "subscribe_ui_events"}
	b, _ := json.Marshal(req)

	// Don't set deadline for subscribe as it's long lived
	_ = c.conn.SetDeadline(time.Time{})
	if _, err := c.conn.Write(append(b, '\n')); err != nil {
		return nil, err
	}

	reader := bufio.NewReader(c.conn)
	// read ack
	line, err := reader.ReadBytes('\n')
	if err != nil {
		return nil, err
	}

	var resp Response
	if err := json.Unmarshal(line, &resp); err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return nil, errors.New(resp.Error)
	}

	ch := make(chan *graphv1.UIEvent, 100)
	go func() {
		defer close(ch)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			// We need a way to interrupt ReadBytes if ctx is done, but standard net.Conn blocking read is tricky.
			// Set a short deadline in a loop to check ctx.
			_ = c.conn.SetReadDeadline(time.Now().Add(1 * time.Second))
			line, err := reader.ReadBytes('\n')
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				return
			}

			var evt graphv1.UIEvent
			if err := json.Unmarshal(line, &evt); err == nil {
				select {
				case <-ctx.Done():
					return
				case ch <- &evt:
				}
			}
		}
	}()

	return ch, nil
}

// CaptureHookEvent sends an agent hook event to the daemon.
func (c *Client) CaptureHookEvent(payload HookEventPayload) (string, error) {
	return c.CaptureHookEventContext(context.Background(), payload)
}

// CaptureHookEventContext sends an agent hook event using the caller's IPC budget.
func (c *Client) CaptureHookEventContext(ctx context.Context, payload HookEventPayload) (string, error) {
	resp, err := c.sendRequestContext(ctx, "capture_hook_event", payload)
	if err != nil {
		return "", err
	}
	if resp.Error != "" {
		return "", errors.New(resp.Error)
	}
	if sid, ok := resp.Result.(string); ok {
		return sid, nil
	}
	return "", nil
}

// GetHookHandoff queries the daemon for current handoff context.
func (c *Client) GetHookHandoff(cwd string) (string, error) {
	return c.GetHookHandoffContext(context.Background(), cwd)
}

// GetHookHandoffContext queries handoff context using the caller's IPC budget.
func (c *Client) GetHookHandoffContext(ctx context.Context, cwd string) (string, error) {
	resp, err := c.sendRequestContext(ctx, "get_hook_handoff", map[string]string{"cwd": cwd})
	if err != nil {
		return "", err
	}
	if resp.Error != "" {
		return "", errors.New(resp.Error)
	}
	if handoff, ok := resp.Result.(string); ok {
		return handoff, nil
	}
	return "", nil
}
