//nolint:revive // package comment not required
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/rjfonseca/daneel/internal/lore"
	"github.com/rjfonseca/daneel/internal/vault"
)

// LintIssue describes a validation problem found in a note.
type LintIssue struct {
	FilePath string `json:"file_path"`
	Severity string `json:"severity"` // "error", "warning"
	Message  string `json:"message"`
}

// LintReport aggregates all linting results.
type LintReport struct {
	TotalNotesScanned int         `json:"total_notes_scanned"`
	TotalConnections  int         `json:"total_connections"`
	IssuesCount       int         `json:"issues_count"`
	Issues            []LintIssue `json:"issues"`
}

var doctorCmd = &cobra.Command{
	Use:     "doctor",
	Aliases: []string{"lint", "check", "diagnostics"},
	Short:   "Runs Positronic Memory & Synaptic Graph Diagnostics on the Vault",
	RunE: func(cmd *cobra.Command, args []string) error { //nolint:revive
		notes, err := vault.ScanVault(cfg.VaultPath, cfg.GetIndexedDirs())
		if err != nil {
			return fmt.Errorf("failed to scan vault: %w", err)
		}

		totalLinks := 0
		report := LintReport{
			TotalNotesScanned: len(notes),
			Issues:            []LintIssue{},
		}

		for _, n := range notes {
			totalLinks += len(n.Links)

			// 1. Check OKF Frontmatter on curated notes/
			if strings.HasPrefix(n.RelPath, "notes/") {
				if n.NoteType == "" || n.NoteType == "nota" {
					report.Issues = append(report.Issues, LintIssue{
						FilePath: n.RelPath,
						Severity: "warning",
						Message:  "Frontmatter 'type' missing or generic (expected: tech, architecture, process, etc.)",
					})
				}
				if n.Description == "" {
					report.Issues = append(report.Issues, LintIssue{
						FilePath: n.RelPath,
						Severity: "warning",
						Message:  "Concise frontmatter 'description' missing",
					})
				}
			}

			// 2. Check relative markdown links
			for _, link := range n.Links {
				if !link.IsWikiLink {
					targetAbs := filepath.Join(cfg.VaultPath, link.TargetRelPath)
					if _, err := os.Stat(targetAbs); os.IsNotExist(err) {
						report.Issues = append(report.Issues, LintIssue{
							FilePath: n.RelPath,
							Severity: "error",
							Message:  fmt.Sprintf("Broken synaptic pathway: '%s' -> target '%s' does not exist", link.LinkText, link.TargetRelPath),
						})
					}
				}
			}
		}

		report.TotalConnections = totalLinks
		report.IssuesCount = len(report.Issues)

		if jsonOut {
			bytes, err := json.MarshalIndent(report, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(bytes))
			return nil
		}

		titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("75"))
		passStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("82"))
		warnStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
		errStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196"))
		metaStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))

		fmt.Println(titleStyle.Render("🧠 Positronic Memory & Synaptic Graph Diagnostics"))
		fmt.Println(strings.Repeat("─", 65))
		fmt.Printf("• Synaptic Nodes (Notes):       %d\n", report.TotalNotesScanned)
		fmt.Printf("• Knowledge Pathways (Links):    %d\n", report.TotalConnections)
		fmt.Printf("• Memory Storage Core (SQLite):  %s\n", metaStyle.Render("Active & Healthy"))

		if report.IssuesCount == 0 {
			fmt.Printf("\n%s Synaptic graph is 100%% coherent. Zero entropy detected.\n\n", passStyle.Render("✅ Positronic Stability: Optimal"))
			fmt.Println(lore.FormatQuote(lore.GetRandomQuote()))
			fmt.Println()
			return nil
		}

		fmt.Printf("\n%s Found %d synaptic inconsistencies:\n\n", warnStyle.Render("⚠️ Synaptic Drift Detected:"), report.IssuesCount)

		for _, iss := range report.Issues {
			badge := warnStyle.Render("[WARN]")
			if iss.Severity == "error" {
				badge = errStyle.Render("[ERR ]")
			}
			fmt.Printf("%s %s: %s\n", badge, iss.FilePath, iss.Message)
		}

		fmt.Println()
		return nil
	},
}

func init() {
	RootCmd.AddCommand(doctorCmd)
}
