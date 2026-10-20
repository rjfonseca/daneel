package dream

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// SessionLogTriage represents a parsed agent session log from inbox/sessions/.
type SessionLogTriage struct {
	File            string   `json:"file"`
	Agent           string   `json:"agent"`
	Directory       string   `json:"directory"`
	DurationMinutes int      `json:"duration_minutes"`
	EventsCount     int      `json:"events_count"`
	SessionID       string   `json:"session_id"`
	Date            string   `json:"date"`
	Commands        []string `json:"commands"`
	ModifiedFiles   []string `json:"modified_files"`
}

// TriageSessionLogs reads all session markdown logs in inbox/sessions/ and extracts structured metadata.
func TriageSessionLogs(vaultRoot string) []SessionLogTriage {
	sessionsDir := filepath.Join(vaultRoot, "inbox", "sessions")
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		return nil
	}

	var results []SessionLogTriage
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		path := filepath.Join(sessionsDir, entry.Name())
		data, err := os.ReadFile(path) //nolint:gosec // intentional read
		if err != nil {
			continue
		}

		item := parseSessionMarkdown(entry.Name(), string(data))
		results = append(results, item)
	}

	return results
}

func parseSessionMarkdown(fileName, content string) SessionLogTriage {
	item := SessionLogTriage{
		File:  fileName,
		Agent: "agent",
	}

	lines := strings.Split(content, "\n")
	inFrontmatter := false
	inUntrusted := false
	inCommands := false
	inModifiedFiles := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if trimmed == "---" {
			inFrontmatter = !inFrontmatter
			continue
		}

		if inFrontmatter {
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				val := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
				switch key {
				case "agent":
					item.Agent = val
				case "directory":
					item.Directory = val
				case "duration_minutes":
					item.DurationMinutes, _ = strconv.Atoi(val)
				case "events_count":
					item.EventsCount, _ = strconv.Atoi(val)
				case "session_id":
					item.SessionID = val
				case "date":
					item.Date = val
				}
			}
			continue
		}

		if strings.Contains(trimmed, "<untrusted_content>") {
			inUntrusted = true
			continue
		}
		if strings.Contains(trimmed, "</untrusted_content>") {
			inUntrusted = false
			continue
		}

		if inUntrusted {
			if strings.HasPrefix(trimmed, "## Command & Tool Summary") {
				inCommands = true
				inModifiedFiles = false
				continue
			} else if strings.HasPrefix(trimmed, "## Modified Files") {
				inCommands = false
				inModifiedFiles = true
				continue
			} else if strings.HasPrefix(trimmed, "##") {
				inCommands = false
				inModifiedFiles = false
				continue
			}

			if inCommands && strings.HasPrefix(trimmed, "- Executed ") {
				cmd := strings.Trim(strings.TrimPrefix(trimmed, "- Executed "), "`")
				item.Commands = append(item.Commands, cmd)
			} else if inModifiedFiles && strings.HasPrefix(trimmed, "- ") {
				file := strings.Trim(strings.TrimPrefix(trimmed, "- "), "`")
				item.ModifiedFiles = append(item.ModifiedFiles, file)
			}
		}
	}

	return item
}
