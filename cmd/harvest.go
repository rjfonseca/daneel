//nolint:revive // package comment not required
package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/rjfonseca/daneel/internal/daemon"
	"github.com/rjfonseca/daneel/internal/harvester"
	"github.com/rjfonseca/daneel/internal/search"
)

var (
	harvestCatalog bool
	harvestPrune   bool
	harvestDryRun  bool
	harvestRemote  bool
	harvestLimit   int
	harvestReindex bool
	harvestToken   string
	harvestType    string
)

var harvestCmd = &cobra.Command{
	Use:     "harvest [gitlab|github|jira] [target]",
	Aliases: []string{"fetch-docs", "scrape-docs"},
	Short:   "Resilient technical documentation harvester for GitLab and GitHub repositories",
	Args:    cobra.MinimumNArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		return daemon.RunOrDelegate(cmd.Context(), "harvest", args, func() error {
			headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
			passStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("82"))
			numStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
			pathStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))

			if harvestCatalog || (len(args) > 0 && strings.ToLower(args[0]) == "catalog") {
				stats, err := harvester.ReconcileSourcesCatalog(cfg.VaultPath)
				if err != nil {
					return fmt.Errorf("failed to reconcile sources catalog: %w", err)
				}
				if jsonOut {
					bytes, _ := json.MarshalIndent(stats, "", "  ")
					fmt.Println(string(bytes))
				} else {
					fmt.Printf("%s Sources catalog reconciled successfully!\n", passStyle.Render("📚 OKF Catalog:"))
					fmt.Printf("📦 Repository MOCs verified: %s\n", numStyle.Render(fmt.Sprint(stats.TotalReposIndexed)))
					fmt.Printf("🌌 Group MOCs verified:        %s\n", numStyle.Render(fmt.Sprint(stats.TotalGroupsIndexed)))
				}
				if harvestReindex && (stats.TotalReposIndexed > 0 || stats.TotalGroupsIndexed > 0) {
					if engine, err := search.Open(cfg.DBPath, cfg.VaultPath); err == nil {
						idx, upd, _ := engine.IndexVault(cmd.Context(), cfg.GetIndexedDirs(), false)
						_ = engine.Close()
						fmt.Printf("   🔄 SQLite FTS5 index updated (%d new, %d modified)\n", idx, upd)
					}
				}
				if cli, err := daemon.NewClient(); err == nil {
					_ = cli.PublishUIEvent(4, "", "")
					_ = cli.Close()
				}
				return nil
			}

			type harvestJob struct {
				platform   string
				target     string
				targetType string
			}

			var jobs []harvestJob
			if len(args) == 0 {
				hc := cfg.GetHarvesterConfig()
				for _, t := range hc.Targets {
					jobs = append(jobs, harvestJob{
						platform:   t.Platform,
						target:     t.TargetName(),
						targetType: t.TargetType(),
					})
				}
				if hc.Jira != nil && hc.Jira.Domain != "" {
					jobs = append(jobs, harvestJob{platform: "jira", target: "", targetType: ""})
				}
				if len(jobs) == 0 {
					return fmt.Errorf("no targets configured in daneel.yaml and no arguments provided")
				}
			} else {
				platform := strings.ToLower(args[0])
				target := ""
				if len(args) > 1 {
					target = args[1]
				}
				tType := harvestType
				if tType == "" {
					tType = "org"
				}

				if target == "" && platform != "jira" && platform != "gdrive" {
					hc := cfg.GetHarvesterConfig()
					for _, t := range hc.Targets {
						if strings.EqualFold(t.Platform, platform) {
							jobs = append(jobs, harvestJob{
								platform:   t.Platform,
								target:     t.TargetName(),
								targetType: t.TargetType(),
							})
						}
					}
					if len(jobs) == 0 {
						return fmt.Errorf("target not specified for %s and no %s targets configured in daneel.yaml", platform, platform)
					}
				} else {
					jobs = append(jobs, harvestJob{platform: platform, target: target, targetType: tType})
				}
			}

			var totalDownloaded int
			var anyErr error
			activeProjectsByPlatform := make(map[string][]string)

			for _, job := range jobs {
				var stats *harvester.HarvestStats
				var err error

				fmt.Printf("\n%s Starting documentation harvest (%s %s [%s])...\n", headerStyle.Render("🌾 Harvester:"), job.platform, job.target, job.targetType)

				if job.platform != "jira" && job.platform != "gdrive" && job.target == "" {
					return fmt.Errorf("target not specified for %s", job.platform)
				}

				switch job.platform {
				case "gitlab", "gl":
					gl := harvester.NewGitLabHarvester(cfg.VaultPath, harvestToken, cfg.GetHarvesterConfig())
					if job.targetType == "user" {
						stats, err = gl.HarvestUser(job.target, harvestLimit)
					} else {
						stats, err = gl.HarvestGroup(job.target, harvestLimit)
					}
				case "github", "gh":
					gh := harvester.NewGitHubHarvester(cfg.VaultPath, harvestToken, cfg.GetHarvesterConfig())
					if job.targetType == "user" {
						stats, err = gh.HarvestUser(job.target, harvestLimit)
					} else {
						stats, err = gh.HarvestOrg(job.target, harvestLimit)
					}
				case "gdrive", "gd":
					gd := harvester.NewGDriveHarvester(cfg)
					stats, err = gd.Harvest(cmd.Context())
				case "jira", "jr":
					jh := harvester.NewJiraHarvester(cfg.VaultPath, harvestToken, cfg.GetHarvesterConfig())
					stats, err = jh.Harvest(cmd.Context())
				default:
					err = fmt.Errorf("unknown platform '%s'", job.platform)
				}

				if err != nil {
					fmt.Printf("❌ Error harvesting %s: %v\n", job.platform, err)
					anyErr = err
					continue
				}

				if jsonOut {
					bytes, _ := json.MarshalIndent(stats, "", "  ")
					fmt.Println(string(bytes))
				} else {
					fmt.Printf("%s Harvest completed in %v!\n", passStyle.Render("🎉 Success!"), time.Duration(stats.DurationMs)*time.Millisecond)
					fmt.Printf("📦 Projects Processed:     %s\n", numStyle.Render(fmt.Sprint(stats.TotalProjects)))
					if stats.SkippedProjects > 0 {
						fmt.Printf("⏭️  Projects Skipped:       %s\n", pathStyle.Render(fmt.Sprint(stats.SkippedProjects)))
					}
					fmt.Printf("📄 Documents Discovered:   %s\n", numStyle.Render(fmt.Sprint(stats.TotalDocsFound)))
					fmt.Printf("⬇️  Downloaded / Updated:   %s\n", passStyle.Render(fmt.Sprint(stats.DownloadedCount)))
					fmt.Printf("⏭️  Unchanged (Cached):     %s\n", pathStyle.Render(fmt.Sprint(stats.SkippedCount)))
					fmt.Printf("📂 Target Directory:       %s\n", pathStyle.Render(stats.OutputDir))
				}

				if stats != nil && len(stats.ActiveProjects) > 0 {
					pKey := strings.ToLower(stats.Platform)
					activeProjectsByPlatform[pKey] = append(activeProjectsByPlatform[pKey], stats.ActiveProjects...)
				}
				totalDownloaded += stats.DownloadedCount
			}

			if harvestPrune {
				fmt.Printf("\n%s Pruning orphaned, excluded, and toxic repositories...\n", headerStyle.Render("🧹 Pruning:"))
				opts := harvester.PruneOptions{
					VaultRoot:        cfg.VaultPath,
					Config:           cfg.GetHarvesterConfig(),
					ActiveByPlatform: activeProjectsByPlatform,
					DryRun:           harvestDryRun,
				}
				pStats, pErr := harvester.PruneSources(opts)
				if pErr != nil {
					fmt.Printf("⚠️ Pruning encountered an error: %v\n", pErr)
				} else if pStats.TotalRemoved > 0 {
					fmt.Printf("   ✅ Removed %d toxic directories and %d orphaned/excluded projects.\n",
						pStats.ToxicDirsCount, pStats.ProjectsPrunedCount)
					totalDownloaded += pStats.TotalRemoved
				} else {
					fmt.Println("   ✅ No orphaned or toxic artifacts found.")
				}
			}

			if harvestReindex && totalDownloaded > 0 {
				fmt.Println("\n🔄 Reindexing Vault with new documents...")
				if engine, err := search.Open(cfg.DBPath, cfg.VaultPath); err == nil {
					idx, upd, _ := engine.IndexVault(cmd.Context(), cfg.GetIndexedDirs(), false)
					engine.Close() //nolint:errcheck,gosec // fully read or safe to ignore
					fmt.Printf("   🔄 SQLite FTS5 index updated (%d new, %d modified)\n", idx, upd)
				}
			}

			if totalDownloaded > 0 {
				if cli, err := daemon.NewClient(); err == nil {
					_ = cli.PublishUIEvent(4, "", "")
					_ = cli.Close()
				}
			}
			return anyErr
		})
	},
}

