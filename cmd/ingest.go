//nolint:revive // package comment not required
package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/rjfonseca/daneel/internal/ops/meetings"
	"github.com/rjfonseca/daneel/internal/search"
)

var (
	ingestZipFile    string
	ingestCleanInbox bool
	ingestReindex    bool
)

var ingestCmd = &cobra.Command{
	Use:     "ingest",
	Aliases: []string{"import", "process"},
	Short:   "Batch ingest and process Google Meet / Gemini meeting notes into the Second Brain",
	RunE: func(cmd *cobra.Command, args []string) error { //nolint:revive
		proc := meetings.NewProcessor(cfg.VaultPath)

		headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
		passStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("82"))
		pathStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))

		var rep *meetings.IngestReport
		var err error

		if ingestZipFile != "" {
			fmt.Printf("%s Processing archive file: %s\n", headerStyle.Render("🎙️ Meeting Ingestion:"), pathStyle.Render(ingestZipFile))
			rep, err = proc.ProcessZip(ingestZipFile)
		} else {
			fmt.Printf("%s Processing pending meeting drafts in inbox/...\n", headerStyle.Render("🎙️ Meeting Ingestion:"))
			rep, err = proc.ProcessInbox(ingestCleanInbox)
		}

		if err != nil {
			return fmt.Errorf("failed to process meeting notes: %w", err)
		}

		if jsonOut {
			bytes, err := json.MarshalIndent(rep, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(bytes))
			return nil
		}

		if rep.TotalProcessed == 0 {
			fmt.Println("ℹ️ No new meeting notes found to process.")
			return nil
		}

		fmt.Printf("\n%s %d meeting(s) processed and archived in sources/reunioes/:\n\n", passStyle.Render("✅ Success!"), rep.TotalProcessed)
		for i, m := range rep.Meetings {
			fmt.Printf("%2d. 📄 %s\n    📁 Destination: %s\n    👥 Participants: %d | 🎯 Action items: %d\n",
				i+1,
				m.Title,
				pathStyle.Render(m.SavedPath),
				len(m.Participants),
				len(m.ActionItems),
			)
		}
		fmt.Println()

		if ingestCleanInbox && rep.CleanedInbox > 0 {
			fmt.Printf("🧹 %d raw draft(s) removed from inbox/.\n", rep.CleanedInbox)
		}

		if ingestReindex && rep.TotalProcessed > 0 {
			fmt.Println("🔄 Reindexing Vault with new meeting notes...")
			if engine, err := search.Open(cfg.DBPath, cfg.VaultPath); err == nil {
				idx, upd, _ := engine.IndexVault(cmd.Context(), cfg.GetIndexedDirs(), false)
				engine.Close() //nolint:errcheck,gosec // fully read or safe to ignore
				fmt.Printf("   🔄 SQLite FTS5 index updated (%d new, %d modified)\n", idx, upd)
			}
		}

		return nil
	},
}

func init() {
	ingestCmd.Flags().StringVarP(&ingestZipFile, "zip", "z", "", "Path to a .zip archive containing Google Meet exports")
	ingestCmd.Flags().BoolVarP(&ingestCleanInbox, "clean", "c", true, "Remove original raw file from inbox/ after successful processing")
	ingestCmd.Flags().BoolVarP(&ingestReindex, "reindex", "r", true, "Reindex SQLite FTS5 after archiving meeting notes")
	RootCmd.AddCommand(ingestCmd)
}
