//nolint:revive // package comment not required
package auditor

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite" // SQLite driver for Cursor state.vscdb inspection
)

// HasVaultSearchTool checks whether the tools used in a turn include Daneel vault discovery or search tools.
func HasVaultSearchTool(tools []string) bool {
	for _, t := range tools {
		lt := strings.ToLower(t)
		if strings.Contains(lt, "search_vault") ||
			strings.Contains(lt, "resolve_note") ||
			strings.Contains(lt, "get_note_content") ||
			strings.Contains(lt, "get_backlinks") ||
			strings.Contains(lt, "search_people") ||
			strings.Contains(lt, "search_projects") ||
			strings.Contains(lt, "search_notes") ||
			strings.Contains(lt, "daneel_search") ||
			strings.Contains(lt, "daneel_resolve") ||
			(strings.Contains(lt, "daneel") && (strings.Contains(lt, "search") || strings.Contains(lt, "resolve") || strings.Contains(lt, "read"))) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Antigravity Scanner
// ---------------------------------------------------------------------------

// AntigravityScanner scans Antigravity session transcripts from the local brain directory.
type AntigravityScanner struct {
	brainDir string
}

// NewAntigravityScanner creates a new Antigravity transcript scanner.
func NewAntigravityScanner(brainDir string) *AntigravityScanner {
	if brainDir == "" {
		if home, err := os.UserHomeDir(); err == nil {
			brainDir = filepath.Join(home, ".gemini", "antigravity-cli", "brain")
		}
	}
	return &AntigravityScanner{brainDir: brainDir}
}

// Name returns the canonical agent name.
func (s *AntigravityScanner) Name() string {
	return "antigravity"
}

// ScanRecent scans recent Antigravity transcripts within the given duration.
func (s *AntigravityScanner) ScanRecent(since time.Duration) ([]Interaction, error) {
	if s.brainDir == "" {
		return nil, nil
	}
	if _, err := os.Stat(s.brainDir); os.IsNotExist(err) {
		return nil, nil
	}

	entries, err := os.ReadDir(s.brainDir)
	if err != nil {
		return nil, err
	}

	cutoff := time.Now().Add(-since)
	var interactions []Interaction

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		convID := entry.Name()
		convDir := filepath.Join(s.brainDir, convID)

		transcriptPath := filepath.Join(convDir, ".system_generated", "logs", "transcript.jsonl")
		if _, err := os.Stat(transcriptPath); os.IsNotExist(err) {
			transcriptPath = filepath.Join(convDir, "transcript.jsonl")
			if _, err := os.Stat(transcriptPath); os.IsNotExist(err) {
				continue
			}
		}

		items, err := s.parseFile(transcriptPath, convID, cutoff)
		if err != nil {
			continue
		}
		interactions = append(interactions, items...)
	}

	sort.Slice(interactions, func(i, j int) bool {
		return interactions[i].Timestamp.After(interactions[j].Timestamp)
	})

	return interactions, nil
}

func (s *AntigravityScanner) parseFile(filePath, convID string, cutoff time.Time) ([]Interaction, error) {
	file, err := os.Open(filePath) //nolint:gosec
	if err != nil {
		return nil, err
	}
	defer file.Close() //nolint:errcheck

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 2*1024*1024)

	var list []Interaction
	var pendingPrompt string
	var pendingTimestamp time.Time

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var tl transcriptLine
		if err := json.Unmarshal(line, &tl); err != nil {
			continue
		}

		tTime, _ := time.Parse(time.RFC3339, tl.CreatedAt)
		if tTime.IsZero() {
			tTime = time.Now()
		}

		if tTime.Before(cutoff) {
			continue
		}

		if tl.Type == "USER_INPUT" && tl.Source == "USER_EXPLICIT" {
			pendingPrompt = extractUserPrompt(tl.Content)
			pendingTimestamp = tTime
		} else if tl.Type == "PLANNER_RESPONSE" && pendingPrompt != "" {
			var tools []string
			for _, tc := range tl.ToolCalls {
				if tc.Name != "" {
					tools = append(tools, tc.Name)
				}
			}

			list = append(list, Interaction{
				AgentName:      "antigravity",
				ConversationID: convID,
				Timestamp:      pendingTimestamp,
				UserPrompt:     pendingPrompt,
				AgentReply:     tl.Content,
				ToolsUsed:      tools,
			})
			pendingPrompt = ""
		}
	}

	return list, scanner.Err()
}

// ---------------------------------------------------------------------------
// Claude Code Scanner
// ---------------------------------------------------------------------------

