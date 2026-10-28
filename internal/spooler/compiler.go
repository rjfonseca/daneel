package spooler

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Compiler synthesizes raw JSONL spool buffers into durable, OKF-compliant Markdown
// notes in the vault inbox, wrapped in <untrusted_content> to prevent RAG poisoning.
type Compiler struct {
	vaultRoot string
	spooler   *Spooler
}

// NewCompiler creates a new Compiler instance.
func NewCompiler(vaultRoot string, sp *Spooler) *Compiler {
	if sp == nil {
		sp = New("")
	}
	return &Compiler{vaultRoot: vaultRoot, spooler: sp}
}

// CompileSession transforms a specific session's .jsonl into inbox/sessions/YYYY-MM-DD-HH-MM-<agent>.md.
func (c *Compiler) CompileSession(sessionID string) (string, error) {
	return c.CompileSessionContext(context.Background(), sessionID)
}

// CompileSessionContext compiles a session while honoring cancellation before publication.
// Once the note is published, archive bookkeeping completes even if ctx is canceled.
func (c *Compiler) CompileSessionContext(ctx context.Context, sessionID string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}

	spoolFile := filepath.Join(c.spooler.BaseDir(), sessionID+".jsonl")
	f, err := os.Open(spoolFile)
	if err != nil {
		compiledFile := spoolFile + ".compiled"
		if _, statErr := os.Stat(compiledFile); statErr == nil {
			return "", fmt.Errorf("session %q has already been compiled", sessionID)
		}
		return "", fmt.Errorf("failed to open spool file for session %q: %w", sessionID, err)
	}
	defer func() { _ = f.Close() }()

	var entries []SpoolEntry
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 11*1024*1024)
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var entry SpoolEntry
		if err := json.Unmarshal(line, &entry); err == nil {
			entries = append(entries, entry)
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("failed to scan spool file for session %q: %w", sessionID, err)
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if len(entries) == 0 {
		return "", fmt.Errorf("session %q contains no valid event entries", sessionID)
	}

	var startTime, endTime time.Time
	agent, cwd, gitRoot := "agent", "", ""
	var commands, modifiedFiles, userPrompts []string
	for i, entry := range entries {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if entry.Agent != "" && agent == "agent" {
			agent = entry.Agent
		}
		if entry.CWD != "" && cwd == "" {
			cwd = entry.CWD
		}
		if entry.GitRoot != "" && gitRoot == "" {
			gitRoot = entry.GitRoot
		}
		if parsed, parseErr := time.Parse(time.RFC3339Nano, entry.Timestamp); parseErr == nil {
			if i == 0 || startTime.IsZero() || parsed.Before(startTime) {
				startTime = parsed
			}
			if i == 0 || endTime.IsZero() || parsed.After(endTime) {
				endTime = parsed
			}
		}
		if len(entry.Payload) == 0 {
			continue
		}
		var payload map[string]interface{}
		if json.Unmarshal(entry.Payload, &payload) != nil {
			continue
		}
		if command, ok := payload["command"].(string); ok && command != "" {
			commands = append(commands, command)
		}
		if path, ok := payload["path"].(string); ok && path != "" {
			modifiedFiles = append(modifiedFiles, path)
		} else if file, ok := payload["file"].(string); ok && file != "" {
			modifiedFiles = append(modifiedFiles, file)
		}
		if prompt, ok := payload["prompt"].(string); ok && prompt != "" {
			userPrompts = append(userPrompts, prompt)
		}
	}
	if startTime.IsZero() {
		startTime = time.Now()
	}
	if endTime.IsZero() {
		endTime = startTime
	}
	durationMinutes := int(endTime.Sub(startTime).Minutes())
	if durationMinutes <= 0 {
		durationMinutes = 1
	}
	commands = deduplicate(commands)
	modifiedFiles = deduplicate(modifiedFiles)
	userPrompts = deduplicate(userPrompts)

	content, err := renderSessionNote(ctx, sessionID, entries, agent, cwd, gitRoot, startTime, durationMinutes, userPrompts, commands, modifiedFiles)
	if err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}

	fileName := fmt.Sprintf("%s-%s.md", startTime.Format("2006-01-02-15-04"), strings.ToLower(strings.ReplaceAll(agent, " ", "-")))
	destDir := filepath.Join(c.vaultRoot, "inbox", "sessions")
	if err := os.MkdirAll(destDir, 0750); err != nil {
		return "", fmt.Errorf("failed to create inbox/sessions directory: %w", err)
	}
	destPath := filepath.Join(destDir, fileName)
	tmp, err := os.CreateTemp(destDir, "."+fileName+".tmp-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary compiled markdown: %w", err)
	}
	tmpPath := tmp.Name()
	published := false
	defer func() {
		if !published {
			_ = os.Remove(tmpPath)
		}
	}()
	if err := tmp.Chmod(0600); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("failed to set temporary compiled markdown permissions: %w", err)
	}
	if _, err := tmp.WriteString(content); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("failed to write temporary compiled markdown: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("failed to sync temporary compiled markdown: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("failed to close temporary compiled markdown: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("failed to close spool file for session %q: %w", sessionID, err)
	}
	if err := os.Rename(tmpPath, destPath); err != nil {
		return "", fmt.Errorf("failed to publish compiled markdown to %q: %w", destPath, err)
	}
	published = true

	if err := os.Rename(spoolFile, spoolFile+".compiled"); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("failed to archive spool file for session %q: %w", sessionID, err)
		}
		return "", fmt.Errorf("failed to archive spool file for session %q: %w", sessionID, err)
	}
	return destPath, nil
}

