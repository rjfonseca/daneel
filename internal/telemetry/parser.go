package telemetry

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/rjfonseca/daneel/internal/config"
	"github.com/rjfonseca/daneel/internal/routine"
	"github.com/rjfonseca/daneel/internal/spooler"
)

// ParserOptions configures directory locations and search window for session ingestion.
type ParserOptions struct {
	BrainDir  string // default ~/.gemini/antigravity-cli/brain
	ClaudeDir string // default ~/.claude
	SpoolDir  string // default filepath.Join(config.XDGCacheDir(), "spool")
	Since     time.Duration
	Config    *config.Config
}

// SessionSummary aggregates a session's own telemetry record with recursive subagent metrics.
type SessionSummary struct {
	Record            routine.SessionTelemetryRecord
	TotalInputTokens  int64   // includes all recursive subagent input tokens
	TotalCacheTokens  int64   // includes all recursive subagent cache read tokens
	TotalOutputTokens int64   // includes all recursive subagent output tokens
	TotalCostUSD      float64 // includes all recursive subagent costs
	ChildSessionIDs   []string
}

type subagentMeta struct {
	ParentID string
	Model    string
	Role     string
}

type parsedTranscript struct {
	record    routine.SessionTelemetryRecord
	childIDs  []string
	childMeta map[string]subagentMeta
}

var (
	reSubagentConvID = regexp.MustCompile(`"conversationId":\s*"([a-zA-Z0-9_-]+)"`)
	reSkillPath      = regexp.MustCompile(`(?:\.agents/skills/|/skills/)([^/]+)/SKILL\.md`)
)

// SyncSessionsToStore scans session transcripts across Antigravity, Claude Code, and Spooler,
// constructs hierarchical subagent trees, rolls up FinOps token costs, and persists them into SQLite.
func SyncSessionsToStore(ctx context.Context, store *routine.SQLiteStore, opts ParserOptions) ([]SessionSummary, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	opts = applyDefaultOptions(opts)
	cutoff := time.Now().Add(-opts.Since)

	sessions := make(map[string]*routine.SessionTelemetryRecord)
	parentToChildren := make(map[string][]string)
	childMetaMap := make(map[string]subagentMeta)

	// 1. Ingest Antigravity Transcripts
	if opts.BrainDir != "" {
		if err := ingestAntigravityDir(opts.BrainDir, cutoff, sessions, parentToChildren, childMetaMap); err != nil {
			return nil, fmt.Errorf("failed to ingest antigravity sessions: %w", err)
		}
	}

	// 2. Ingest Claude Code Logs
	if opts.ClaudeDir != "" {
		if err := ingestClaudeDir(opts.ClaudeDir, cutoff, sessions); err != nil {
			return nil, fmt.Errorf("failed to ingest claude code sessions: %w", err)
		}
	}

	// 3. Ingest Ambient Spooler Logs
	if opts.SpoolDir != "" {
		if err := ingestSpoolerDir(opts.SpoolDir, cutoff, sessions); err != nil {
			return nil, fmt.Errorf("failed to ingest spooler sessions: %w", err)
		}
	}

	// 4. Resolve Parent, Model, Role, and Individual Costs
	defaultModel := opts.Config.GetFinOpsConfig().DefaultModel
	if defaultModel == "" {
		defaultModel = "gemini-pro"
	}

	for id, rec := range sessions {
		if meta, ok := childMetaMap[id]; ok {
			if rec.ParentSessionID == "" {
				rec.ParentSessionID = meta.ParentID
			}
			if rec.Role == "" {
				rec.Role = meta.Role
			}
			if rec.Model == "" || rec.Model == "inherit" {
				if meta.Model != "" && meta.Model != "inherit" {
					rec.Model = normalizeModelName(meta.Model)
				}
			}
		}

		if rec.Model == "" || rec.Model == "inherit" {
			if rec.ParentSessionID != "" {
				if parent, ok := sessions[rec.ParentSessionID]; ok && parent.Model != "" && parent.Model != "inherit" {
					rec.Model = parent.Model
				}
			}
		}

		if rec.Model == "" || rec.Model == "inherit" {
			rec.Model = defaultModel
		}

		rec.EstimatedCostUSD = opts.Config.CalculateCostUSD(rec.Model, rec.InputTokens, rec.CacheReadTokens, rec.OutputTokens)
	}

	// 5. Build Recursive Summaries & Subagent Count
	summaries := make([]SessionSummary, 0, len(sessions))
	for _, rec := range sessions {
		children := parentToChildren[rec.SessionID]
		sort.Strings(children)
		rec.SubagentsCount = len(children)

		visited := make(map[string]bool)
		totIn, totCache, totOut, totCost := rollupDescendants(rec.SessionID, sessions, parentToChildren, visited)

		summary := SessionSummary{
			Record:            *rec,
			TotalInputTokens:  rec.InputTokens + totIn,
			TotalCacheTokens:  rec.CacheReadTokens + totCache,
			TotalOutputTokens: rec.OutputTokens + totOut,
			TotalCostUSD:      rec.EstimatedCostUSD + totCost,
			ChildSessionIDs:   children,
		}
		summaries = append(summaries, summary)
	}

	// Sort summaries deterministically by StartedAt DESC, then SessionID
	sort.Slice(summaries, func(i, j int) bool {
		if summaries[i].Record.StartedAt != summaries[j].Record.StartedAt {
			return summaries[i].Record.StartedAt > summaries[j].Record.StartedAt
		}
		return summaries[i].Record.SessionID < summaries[j].Record.SessionID
	})

	// 6. Persist to SQLite Store
	if store != nil {
		for _, s := range summaries {
			if err := store.UpsertSessionTelemetry(ctx, s.Record); err != nil {
				return nil, fmt.Errorf("failed to persist session telemetry for %s: %w", s.Record.SessionID, err)
			}
		}
	}

	return summaries, nil
}