var harvestCatalogCmd = &cobra.Command{
	Use:   "catalog",
	Short: "Reconcile OKF index.md MOCs for all groups and repositories across sources/",
	RunE: func(cmd *cobra.Command, args []string) error {
		harvestCatalog = true
		return harvestCmd.RunE(cmd, []string{"catalog"})
	},
}

var harvestPruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Prune orphaned, excluded, and toxic repositories and directories from sources/",
	RunE: func(cmd *cobra.Command, args []string) error {
		return daemon.RunOrDelegate(cmd.Context(), "harvest", append([]string{"prune"}, args...), func() error {
			headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
			passStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("82"))
			warnStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
			pathStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))

			if !jsonOut {
				fmt.Printf("%s Starting sources cleanup and pruning...\n", headerStyle.Render("🌾 Harvester Pruning:"))
			}

			var activeMap map[string][]string
			if harvestRemote {
				if !jsonOut {
					fmt.Println("📡 Querying remote APIs to map active repositories...")
				}
				active, err := harvester.LoadActiveRemoteProjects(cfg.VaultPath, cfg.GetHarvesterConfig(), harvestToken)
				if err != nil {
					return fmt.Errorf("could not fetch remote projects for pruning: %w", err)
				} else {
					activeMap = active
					totalActive := 0
					for _, list := range activeMap {
						totalActive += len(list)
					}
					if !jsonOut {
						fmt.Printf("   ✅ Mapped %d active remote projects across platforms.\n", totalActive)
					}
				}
			}

			opts := harvester.PruneOptions{
				VaultRoot:        cfg.VaultPath,
				Config:           cfg.GetHarvesterConfig(),
				ActiveByPlatform: activeMap,
				DryRun:           harvestDryRun,
			}

			stats, err := harvester.PruneSources(opts)
			if err != nil {
				return fmt.Errorf("pruning failed: %w", err)
			}

			if jsonOut {
				bytes, _ := json.MarshalIndent(stats, "", "  ")
				fmt.Println(string(bytes))
			} else {
				actionLabel := "📦 Quarantined"
				if harvestDryRun {
					actionLabel = "🔍 Dry-run: would prune"
				}
				fmt.Printf("\n%s %d item(s) in %v:\n", passStyle.Render(actionLabel), stats.TotalRemoved, time.Duration(stats.DurationMs)*time.Millisecond)
				fmt.Printf("   • Toxic directories:         %s\n", warnStyle.Render(fmt.Sprint(stats.ToxicDirsCount)))
				fmt.Printf("   • Excluded/orphaned projects: %s\n", warnStyle.Render(fmt.Sprint(stats.ProjectsPrunedCount)))

				if len(stats.Items) > 0 {
					fmt.Println()
					for _, it := range stats.Items {
						fmt.Printf("   • [%s] %s (%s)\n", pathStyle.Render(it.Type), it.Path, it.Reason)
						if it.RestorePath != "" {
							fmt.Printf("     Recovery copy: %s\n", it.RestorePath)
						}
					}
				}
			}

			if !harvestDryRun && stats.TotalRemoved > 0 && harvestReindex {
				if !jsonOut {
					fmt.Println("\n🔄 Reindexing Vault after pruning...")
				}
				if engine, err := search.Open(cfg.DBPath, cfg.VaultPath); err == nil {
					idx, upd, _ := engine.IndexVault(cmd.Context(), cfg.GetIndexedDirs(), false)
					engine.Close() //nolint:errcheck,gosec // fully read or safe to ignore
					if !jsonOut {
						fmt.Printf("   🔄 SQLite FTS5 index updated (%d new, %d modified)\n", idx, upd)
					}
				}
			}

			if !harvestDryRun && stats.TotalRemoved > 0 {
				if cli, err := daemon.NewClient(); err == nil {
					_ = cli.PublishUIEvent(4, "", "")
					_ = cli.Close()
				}
			}
			return nil
		})
	},
}