// ClaudeCodeScanner scans Claude Code JSONL transcripts from ~/.claude.
type ClaudeCodeScanner struct {
	claudeDir string
}

// NewClaudeCodeScanner creates a new Claude Code transcript scanner.
func NewClaudeCodeScanner(claudeDir string) *ClaudeCodeScanner {
	if claudeDir == "" {
		if home, err := os.UserHomeDir(); err == nil {
			claudeDir = filepath.Join(home, ".claude")
		}
	}
	return &ClaudeCodeScanner{claudeDir: claudeDir}
}

// Name returns the canonical agent name.
func (s *ClaudeCodeScanner) Name() string {
	return "claude-code"
}

type claudeGenericLine struct {
	Type      string          `json:"type"`
	Role      string          `json:"role"`
	Timestamp string          `json:"timestamp"`
	CreatedAt string          `json:"created_at"`
	Content   json.RawMessage `json:"content"`
	Message   *struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	} `json:"message"`
	Tools []string `json:"tools"`
	// Alternate turn schema
	UserPrompt string   `json:"user_prompt"`
	AgentReply string   `json:"agent_reply"`
	ToolCalls  []string `json:"tool_calls"`
}

// ScanRecent scans recent Claude Code transcripts within the given duration.
func (s *ClaudeCodeScanner) ScanRecent(since time.Duration) ([]Interaction, error) {
	if s.claudeDir == "" {
		return nil, nil
	}
	if _, err := os.Stat(s.claudeDir); os.IsNotExist(err) {
		return nil, nil
	}

	cutoff := time.Now().Add(-since)
	var interactions []Interaction

	// Search for .jsonl files in claudeDir and its subdirectories
	err := filepath.Walk(s.claudeDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil //nolint:nilerr // continue walking despite unreadable files
		}
		if info.IsDir() {
			return nil
		}
		if strings.HasSuffix(info.Name(), ".jsonl") {
			relPath, _ := filepath.Rel(s.claudeDir, path)
			items, err := s.parseFile(path, relPath, cutoff)
			if err == nil {
				interactions = append(interactions, items...)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(interactions, func(i, j int) bool {
		return interactions[i].Timestamp.After(interactions[j].Timestamp)
	})

	return interactions, nil
}

func (s *ClaudeCodeScanner) parseFile(filePath, convID string, cutoff time.Time) ([]Interaction, error) {
	file, err := os.Open(filePath) //nolint:gosec
	if err != nil {
		return nil, err
	}
	defer file.Close() //nolint:errcheck

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 2*1024*1024)

	var list []Interaction
	var pendingPrompt string
	var pendingTimestamp time.Time

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var cl claudeGenericLine
		if err := json.Unmarshal(line, &cl); err != nil {
			continue
		}

		// Support direct turn objects
		if cl.UserPrompt != "" && cl.AgentReply != "" {
			tTime := parseFlexibleTime(cl.Timestamp, cl.CreatedAt)
			if !tTime.Before(cutoff) {
				tools := cl.Tools
				if len(tools) == 0 {
					tools = cl.ToolCalls
				}
				list = append(list, Interaction{
					AgentName:      "claude-code",
					ConversationID: convID,
					Timestamp:      tTime,
					UserPrompt:     cleanUserPrompt(cl.UserPrompt),
					AgentReply:     cl.AgentReply,
					ToolsUsed:      tools,
				})
			}
			continue
		}

		// Support message-by-message streams
		role := strings.ToLower(cl.Role)
		if role == "" {
			role = strings.ToLower(cl.Type)
		}
		if cl.Message != nil && cl.Message.Role != "" {
			role = strings.ToLower(cl.Message.Role)
		}

		tTime := parseFlexibleTime(cl.Timestamp, cl.CreatedAt)

		if role == "user" {
			pendingPrompt = rawContentToString(cl.Content)
			if pendingPrompt == "" && cl.Message != nil {
				pendingPrompt = rawContentToString(cl.Message.Content)
			}
			pendingPrompt = cleanUserPrompt(pendingPrompt)
			pendingTimestamp = tTime
		} else if (role == "assistant" || role == "agent") && pendingPrompt != "" {
			if !pendingTimestamp.Before(cutoff) {
				reply := rawContentToString(cl.Content)
				if reply == "" && cl.Message != nil {
					reply = rawContentToString(cl.Message.Content)
				}

				tools := cl.Tools
				if len(tools) == 0 {
					tools = cl.ToolCalls
				}

				list = append(list, Interaction{
					AgentName:      "claude-code",
					ConversationID: convID,
					Timestamp:      pendingTimestamp,
					UserPrompt:     pendingPrompt,
					AgentReply:     reply,
					ToolsUsed:      tools,
				})
			}
			pendingPrompt = ""
		}
	}

	return list, scanner.Err()
}

