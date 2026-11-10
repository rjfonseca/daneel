//nolint:revive // package comment not required
package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/rjfonseca/daneel/internal/daemon"
	"github.com/rjfonseca/daneel/internal/ingest"
	"github.com/rjfonseca/daneel/internal/ops/meetings"
	"github.com/rjfonseca/daneel/internal/search"
)

var (
	ingestDest       string
	ingestRecursive  bool
	ingestMove       bool
	ingestReindex    bool
	ingestZipFile    string
	ingestCleanInbox bool
)

var ingestCmd = &cobra.Command{
	Use:     "ingest [paths...]",
	Aliases: []string{"import", "process"},
	Short:   "Ingest files, directories, or meeting notes into the Second Brain",
	Args:    cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error { //nolint:revive
		if cfg == nil {
			return errors.New("configuration not initialized")
		}
		if cfg.VaultPath == "" {
			return errors.New("vault path is not configured")
		}

		if len(args) > 0 {
			var searchEngine *search.Engine
			if ingestReindex {
				if sEngine, err := search.Open(cfg.DBPath, cfg.VaultPath); err == nil {
					searchEngine = sEngine
					defer searchEngine.Close() //nolint:errcheck,gosec
				}
			}

			engine := ingest.NewEngine(cfg.VaultPath, searchEngine, daemon.GetDefaultHub())
			opts := ingest.IngestOptions{
				Destination: ingestDest,
				Recursive:   ingestRecursive,
				Move:        ingestMove,
			}

			combinedReport := &ingest.IngestReport{
				Items: make([]ingest.IngestedNote, 0),
			}

			cmdCtx := cmd.Context()
			if cmdCtx == nil {
				cmdCtx = context.Background()
			}

			for _, path := range args {
				rep, err := engine.IngestPath(cmdCtx, path, opts)
				if err != nil {
					combinedReport.Errors = append(combinedReport.Errors, fmt.Sprintf("%s: %v", path, err))
					continue
				}
				combinedReport.Items = append(combinedReport.Items, rep.Items...)
				combinedReport.TotalProcessed += rep.TotalProcessed
				if len(rep.Errors) > 0 {
					combinedReport.Errors = append(combinedReport.Errors, rep.Errors...)
				}
			}

			if jsonOut {
				bytes, err := json.MarshalIndent(combinedReport, "", "  ")
				if err != nil {
					return err
				}
				fmt.Println(string(bytes))
				return nil
			}

			if combinedReport.TotalProcessed == 0 && len(combinedReport.Errors) == 0 {
				fmt.Println("ℹ️ No markdown notes found to process.")
				return nil
			}

			passStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("82"))
			warnStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
			errStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196"))
			pathStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))

			if combinedReport.TotalProcessed > 0 {
				fmt.Printf("\n%s %d note(s) processed and ingested into %s:\n\n", passStyle.Render("✅ Success!"), combinedReport.TotalProcessed, pathStyle.Render(ingestDest))
				for i, item := range combinedReport.Items {
					fmt.Printf("%2d. 📄 %s\n    📁 Destination: %s\n    📏 Size: %d bytes\n",
						i+1,
						item.Title,
						pathStyle.Render(item.RelPath),
						item.SizeBytes,
					)
					if item.Warning != "" {
						fmt.Printf("    ⚠️  Warning: %s\n", warnStyle.Render(item.Warning))
					}
				}
				fmt.Println()
			}

			if len(combinedReport.Errors) > 0 {
				fmt.Printf("%s %d error(s) encountered during ingestion:\n", errStyle.Render("⚠️ Errors:"), len(combinedReport.Errors))
				for _, e := range combinedReport.Errors {
					fmt.Printf("  • %s\n", e)
				}
				fmt.Println()
			}

			if ingestReindex && combinedReport.TotalProcessed > 0 {
				fmt.Println("🔄 SQLite FTS5 search index updated.")
			}

			if combinedReport.TotalProcessed == 0 && len(combinedReport.Errors) > 0 {
				return fmt.Errorf("ingest failed: %s", strings.Join(combinedReport.Errors, "; "))
			}

			return nil
		}

		// Backwards compatibility: meeting notes processor in inbox/ or zip
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
	ingestCmd.Flags().StringVarP(&ingestDest, "dest", "d", "inbox", "Destination folder in vault")
	ingestCmd.Flags().BoolVarP(&ingestRecursive, "recursive", "r", true, "Recursively scan directories")
	ingestCmd.Flags().BoolVarP(&ingestMove, "move", "m", false, "Remove source file(s) after successful ingestion")
	ingestCmd.Flags().BoolVar(&ingestReindex, "reindex", true, "Reindex SQLite FTS5 after ingestion")
	ingestCmd.Flags().StringVarP(&ingestZipFile, "zip", "z", "", "Path to a .zip archive containing Google Meet exports")
	ingestCmd.Flags().BoolVarP(&ingestCleanInbox, "clean", "c", true, "Remove original raw file from inbox/ after successful processing")
	RootCmd.AddCommand(ingestCmd)
}