func applyDefaultOptions(opts ParserOptions) ParserOptions {
	allDirsEmpty := opts.BrainDir == "" && opts.ClaudeDir == "" && opts.SpoolDir == ""
	home, _ := os.UserHomeDir()
	if allDirsEmpty {
		if home != "" {
			opts.BrainDir = filepath.Join(home, ".gemini", "antigravity-cli", "brain")
			opts.ClaudeDir = filepath.Join(home, ".claude")
		}
		opts.SpoolDir = filepath.Join(config.XDGCacheDir(), "spool")
	}
	if opts.Since <= 0 {
		opts.Since = 7 * 24 * time.Hour
	}
	if opts.Config == nil {
		opts.Config = &config.Config{
			Telemetry: config.TelemetryConfig{
				FinOps: config.DefaultFinOpsConfig(),
			},
		}
	}
	return opts
}

func rollupDescendants(sessionID string, sessions map[string]*routine.SessionTelemetryRecord, parentToChildren map[string][]string, visited map[string]bool) (int64, int64, int64, float64) {
	if visited[sessionID] {
		return 0, 0, 0, 0
	}
	visited[sessionID] = true

	var totalIn, totalCache, totalOut int64
	var totalCost float64

	for _, childID := range parentToChildren[sessionID] {
		if child, ok := sessions[childID]; ok {
			totalIn += child.InputTokens
			totalCache += child.CacheReadTokens
			totalOut += child.OutputTokens
			totalCost += child.EstimatedCostUSD

			cIn, cCache, cOut, cCost := rollupDescendants(childID, sessions, parentToChildren, visited)
			totalIn += cIn
			totalCache += cCache
			totalOut += cOut
			totalCost += cCost
		}
	}

	return totalIn, totalCache, totalOut, totalCost
}

// FindSessionInWindow identifies a matching root or routine session within a given temporal execution window.
func FindSessionInWindow(summaries []SessionSummary, startedAt, completedAt time.Time, routineName string) *SessionSummary {
	var bestMatch *SessionSummary
	var bestScore int

	windowStart := startedAt.Add(-30 * time.Second)
	windowEnd := completedAt.Add(30 * time.Second)

	for i := range summaries {
		s := &summaries[i]
		score := 0

		routineMatches := routineName != "" && s.Record.RoutineName == routineName
		if routineMatches {
			score += 100
		}

		sessStart := time.Unix(s.Record.StartedAt, 0)
		sessEnd := time.Unix(s.Record.UpdatedAt, 0)
		if sessEnd.Before(sessStart) {
			sessEnd = sessStart
		}

		timeOverlaps := false
		if !startedAt.IsZero() && !completedAt.IsZero() {
			if !sessEnd.Before(windowStart) && !sessStart.After(windowEnd) {
				timeOverlaps = true
				score += 50
			}
		}

		if !routineMatches && !timeOverlaps {
			continue
		}

		// Prefer root sessions
		if s.Record.ParentSessionID == "" {
			score += 20
		}

		if score > bestScore {
			bestScore = score
			bestMatch = s
		}
	}

	return bestMatch
}