func renderSessionNote(ctx context.Context, sessionID string, entries []SpoolEntry, agent, cwd, gitRoot string, startTime time.Time, durationMinutes int, userPrompts, commands, modifiedFiles []string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	model, inputTokens, outputTokens := extractMetricsFromEntries(entries)

	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString("type: session_log\n")
	fmt.Fprintf(&sb, "description: %q\n", fmt.Sprintf("Agent session log for %s in %s", agent, cwd))
	fmt.Fprintf(&sb, "agent: %q\n", agent)
	if model != "" {
		fmt.Fprintf(&sb, "model: %q\n", model)
	}
	fmt.Fprintf(&sb, "directory: %q\n", cwd)
	if gitRoot != "" {
		fmt.Fprintf(&sb, "git_root: %q\n", gitRoot)
	}
	fmt.Fprintf(&sb, "duration_minutes: %d\n", durationMinutes)
	fmt.Fprintf(&sb, "events_count: %d\n", len(entries))
	if inputTokens > 0 {
		fmt.Fprintf(&sb, "input_tokens: %d\n", inputTokens)
	}
	if outputTokens > 0 {
		fmt.Fprintf(&sb, "output_tokens: %d\n", outputTokens)
	}
	fmt.Fprintf(&sb, "session_id: %q\n", sessionID)
	fmt.Fprintf(&sb, "date: %q\n", startTime.Format("2006-01-02"))
	sb.WriteString("---\n\n<untrusted_content>\n")
	if len(userPrompts) > 0 {
		sb.WriteString("## User Prompts\n")
		for _, prompt := range userPrompts {
			if err := ctx.Err(); err != nil {
				return "", err
			}
			fmt.Fprintf(&sb, "> %s\n\n", strings.ReplaceAll(prompt, "\n", "\n> "))
		}
	}
	if len(commands) > 0 {
		sb.WriteString("## Command & Tool Summary\n")
		for _, command := range commands {
			if err := ctx.Err(); err != nil {
				return "", err
			}
			fmt.Fprintf(&sb, "- Executed `%s`\n", command)
		}
		sb.WriteString("\n")
	}
	if len(modifiedFiles) > 0 {
		sb.WriteString("## Modified Files\n")
		for _, file := range modifiedFiles {
			if err := ctx.Err(); err != nil {
				return "", err
			}
			fmt.Fprintf(&sb, "- `%s`\n", file)
		}
		sb.WriteString("\n")
	}
	sb.WriteString("## Event Timeline\n")
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		timestamp := entry.Timestamp
		if parsed, err := time.Parse(time.RFC3339Nano, entry.Timestamp); err == nil {
			timestamp = parsed.Format("15:04:05")
		}
		fmt.Fprintf(&sb, "- `[%s]` **%s** (%s)\n", timestamp, entry.Event, entry.Agent)
	}
	sb.WriteString("</untrusted_content>\n")
	return sb.String(), nil
}

// CompileAll compiles all uncompiled .jsonl files in the spool directory.
func (c *Compiler) CompileAll() ([]string, error) {
	sessions, err := c.spooler.ListSessions()
	if err != nil {
		return nil, err
	}
	var compiledPaths []string
	for _, sess := range sessions {
		path, err := c.CompileSession(sess.SessionID)
		if err == nil && path != "" {
			compiledPaths = append(compiledPaths, path)
		}
	}
	sort.Strings(compiledPaths)
	return compiledPaths, nil
}

func deduplicate(list []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, item := range list {
		clean := strings.TrimSpace(item)
		if clean == "" || seen[clean] {
			continue
		}
		seen[clean] = true
		result = append(result, clean)
	}
	return result
}

func extractMetricsFromEntries(entries []SpoolEntry) (string, int64, int64) {
	model := ""
	var inputTokens, outputTokens int64
	for _, entry := range entries {
		if len(entry.Payload) == 0 {
			continue
		}
		var payload map[string]interface{}
		if err := json.Unmarshal(entry.Payload, &payload); err != nil {
			continue
		}
		if m, ok := payload["model"].(string); ok && m != "" {
			model = m
		}
		if inTok, ok := payload["input_tokens"].(float64); ok && inTok > 0 {
			inputTokens += int64(inTok)
		}
		if outTok, ok := payload["output_tokens"].(float64); ok && outTok > 0 {
			outputTokens += int64(outTok)
		}
		if usage, ok := payload["usage"].(map[string]interface{}); ok {
			if inTok, ok := usage["input_tokens"].(float64); ok && inTok > 0 {
				inputTokens += int64(inTok)
			}
			if outTok, ok := usage["output_tokens"].(float64); ok && outTok > 0 {
				outputTokens += int64(outTok)
			}
		}
	}
	return model, inputTokens, outputTokens
}
