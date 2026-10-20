//nolint:revive // package comment not required
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/rjfonseca/daneel/internal/config"
	"github.com/rjfonseca/daneel/internal/ops/workspace"
	"github.com/rjfonseca/daneel/internal/search"
)

var (
	wsRoot                string
	wsSyncClean           bool
	wsWriteMOC            bool
	wsOutput              string
	wsInactivityThreshold string
)

var workspaceCmd = &cobra.Command{
	Use:     "workspace",
	Aliases: []string{"ws", "work-moc"},
	Short:   "Scans ~/Work, extracts Git metadata, and generates an enriched Workspace MOC",
	RunE: func(cmd *cobra.Command, args []string) error { //nolint:revive
		root := wsRoot
		if root == "" && cfg != nil {
			root = cfg.GetWorkDir()
		}
		scanner := workspace.NewScanner(root, cfg.VaultPath)

		threshold := 48 * time.Hour
		if cmd.Flags().Changed("inactivity-threshold") {
			if d, err := config.ParseDuration(wsInactivityThreshold); err == nil && d > 0 {
				threshold = d
			}
		} else if cfg != nil {
			threshold = cfg.GetBranchInactivityThreshold()
		}
		scanner.SetInactivityThreshold(threshold)

		start := time.Now()
		rep, err := scanner.Scan(wsSyncClean)
		if err != nil {
			return fmt.Errorf("failed to scan workspace: %w", err)
		}
		elapsed := time.Since(start)

		// Cache scanned repos in SQLite for instant discovery
		if engine, err := search.Open(cfg.DBPath, cfg.VaultPath); err == nil {
			var all []workspace.RepoInfo
			for _, catRepos := range rep.Categories {
				all = append(all, catRepos...)
			}
			_ = engine.SaveWorkspaceRepos(cmd.Context(), all)
			_ = engine.Close()
		}

		if wsWriteMOC {
			mocContent := scanner.GenerateMOCContent(rep)
			targetMOC := rep.MOCFilePath
			if wsOutput != "" {
				targetMOC = wsOutput
			}
			_ = os.MkdirAll(filepath.Dir(targetMOC), 0750)
			if err := os.WriteFile(targetMOC, []byte(mocContent), 0600); err != nil {
				return fmt.Errorf("failed to save workspace MOC: %w", err)
			}
		}

		if jsonOut {
			bytes, err := json.MarshalIndent(rep, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(bytes))
			return nil
		}

		headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
		passStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("82"))
		warnStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
		pathStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))

		fmt.Println(headerStyle.Render("🗺️ Workspace (~/Work) Cataloged Successfully!"))
		fmt.Printf("⚡ %d repositories scanned in %v\n\n", rep.TotalRepos, elapsed)

		fmt.Printf("📊 Health Summary:\n")
		fmt.Printf("   • %s: %d repositories up to date\n", passStyle.Render("🟢 Clean"), rep.CleanCount)
		fmt.Printf("   • %s: %d repositories with local changes\n", warnStyle.Render("🟡 Dirty"), rep.DirtyCount)
		if rep.SyncedCount > 0 {
			fmt.Printf("   • %s: %d clean repositories pulled\n", passStyle.Render("🔄 Synced"), rep.SyncedCount)
		}
		if rep.PrunedBranchesCount > 0 {
			fmt.Printf("   • %s: %d merged local branches removed\n", passStyle.Render("🌿 Pruned Branches"), rep.PrunedBranchesCount)
		}
		fmt.Println()

		if wsWriteMOC {
			fmt.Printf("📄 MOC Updated: %s\n\n", pathStyle.Render(rep.MOCFilePath))
		}

		if len(rep.DirtyRepos) > 0 {
			fmt.Println("⚠️ Repositories with Pending Local Changes:")
			limit := 5
			if len(rep.DirtyRepos) < limit {
				limit = len(rep.DirtyRepos)
			}
			for i, dr := range rep.DirtyRepos[:limit] {
				fmt.Printf("   %2d. %s (`%s` on branch `%s` - %d modified, %d untracked)\n",
					i+1,
					dr.Name,
					dr.RelPath,
					dr.Branch,
					dr.ModifiedCount,
					dr.UntrackedCount,
				)
			}
			if len(rep.DirtyRepos) > limit {
				fmt.Printf("   ... and %d more repositories.\n", len(rep.DirtyRepos)-limit)
			}
			fmt.Println()
		}

		return nil
	},
}

func init() {
	workspaceCmd.Flags().StringVar(&wsRoot, "root", "", "Workspace directory path (default: ~/Work)")
	workspaceCmd.Flags().BoolVar(&wsSyncClean, "sync-clean", true, "Execute automatic git pull on clean repositories on main/master")
	workspaceCmd.Flags().StringVar(&wsInactivityThreshold, "inactivity-threshold", "48h", "Inactivity duration before resetting clean feature branches to main/master (e.g. 48h, 24h, 2d)")
	workspaceCmd.Flags().BoolVar(&wsWriteMOC, "write", true, "Automatically write moc-workspace-work.md file to notes/organizacao/")
	workspaceCmd.Flags().StringVarP(&wsOutput, "output", "o", "", "Custom output path for the MOC")
	RootCmd.AddCommand(workspaceCmd)
}