func init() {
	harvestCmd.Flags().BoolVar(&harvestCatalog, "catalog", false, "Reconcile OKF index.md MOCs without downloading")
	harvestCmd.Flags().BoolVar(&harvestPrune, "prune", false, "Prune orphaned, excluded, or toxic projects and directories after harvest")
	harvestCmd.Flags().BoolVar(&harvestDryRun, "dry-run", false, "Perform dry-run without removing files")
	harvestCmd.Flags().IntVarP(&harvestLimit, "limit", "l", 0, "Maximum number of repositories to process (default: all)")
	harvestCmd.Flags().BoolVarP(&harvestReindex, "reindex", "r", true, "Automatically reindex SQLite FTS5 if documents are changed")
	harvestCmd.Flags().StringVarP(&harvestToken, "token", "t", "", "Authentication token (or use environment variables)")
	harvestCmd.Flags().StringVar(&harvestType, "type", "", "Target type: 'org' (default) or 'user'")

	harvestPruneCmd.Flags().BoolVar(&harvestDryRun, "dry-run", false, "Perform dry-run without removing files")
	harvestPruneCmd.Flags().BoolVar(&harvestRemote, "remote", true, "Query remote APIs (GitLab/GitHub) to discover remote orphans")
	harvestPruneCmd.Flags().BoolVarP(&harvestReindex, "reindex", "r", true, "Automatically reindex SQLite FTS5 after pruning")
	harvestPruneCmd.Flags().StringVarP(&harvestToken, "token", "t", "", "Authentication token (or use environment variables)")

	harvestCmd.AddCommand(harvestCatalogCmd)
	harvestCmd.AddCommand(harvestPruneCmd)
	RootCmd.AddCommand(harvestCmd)
}