// ----------------------------------------------------------------------------
// Antigravity Transcript Parser
// ----------------------------------------------------------------------------

type rawTranscriptLine struct {
	StepIndex       int                  `json:"step_index"`
	Source          string               `json:"source"`
	Type            string               `json:"type"`
	Status          string               `json:"status"`
	CreatedAt       string               `json:"created_at"`
	Content         string               `json:"content"`
	InputTokens     int64                `json:"input_tokens"`
	CacheReadTokens int64                `json:"cache_read_tokens"`
	OutputTokens    int64                `json:"output_tokens"`
	ToolCalls       []rawToolCallWrapper `json:"tool_calls"`
}

type rawToolCallWrapper struct {
	Name string                 `json:"name"`
	Args map[string]interface{} `json:"args"`
}

func ingestAntigravityDir(brainDir string, cutoff time.Time, sessions map[string]*routine.SessionTelemetryRecord, parentToChildren map[string][]string, childMetaMap map[string]subagentMeta) error {
	entries, err := os.ReadDir(brainDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		convID := entry.Name()
		convDir := filepath.Join(brainDir, convID)

		transcriptFile := filepath.Join(convDir, ".system_generated", "logs", "transcript.jsonl")
		if _, err := os.Stat(transcriptFile); err != nil {
			transcriptFile = filepath.Join(convDir, "transcript.jsonl")
			if _, err := os.Stat(transcriptFile); err != nil {
				continue
			}
		}

		info, err := os.Stat(transcriptFile)
		if err == nil && info.ModTime().Before(cutoff) {
			continue
		}

		parsed, err := parseAntigravityTranscript(convID, transcriptFile)
		if err != nil || parsed == nil {
			continue
		}

		sessions[convID] = &parsed.record
		if len(parsed.childIDs) > 0 {
			parentToChildren[convID] = append(parentToChildren[convID], parsed.childIDs...)
		}
		for cid, meta := range parsed.childMeta {
			childMetaMap[cid] = meta
		}
	}

	return nil
}