// ---------------------------------------------------------------------------
// Cursor Scanner
// ---------------------------------------------------------------------------

// CursorScanner scans Cursor chat sessions from ~/.config/Cursor/User/workspaceStorage or ~/.cursor.
type CursorScanner struct {
	cursorDir string
}

// NewCursorScanner creates a new Cursor chat session scanner.
func NewCursorScanner(cursorDir string) *CursorScanner {
	if cursorDir == "" {
		if home, err := os.UserHomeDir(); err == nil {
			primary := filepath.Join(home, ".config", "Cursor", "User", "workspaceStorage")
			if _, err := os.Stat(primary); err == nil {
				cursorDir = primary
			} else {
				cursorDir = filepath.Join(home, ".cursor")
			}
		}
	}
	return &CursorScanner{cursorDir: cursorDir}
}

// Name returns the canonical agent name.
func (s *CursorScanner) Name() string {
	return "cursor"
}

// ScanRecent scans recent Cursor transcripts and workspace databases.
func (s *CursorScanner) ScanRecent(since time.Duration) ([]Interaction, error) {
	if s.cursorDir == "" {
		return nil, nil
	}
	if _, err := os.Stat(s.cursorDir); os.IsNotExist(err) {
		return nil, nil
	}

	cutoff := time.Now().Add(-since)
	var interactions []Interaction

	// 1. Walk looking for state.vscdb SQLite databases or .json/.jsonl logs
	_ = filepath.Walk(s.cursorDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil //nolint:nilerr // continue walking despite unreadable files
		}
		if info.IsDir() {
			return nil
		}

		if info.Name() == "state.vscdb" {
			items, err := s.scanSQLiteDB(path, cutoff)
			if err == nil {
				interactions = append(interactions, items...)
			}
		} else if strings.HasSuffix(info.Name(), ".jsonl") || strings.HasSuffix(info.Name(), ".json") {
			relPath, _ := filepath.Rel(s.cursorDir, path)
			items, err := s.parseJSONFile(path, relPath, cutoff)
			if err == nil {
				interactions = append(interactions, items...)
			}
		}
		return nil
	})

	sort.Slice(interactions, func(i, j int) bool {
		return interactions[i].Timestamp.After(interactions[j].Timestamp)
	})

	return interactions, nil
}

func (s *CursorScanner) scanSQLiteDB(dbPath string, cutoff time.Time) ([]Interaction, error) {
	dsn := "file:" + dbPath + "?mode=ro"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	defer db.Close() //nolint:errcheck

	rows, err := db.Query("SELECT key, value FROM ItemTable WHERE key LIKE '%chatdata%' OR key LIKE '%composerData%'")
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck

	var list []Interaction
	convID := filepath.Base(filepath.Dir(dbPath))

	for rows.Next() {
		var key, val string
		if err := rows.Scan(&key, &val); err != nil {
			continue
		}

		items := parseCursorJSONData(val, convID, cutoff)
		list = append(list, items...)
	}

	return list, nil
}

func (s *CursorScanner) parseJSONFile(filePath, convID string, cutoff time.Time) ([]Interaction, error) {
	data, err := os.ReadFile(filePath) //nolint:gosec
	if err != nil {
		return nil, err
	}

	return parseCursorJSONData(string(data), convID, cutoff), nil
}

func parseCursorJSONData(raw string, convID string, cutoff time.Time) []Interaction {
	var list []Interaction

	// Support array of turns
	var turns []struct {
		Timestamp string   `json:"timestamp"`
		User      string   `json:"user"`
		Assistant string   `json:"assistant"`
		Tools     []string `json:"tools"`
	}
	if err := json.Unmarshal([]byte(raw), &turns); err == nil && len(turns) > 0 {
		for _, t := range turns {
			tTime := parseFlexibleTime(t.Timestamp, "")
			if !tTime.Before(cutoff) && t.User != "" && t.Assistant != "" {
				list = append(list, Interaction{
					AgentName:      "cursor",
					ConversationID: convID,
					Timestamp:      tTime,
					UserPrompt:     cleanUserPrompt(t.User),
					AgentReply:     t.Assistant,
					ToolsUsed:      t.Tools,
				})
			}
		}
		return list
	}

	// Support complex Cursor chat structure (tabs/bubbles/messages)
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &obj); err != nil {
		return list
	}

	tabs, ok := obj["tabs"].([]interface{})
	if !ok {
		// Try bubbles or messages directly
		if bubbles, ok := obj["bubbles"].([]interface{}); ok {
			return extractBubbles(bubbles, convID, cutoff)
		}
		return list
	}

	for _, tab := range tabs {
		tabMap, ok := tab.(map[string]interface{})
		if !ok {
			continue
		}
		if bubbles, ok := tabMap["bubbles"].([]interface{}); ok {
			items := extractBubbles(bubbles, convID, cutoff)
			list = append(list, items...)
		}
	}

	return list
}

