//nolint:revive // package comment not required
package backlog

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// BacklogItem represents a single task in the active backlog.
type BacklogItem struct { //nolint:revive
	Priority    string `json:"priority"` // P0, P1, P2, P3
	Title       string `json:"title"`
	Description string `json:"description"`
	Completed   bool   `json:"completed"`
	Raw         string `json:"raw"`
}

// BacklogReport holds the categorized backlog.
type BacklogReport struct { //nolint:revive
	Items []BacklogItem `json:"items"`
}

// GetActiveBacklog parses the active backlog section from the weekly summary document.
func GetActiveBacklog(vaultRoot string) (*BacklogReport, error) {
	sumarioPath := filepath.Join(vaultRoot, "notes", "carreira", "sumario-semanal-atividades-impactos-2026.md")
	if _, err := os.Stat(sumarioPath); os.IsNotExist(err) {
		sumarioPath = filepath.Join(vaultRoot, "meta", "carreira", "sumario-semanal-atividades-impactos-2026.md")
	}
	data, err := os.ReadFile(sumarioPath) //nolint:gosec // intentional
	if err != nil {
		return nil, err
	}

	content := string(data)
	lines := strings.Split(content, "\n")

	report := &BacklogReport{}
	currentPriority := "P1"
	inBacklogSection := false

	prioRegex := regexp.MustCompile(`###\s+[🔴🟡🟢]?\s*(P[0-3])`)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.Contains(trimmed, "Foco Atual") || strings.Contains(trimmed, "Backlog") || strings.Contains(trimmed, "Current Focus") || strings.Contains(trimmed, "Active Backlog") {
			inBacklogSection = true
			continue
		}

		if inBacklogSection && strings.HasPrefix(trimmed, "## ") && !strings.Contains(trimmed, "Foco Atual") && !strings.Contains(trimmed, "Backlog") && !strings.Contains(trimmed, "Current Focus") {
			break
		}

		if !inBacklogSection {
			continue
		}

		if pm := prioRegex.FindStringSubmatch(trimmed); len(pm) > 1 {
			currentPriority = pm[1]
			continue
		}

		if strings.HasPrefix(trimmed, "- [ ]") || strings.HasPrefix(trimmed, "- [x]") {
			completed := strings.HasPrefix(trimmed, "- [x]")
			rawTask := strings.TrimSpace(trimmed[5:])
			cleanTask := regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`).ReplaceAllString(rawTask, "$1")

			title := cleanTask
			desc := ""
			if idx := strings.Index(cleanTask, ":"); idx != -1 {
				title = strings.TrimSpace(cleanTask[:idx])
				desc = strings.TrimSpace(cleanTask[idx+1:])
			}

			report.Items = append(report.Items, BacklogItem{
				Priority:    currentPriority,
				Title:       title,
				Description: desc,
				Completed:   completed,
				Raw:         rawTask,
			})
		}
	}

	return report, nil
}