func parseAntigravityTranscript(convID, filePath string) (*parsedTranscript, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	rec := routine.SessionTelemetryRecord{
		SessionID: convID,
		Agent:     "antigravity",
	}

	var childIDs []string
	childMeta := make(map[string]subagentMeta)
	skillsSet := make(map[string]bool)
	var pendingSubagent *subagentMeta

	var minTime, maxTime time.Time
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)

	for scanner.Scan() {
		lineBytes := scanner.Bytes()
		if len(lineBytes) == 0 {
			continue
		}

		var line rawTranscriptLine
		if err := json.Unmarshal(lineBytes, &line); err != nil {
			continue
		}

		if line.CreatedAt != "" {
			if t, err := time.Parse(time.RFC3339Nano, line.CreatedAt); err == nil {
				if minTime.IsZero() || t.Before(minTime) {
					minTime = t
				}
				if maxTime.IsZero() || t.After(maxTime) {
					maxTime = t
				}
			}
		}

		switch line.Type {
		case "USER_INPUT", "SYSTEM_MESSAGE":
			if strings.Contains(line.Content, "Model Selection") {
				if normalized := detectModelFromSettings(line.Content); normalized != "" {
					rec.Model = normalized
				}
			}
			if strings.Contains(line.Content, "Execute the complete Autonomous Second Brain Strategic Dream Cycle protocol") ||
				strings.Contains(line.Content, "vault-dream-cycle") {
				rec.IsRoutine = true
				rec.RoutineName = "dream"
			}

		case "PLANNER_RESPONSE":
			rec.TurnsCount++
			rec.InputTokens += line.InputTokens
			rec.CacheReadTokens += line.CacheReadTokens
			rec.OutputTokens += line.OutputTokens

			for _, tc := range line.ToolCalls {
				switch tc.Name {
				case "read_file", "view_file":
					p := getPathArg(tc.Args)
					if m := reSkillPath.FindStringSubmatch(p); len(m) > 1 {
						skillsSet[m[1]] = true
					}
				case "invoke_subagent":
					model := cleanArg(tc.Args["Model"])
					role := cleanArg(tc.Args["Role"])
					pendingSubagent = &subagentMeta{
						ParentID: convID,
						Model:    model,
						Role:     role,
					}
				case "run_command":
					cmd := cleanArg(tc.Args["CommandLine"])
					if strings.Contains(cmd, "daneel ") || strings.HasPrefix(cmd, "daneel") {
						skillsSet["daneel"] = true
					}
				default:
					if isDaneelMCPTool(tc.Name) {
						skillsSet[tc.Name] = true
					}
				}
			}

		case "GENERIC":
			if strings.Contains(line.Content, "Created the following subagent:") && strings.Contains(line.Content, "conversationId") {
				if match := reSubagentConvID.FindStringSubmatch(line.Content); len(match) > 1 {
					cid := match[1]
					childIDs = append(childIDs, cid)
					if pendingSubagent != nil {
						childMeta[cid] = *pendingSubagent
						pendingSubagent = nil
					} else {
						childMeta[cid] = subagentMeta{ParentID: convID}
					}
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if !minTime.IsZero() {
		rec.StartedAt = minTime.Unix()
	} else {
		if fi, err := os.Stat(filePath); err == nil {
			rec.StartedAt = fi.ModTime().Unix()
		}
	}
	if !maxTime.IsZero() {
		rec.UpdatedAt = maxTime.Unix()
	} else {
		rec.UpdatedAt = rec.StartedAt
	}

	if len(skillsSet) > 0 {
		var skills []string
		for s := range skillsSet {
			skills = append(skills, s)
		}
		sort.Strings(skills)
		rec.SkillsUsed = strings.Join(skills, ",")
	}

	return &parsedTranscript{
		record:    rec,
		childIDs:  childIDs,
		childMeta: childMeta,
	}, nil
}

func detectModelFromSettings(content string) string {
	lower := strings.ToLower(content)
	switch {
	case strings.Contains(lower, "flash lite") || strings.Contains(lower, "flash_lite"):
		return "gemini-flash-lite"
	case strings.Contains(lower, "flash"):
		return "gemini-flash"
	case strings.Contains(lower, "sonnet"):
		return "claude-sonnet"
	case strings.Contains(lower, "haiku"):
		return "claude-haiku"
	case strings.Contains(lower, "opus"):
		return "claude-opus"
	case strings.Contains(lower, "codex") || strings.Contains(lower, "gpt"):
		return "gpt-codex"
	case strings.Contains(lower, "pro") || strings.Contains(lower, "argon"):
		return "gemini-pro"
	default:
		return "gemini-pro"
	}
}

func normalizeModelName(raw string) string {
	lower := strings.ToLower(raw)
	switch {
	case strings.Contains(lower, "flash lite") || strings.Contains(lower, "flash_lite"):
		return "gemini-flash-lite"
	case strings.Contains(lower, "flash"):
		return "gemini-flash"
	case strings.Contains(lower, "sonnet"):
		return "claude-sonnet"
	case strings.Contains(lower, "haiku"):
		return "claude-haiku"
	case strings.Contains(lower, "opus"):
		return "claude-opus"
	case strings.Contains(lower, "codex") || strings.Contains(lower, "gpt"):
		return "gpt-codex"
	case strings.Contains(lower, "pro") || strings.Contains(lower, "argon"):
		return "gemini-pro"
	default:
		if raw != "" && raw != "inherit" {
			return raw
		}
		return ""
	}
}

func isDaneelMCPTool(name string) bool {
	switch name {
	case "resolve_note", "search_vault", "person_briefing", "search_people",
		"search_projects", "get_backlinks", "vault_health", "compile_staff_dossier",
		"validate_public_export":
		return true
	default:
		return strings.HasPrefix(name, "daneel_")
	}
}

func getPathArg(args map[string]interface{}) string {
	if p, ok := args["Path"].(string); ok {
		return cleanArg(p)
	}
	if p, ok := args["AbsolutePath"].(string); ok {
		return cleanArg(p)
	}
	return ""
}

func cleanArg(val interface{}) string {
	if val == nil {
		return ""
	}
	s, ok := val.(string)
	if !ok {
		return fmt.Sprintf("%v", val)
	}
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "\"")
	return s
}

// ----------------------------------------------------------------------------
// Claude Code Logs Ingestion
// ----------------------------------------------------------------------------

func ingestClaudeDir(claudeDir string, cutoff time.Time, sessions map[string]*routine.SessionTelemetryRecord) error {
	if _, err := os.Stat(claudeDir); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	return filepath.WalkDir(claudeDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".jsonl") {
			return nil
		}

		info, err := d.Info()
		if err == nil && info.ModTime().Before(cutoff) {
			return nil
		}

		sessionID := strings.TrimSuffix(d.Name(), ".jsonl")
		if _, exists := sessions[sessionID]; exists {
			return nil
		}

		rec, err := parseClaudeFile(sessionID, path)
		if err == nil && rec != nil {
			sessions[sessionID] = rec
		}
		return nil
	})
}

