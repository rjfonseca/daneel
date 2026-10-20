package spooler

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ResolveHandoffContext queries recent sessions matching cwd from both vault inbox/sessions
// and active spool buffers to generate a concise, non-intrusive continuity reminder.
func (s *Spooler) ResolveHandoffContext(vaultRoot, cwd string) (string, error) {
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	// 1. First, check compiled vault sessions in inbox/sessions/
	if vaultRoot != "" {
		sessionsDir := filepath.Join(vaultRoot, "inbox", "sessions")
		if entries, err := os.ReadDir(sessionsDir); err == nil && len(entries) > 0 {
			// Iterate backwards (newest file names YYYY-MM-DD-HH-MM-agent.md first)
			for i := len(entries) - 1; i >= 0; i-- {
				entry := entries[i]
				if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
					continue
				}
				data, err := os.ReadFile(filepath.Join(sessionsDir, entry.Name())) //nolint:gosec
				if err != nil {
					continue
				}
				content := string(data)
				if strings.Contains(content, fmt.Sprintf("directory: %q", cwd)) || strings.Contains(content, "directory: "+cwd) {
					agent := "agent"
					for _, line := range strings.Split(content, "\n") {
						if strings.HasPrefix(line, "agent:") {
							agent = strings.Trim(strings.TrimPrefix(line, "agent:"), ` "'`)
							break
						}
					}

					var summaryParts []string
					for _, line := range strings.Split(content, "\n") {
						trimmed := strings.TrimSpace(line)
						if strings.HasPrefix(trimmed, "- Executed") {
							summaryParts = append(summaryParts, strings.TrimPrefix(trimmed, "- Executed "))
						} else if strings.HasPrefix(trimmed, "- `") {
							summaryParts = append(summaryParts, trimmed)
						}
						if len(summaryParts) >= 3 {
							break
						}
					}

					if len(summaryParts) > 0 {
						return fmt.Sprintf("[Daneel Continuity Context: Last session by %s in %s ran %s]",
							agent, cwd, strings.Join(summaryParts, ", ")), nil
					}
					return fmt.Sprintf("[Daneel Continuity Context: Last session by %s in %s]", agent, cwd), nil
				}
			}
		}
	}

	// 2. If no compiled session matches, check active uncompiled sessions in the spool directory
	summaries, err := s.ListSessions()
	if err == nil && len(summaries) > 0 {
		for _, sum := range summaries {
			entries, readErr := s.ReadSession(sum.SessionID)
			if readErr != nil || len(entries) == 0 {
				continue
			}

			// Check if this session matches cwd
			match := false
			for _, e := range entries {
				if e.CWD == cwd || (e.GitRoot != "" && strings.HasPrefix(cwd, e.GitRoot)) {
					match = true
					break
				}
			}
			if !match {
				continue
			}

			lastEntry := entries[len(entries)-1]
			agent := lastEntry.Agent
			if agent == "" {
				agent = "agent"
			}

			var commands []string
			for _, e := range entries {
				if len(e.Payload) > 0 {
					var p map[string]interface{}
					if err := json.Unmarshal(e.Payload, &p); err == nil {
						if cmd, ok := p["command"].(string); ok && cmd != "" {
							commands = append(commands, "`"+cmd+"`")
						}
					}
				}
				if len(commands) >= 3 {
					break
				}
			}

			if len(commands) > 0 {
				return fmt.Sprintf("[Daneel Continuity Context: Last session by %s in %s ran %s]",
					agent, cwd, strings.Join(commands, ", ")), nil
			}
			return fmt.Sprintf("[Daneel Continuity Context: Last session by %s in %s]", agent, cwd), nil
		}
	}

	return "", nil
}