func extractBubbles(bubbles []interface{}, convID string, cutoff time.Time) []Interaction {
	var list []Interaction
	var pendingPrompt string
	var pendingTimestamp time.Time

	for _, b := range bubbles {
		bMap, ok := b.(map[string]interface{})
		if !ok {
			continue
		}

		bubbleType, _ := bMap["type"].(string)
		text, _ := bMap["rawText"].(string)
		if text == "" {
			text, _ = bMap["text"].(string)
		}

		tTime := time.Now()
		if ts, ok := bMap["timestamp"].(float64); ok && ts > 0 {
			tTime = time.UnixMilli(int64(ts))
		}

		if bubbleType == "user" {
			pendingPrompt = cleanUserPrompt(text)
			pendingTimestamp = tTime
		} else if (bubbleType == "ai" || bubbleType == "assistant") && pendingPrompt != "" {
			if !pendingTimestamp.Before(cutoff) {
				list = append(list, Interaction{
					AgentName:      "cursor",
					ConversationID: convID,
					Timestamp:      pendingTimestamp,
					UserPrompt:     pendingPrompt,
					AgentReply:     text,
					ToolsUsed:      []string{},
				})
			}
			pendingPrompt = ""
		}
	}

	return list
}

// ---------------------------------------------------------------------------
// Multi-Scanner Aggregator
// ---------------------------------------------------------------------------

// MultiScanner coordinates scanning across all supported AI agent frameworks.
type MultiScanner struct {
	scanners []AgentLogScanner
}

// NewMultiScanner initializes an aggregated scanner with the given scanner list.
func NewMultiScanner(scanners ...AgentLogScanner) *MultiScanner {
	return &MultiScanner{scanners: scanners}
}

// NewDefaultMultiScanner creates a MultiScanner configured with default agent paths.
func NewDefaultMultiScanner(brainDir, claudeDir, cursorDir string) *MultiScanner {
	return NewMultiScanner(
		NewAntigravityScanner(brainDir),
		NewClaudeCodeScanner(claudeDir),
		NewCursorScanner(cursorDir),
	)
}

// ScanRecent scans across registered agent scanners filtered by agent name ("all", "antigravity", "claude-code", "cursor").
func (m *MultiScanner) ScanRecent(since time.Duration, agentFilter string) ([]Interaction, error) {
	normFilter := strings.ToLower(strings.TrimSpace(agentFilter))
	switch normFilter {
	case "", "all":
		normFilter = "all"
	case "agy":
		normFilter = "antigravity"
	case "claude":
		normFilter = "claude-code"
	}

	var all []Interaction
	for _, scanner := range m.scanners {
		if normFilter != "all" && !strings.EqualFold(scanner.Name(), normFilter) {
			continue
		}

		items, err := scanner.ScanRecent(since)
		if err != nil {
			continue
		}
		all = append(all, items...)
	}

	sort.Slice(all, func(i, j int) bool {
		return all[i].Timestamp.After(all[j].Timestamp)
	})

	return all, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func parseFlexibleTime(primary, secondary string) time.Time {
	for _, s := range []string{primary, secondary} {
		if s == "" {
			continue
		}
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			return t
		}
		if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
			return t
		}
	}
	return time.Now()
}

func cleanUserPrompt(raw string) string {
	clean := extractUserPrompt(raw)
	return cleanPrompt(clean)
}

func rawContentToString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var str string
	if err := json.Unmarshal(raw, &str); err == nil {
		return str
	}

	// Could be an array of blocks: [{"type": "text", "text": "..."}]
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &blocks); err == nil {
		var sb strings.Builder
		for _, b := range blocks {
			if b.Text != "" {
				if sb.Len() > 0 {
					sb.WriteString("\n")
				}
				sb.WriteString(b.Text)
			}
		}
		return sb.String()
	}

	return string(raw)
}