func parseClaudeFile(sessionID, filePath string) (*routine.SessionTelemetryRecord, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	rec := routine.SessionTelemetryRecord{
		SessionID: sessionID,
		Agent:     "claude-code",
		Model:     "claude-sonnet",
	}

	var minTime, maxTime time.Time
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)

	for scanner.Scan() {
		lineBytes := scanner.Bytes()
		if len(lineBytes) == 0 {
			continue
		}

		var payload map[string]interface{}
		if err := json.Unmarshal(lineBytes, &payload); err != nil {
			continue
		}

		rec.TurnsCount++

		if tsStr, ok := payload["timestamp"].(string); ok && tsStr != "" {
			if t, err := time.Parse(time.RFC3339Nano, tsStr); err == nil {
				if minTime.IsZero() || t.Before(minTime) {
					minTime = t
				}
				if maxTime.IsZero() || t.After(maxTime) {
					maxTime = t
				}
			}
		}

		if m, ok := payload["model"].(string); ok && m != "" {
			rec.Model = normalizeModelName(m)
		}

		var inTok, cacheTok, outTok int64
		if usage, ok := payload["usage"].(map[string]interface{}); ok {
			if v, ok := usage["input_tokens"].(float64); ok {
				inTok = int64(v)
			}
			if v, ok := usage["cache_read_input_tokens"].(float64); ok {
				cacheTok = int64(v)
			}
			if v, ok := usage["output_tokens"].(float64); ok {
				outTok = int64(v)
			}
		} else {
			if v, ok := payload["input_tokens"].(float64); ok {
				inTok = int64(v)
			}
			if v, ok := payload["cache_read_tokens"].(float64); ok {
				cacheTok = int64(v)
			}
			if v, ok := payload["output_tokens"].(float64); ok {
				outTok = int64(v)
			}
		}

		if inTok == 0 && outTok == 0 {
			// Estimate based on characters
			if txt, ok := payload["prompt"].(string); ok && len(txt) > 0 {
				inTok = int64(len(txt) / 4)
			} else if txt, ok := payload["content"].(string); ok && len(txt) > 0 {
				inTok = int64(len(txt) / 4)
			}
			if txt, ok := payload["reply"].(string); ok && len(txt) > 0 {
				outTok = int64(len(txt) / 4)
			} else if txt, ok := payload["output"].(string); ok && len(txt) > 0 {
				outTok = int64(len(txt) / 4)
			}
		}

		rec.InputTokens += inTok
		rec.CacheReadTokens += cacheTok
		rec.OutputTokens += outTok
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if !minTime.IsZero() {
		rec.StartedAt = minTime.Unix()
	} else {
		if fi, err := os.Stat(filePath); err == nil {
			rec.StartedAt = fi.ModTime().Unix()
		}
	}
	if !maxTime.IsZero() {
		rec.UpdatedAt = maxTime.Unix()
	} else {
		rec.UpdatedAt = rec.StartedAt
	}

	return &rec, nil
}

