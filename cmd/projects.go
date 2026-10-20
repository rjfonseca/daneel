//nolint:revive // package comment not required
package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/rjfonseca/daneel/internal/search"
)

var (
	projectsDirty bool
	projectsClean bool
	projectsStack string
	projectsLimit int
)

var projectsCmd = &cobra.Command{
	Use:     "projects [query]",
	Aliases: []string{"repos", "proj"},
	Short:   "Instant (< 3ms) search across local Git repositories in ~/Work without filesystem overhead",
	RunE: func(cmd *cobra.Command, args []string) error {
		query := strings.Join(args, " ")

		engine, err := search.Open(cfg.DBPath, cfg.VaultPath)
		if err != nil {
			return fmt.Errorf("failed to open search engine: %w", err)
		}
		defer engine.Close() //nolint:errcheck,gosec

		repos, err := engine.SearchProjects(cmd.Context(), query, projectsDirty, projectsClean, projectsStack, projectsLimit)
		if err != nil {
			return fmt.Errorf("projects search failed: %w", err)
		}

		if jsonOut {
			bytes, err := json.MarshalIndent(repos, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(bytes))
			return nil
		}

		if len(repos) == 0 {
			filterDesc := ""
			if query != "" {
				filterDesc = fmt.Sprintf("for '%s' ", query)
			}
			fmt.Printf("📦 No repositories found %sin workspace (~/Work).\n", filterDesc)
			return nil
		}

		nameStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
		hostStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
		pathStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
		branchStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("255"))
		cleanBadge := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("82")).Render("[🟢 Clean]")
		dirtyBadge := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")).Render("[🟡 Dirty]")
		warnHeader := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))

		if projectsDirty && query == "" {
			fmt.Println(warnHeader.Render("⚠️ Repositories with pending local changes:"))
			for i, r := range repos {
				fmt.Printf("   %2d. %s (%s) — %s\n",
					i+1,
					nameStyle.Render(r.Name),
					hostStyle.Render(r.RelPath),
					pathStyle.Render(r.Path),
				)
			}
			return nil
		}

		for i, r := range repos {
			status := cleanBadge
			if r.Dirty {
				status = dirtyBadge
			}

			fmt.Printf("📦 %s %s\n",
				nameStyle.Render(r.Name),
				hostStyle.Render(fmt.Sprintf("(%s)", r.RelPath)),
			)
			fmt.Printf("   📂 %s\n", pathStyle.Render(r.Path))
			fmt.Printf("   🌿 Branch: %s %s\n", branchStyle.Render(r.Branch), status)
			if r.Stack != "" {
				fmt.Printf("   🛠️ Stack: %s\n", r.Stack)
			}
			if r.Description != "" {
				fmt.Printf("   📝 %s\n", hostStyle.Render(r.Description))
			}
			if i < len(repos)-1 {
				fmt.Println()
			}
		}

		return nil
	},
}

func init() {
	projectsCmd.Flags().BoolVar(&projectsDirty, "dirty", false, "Filter only repositories with uncommitted local changes")
	projectsCmd.Flags().BoolVar(&projectsClean, "clean", false, "Filter only clean repositories")
	projectsCmd.Flags().StringVar(&projectsStack, "stack", "", "Filter by primary language or stack (e.g. go, python, typescript)")
	projectsCmd.Flags().IntVarP(&projectsLimit, "limit", "l", 20, "Maximum number of repositories to return")
	RootCmd.AddCommand(projectsCmd)
}
