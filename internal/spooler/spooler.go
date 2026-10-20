package spooler

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rjfonseca/daneel/internal/config"
)

// Spooler manages the low-latency disk spooling of agent lifecycle events.
type Spooler struct {
	baseDir string
	mu      sync.Mutex
	locks   map[string]*sync.Mutex
}

// DefaultDir returns the standard spool directory under XDG cache.
func DefaultDir() string {
	return filepath.Join(config.XDGCacheDir(), "spool")
}

// New creates a new Spooler targeting baseDir. If baseDir is empty, DefaultDir() is used.
func New(baseDir string) *Spooler {
	if baseDir == "" {
		baseDir = DefaultDir()
	}
	return &Spooler{
		baseDir: baseDir,
		locks:   make(map[string]*sync.Mutex),
	}
}

func (s *Spooler) getSessionLock(sessionID string) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()

	l, exists := s.locks[sessionID]
	if !exists {
		l = &sync.Mutex{}
		s.locks[sessionID] = l
	}
	return l
}

// BaseDir returns the root directory where spool files are written.
func (s *Spooler) BaseDir() string {
	return s.baseDir
}

// AppendEvent normalizes and appends an event to the session's .jsonl spool file.
// Returns the resolved session ID and any error encountered.
func (s *Spooler) AppendEvent(event Event) (string, error) {
	if err := os.MkdirAll(s.baseDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create spool directory: %w", err)
	}

	if event.CWD == "" || event.GitRoot == "" {
		cwd, gitRoot := s.ResolveCWDAndGit(event.CWD, event.Raw)
		event.CWD = cwd
		event.GitRoot = gitRoot
	}

	sessionID := s.ResolveSessionID(event.SessionID, event.Raw, event.CWD)
	if sessionID == "" {
		sessionID = "unknown-session"
	}

	if event.Agent == "" {
		event.Agent = s.resolveAgent(event.Raw)
	}

	if event.Event == "" {
		event.Event = "HookEvent"
	}

	ts := event.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}

	entry := SpoolEntry{
		SessionID: sessionID,
		Agent:     event.Agent,
		Event:     event.Event,
		CWD:       event.CWD,
		GitRoot:   event.GitRoot,
		Timestamp: ts.Format(time.RFC3339Nano),
		Payload:   event.Raw,
	}

	line, err := json.Marshal(entry)
	if err != nil {
		return "", fmt.Errorf("failed to serialize spool entry: %w", err)
	}

	lock := s.getSessionLock(sessionID)
	lock.Lock()
	defer lock.Unlock()

	filePath := filepath.Join(s.baseDir, sessionID+".jsonl")
	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return "", fmt.Errorf("failed to open spool file %q: %w", filePath, err)
	}
	defer func() { _ = f.Close() }()

	if _, err := f.Write(append(line, '\n')); err != nil {
		return "", fmt.Errorf("failed to write spool line: %w", err)
	}

	return sessionID, nil
}

// ResolveSessionID extracts session ID using cascading priorities:
// 1. Explicit ID
// 2. Body conversationId (Antigravity protojson)
// 3. Body sessionId / session_id / session (Claude Code / generic)
// 4. Nested info.id
// 5. Deterministic hash fallback based on directory and current date.
func (s *Spooler) ResolveSessionID(explicit string, raw []byte, cwd string) string {
	if explicit != "" {
		return explicit
	}

	if len(raw) > 0 {
		var payload map[string]interface{}
		if err := json.Unmarshal(raw, &payload); err == nil {
			if id, ok := payload["conversationId"].(string); ok && id != "" {
				return id
			}
			if id, ok := payload["sessionId"].(string); ok && id != "" {
				return id
			}
			if id, ok := payload["session_id"].(string); ok && id != "" {
				return id
			}
			if id, ok := payload["session"].(string); ok && id != "" {
				return id
			}
			if info, ok := payload["info"].(map[string]interface{}); ok {
				if id, ok := info["id"].(string); ok && id != "" {
					return id
				}
				if id, ok := info["sessionId"].(string); ok && id != "" {
					return id
				}
			}
		}
	}

	// Deterministic fallback
	dateKey := time.Now().Format("20060102")
	hash := sha256.Sum256([]byte(cwd + ":" + dateKey))
	return fmt.Sprintf("session-%s-%s", dateKey, hex.EncodeToString(hash[:])[:8])
}

// ResolveCWDAndGit extracts CWD from explicit param, JSON payload workspace paths or CWD,
// and identifies the root .git directory if present.
func (s *Spooler) ResolveCWDAndGit(explicit string, raw []byte) (string, string) {
	cwd := explicit
	if cwd == "" && len(raw) > 0 {
		var payload map[string]interface{}
		if err := json.Unmarshal(raw, &payload); err == nil {
			if p, ok := payload["cwd"].(string); ok && p != "" {
				cwd = p
			} else if paths, ok := payload["workspacePaths"].([]interface{}); ok && len(paths) > 0 {
				if first, ok := paths[0].(string); ok && first != "" {
					cwd = first
				}
			} else if roots, ok := payload["workspace_roots"].([]interface{}); ok && len(roots) > 0 {
				if first, ok := roots[0].(string); ok && first != "" {
					cwd = first
				}
			}
		}
	}

	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	// Find git root
	gitRoot := ""
	curr := cwd
	for {
		gitDir := filepath.Join(curr, ".git")
		if info, err := os.Stat(gitDir); err == nil && (info.IsDir() || !info.IsDir()) {
			gitRoot = curr
			break
		}
		parent := filepath.Dir(curr)
		if parent == curr {
			break
		}
		curr = parent
	}

	return cwd, gitRoot
}

func (s *Spooler) resolveAgent(raw []byte) string {
	if len(raw) == 0 {
		return "agent"
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(raw, &payload); err == nil {
		if agent, ok := payload["agent"].(string); ok && agent != "" {
			return agent
		}
		// Antigravity signals
		if _, ok := payload["conversationId"]; ok {
			return "antigravity"
		}
	}
	return "agent"
}

// ListSessions inspects the spool directory and returns metadata for all .jsonl files.
func (s *Spooler) ListSessions() ([]SessionSummary, error) {
	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var summaries []SessionSummary
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		sessionID := strings.TrimSuffix(entry.Name(), ".jsonl")
		filePath := filepath.Join(s.baseDir, entry.Name())

		eventCount := 0
		if file, err := os.Open(filePath); err == nil {
			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				eventCount++
			}
			_ = file.Close()
		}

		summaries = append(summaries, SessionSummary{
			SessionID:   sessionID,
			FilePath:    filePath,
			SizeBytes:   info.Size(),
			EventCount:  eventCount,
			LastUpdated: info.ModTime(),
		})
	}

	return summaries, nil
}

// ReadSession reads all raw spool entries for a given session ID.
func (s *Spooler) ReadSession(sessionID string) ([]SpoolEntry, error) {
	filePath := filepath.Join(s.baseDir, sessionID+".jsonl")
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var entries []SpoolEntry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var entry SpoolEntry
		if err := json.Unmarshal(line, &entry); err == nil {
			entries = append(entries, entry)
		}
	}
	return entries, nil
}