// ----------------------------------------------------------------------------
// Ambient Spooler Logs Ingestion
// ----------------------------------------------------------------------------

func ingestSpoolerDir(spoolDir string, cutoff time.Time, sessions map[string]*routine.SessionTelemetryRecord) error {
	entries, err := os.ReadDir(spoolDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".jsonl") && !strings.HasSuffix(name, ".jsonl.compiled") {
			continue
		}

		info, err := entry.Info()
		if err == nil && info.ModTime().Before(cutoff) {
			continue
		}

		sessionID := name
		sessionID = strings.TrimSuffix(sessionID, ".compiled")
		sessionID = strings.TrimSuffix(sessionID, ".jsonl")

		if _, exists := sessions[sessionID]; exists {
			continue
		}

		filePath := filepath.Join(spoolDir, name)
		rec, err := parseSpoolerFile(sessionID, filePath)
		if err == nil && rec != nil {
			sessions[sessionID] = rec
		}
	}

	return nil
}

func parseSpoolerFile(sessionID, filePath string) (*routine.SessionTelemetryRecord, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	rec := routine.SessionTelemetryRecord{
		SessionID: sessionID,
		Agent:     "agent",
	}

	var minTime, maxTime time.Time
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)

	for scanner.Scan() {
		lineBytes := scanner.Bytes()
		if len(lineBytes) == 0 {
			continue
		}

		var entry spooler.SpoolEntry
		if err := json.Unmarshal(lineBytes, &entry); err != nil {
			continue
		}

		rec.TurnsCount++
		if entry.Agent != "" && (rec.Agent == "agent" || rec.Agent == "") {
			rec.Agent = entry.Agent
		}

		if entry.Timestamp != "" {
			if t, err := time.Parse(time.RFC3339Nano, entry.Timestamp); err == nil {
				if minTime.IsZero() || t.Before(minTime) {
					minTime = t
				}
				if maxTime.IsZero() || t.After(maxTime) {
					maxTime = t
				}
			}
		}

		var payload map[string]interface{}
		if len(entry.Payload) > 0 {
			_ = json.Unmarshal(entry.Payload, &payload)
		}

		var inTok, cacheTok, outTok int64
		if m, ok := payload["model"].(string); ok && m != "" {
			rec.Model = normalizeModelName(m)
		}

		if usage, ok := payload["usage"].(map[string]interface{}); ok {
			if v, ok := usage["input_tokens"].(float64); ok {
				inTok = int64(v)
			}
			if v, ok := usage["cache_read_tokens"].(float64); ok {
				cacheTok = int64(v)
			}
			if v, ok := usage["output_tokens"].(float64); ok {
				outTok = int64(v)
			}
		} else {
			if v, ok := payload["input_tokens"].(float64); ok {
				inTok = int64(v)
			}
			if v, ok := payload["cache_read_tokens"].(float64); ok {
				cacheTok = int64(v)
			}
			if v, ok := payload["output_tokens"].(float64); ok {
				outTok = int64(v)
			}
		}

		if inTok == 0 && outTok == 0 && payload != nil {
			if prompt, ok := payload["prompt"].(string); ok && len(prompt) > 0 {
				inTok = int64(len(prompt) / 4)
			}
			if result, ok := payload["result"].(string); ok && len(result) > 0 {
				outTok = int64(len(result) / 4)
			} else if reply, ok := payload["content"].(string); ok && len(reply) > 0 {
				outTok = int64(len(reply) / 4)
			}
		}

		rec.InputTokens += inTok
		rec.CacheReadTokens += cacheTok
		rec.OutputTokens += outTok
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if !minTime.IsZero() {
		rec.StartedAt = minTime.Unix()
	} else {
		if fi, err := os.Stat(filePath); err == nil {
			rec.StartedAt = fi.ModTime().Unix()
		}
	}
	if !maxTime.IsZero() {
		rec.UpdatedAt = maxTime.Unix()
	} else {
		rec.UpdatedAt = rec.StartedAt
	}

	return &rec, nil
}
