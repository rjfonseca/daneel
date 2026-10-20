//nolint:revive // package comment not required
package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/rjfonseca/daneel/internal/search"
)

var (
	searchScope   string
	searchType    string
	searchTag     string
	searchLimit   int
	searchReindex bool
)

var searchCmd = &cobra.Command{
	Use:     "search [query]",
	Aliases: []string{"find", "q"},
	Short:   "Full-text search in Vault via SQLite FTS5, BM25 ranking, and faceted filters",
	Args:    cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		queryStr := strings.Join(args, " ")
		if queryStr == "" && searchType == "" && searchTag == "" {
			return fmt.Errorf("search requires a query string, --type, or --tag")
		}

		engine, err := search.Open(cfg.DBPath, cfg.VaultPath)
		if err != nil {
			return fmt.Errorf("failed to open search engine: %w", err)
		}
		defer engine.Close() //nolint:errcheck,gosec // fully read or safe to ignore

		if searchReindex {
			if verbose {
				fmt.Println("🔄 Reindexing Vault...")
			}
			idx, upd, err := engine.IndexVault(cmd.Context(), cfg.GetIndexedDirs(), true)
			if err != nil {
				return fmt.Errorf("reindexing failed: %w", err)
			}
			if verbose {
				fmt.Printf("✅ Reindexing complete: %d new notes, %d updated.\n\n", idx, upd)
			}
		} else {
			// Auto incremental index
			_, _, _ = engine.IndexVault(cmd.Context(), cfg.GetIndexedDirs(), false)
		}

		start := time.Now()
		results, err := engine.SearchFaceted(cmd.Context(), search.SearchOptions{
			Query:    queryStr,
			Scope:    searchScope,
			NoteType: searchType,
			Tag:      searchTag,
			Limit:    searchLimit,
		})
		if err != nil {
			return fmt.Errorf("search error: %w", err)
		}
		elapsed := time.Since(start)

		if jsonOut {
			bytes, err := json.MarshalIndent(results, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(bytes))
			return nil
		}

		displayQuery := queryStr
		if displayQuery == "" {
			if searchType != "" && searchTag != "" {
				displayQuery = fmt.Sprintf("type:%s tag:%s", searchType, searchTag)
			} else if searchType != "" {
				displayQuery = fmt.Sprintf("type:%s", searchType)
			} else {
				displayQuery = fmt.Sprintf("tag:%s", searchTag)
			}
		}

		if len(results) == 0 {
			fmt.Printf("🔍 No results found in Vault for: '%s' (Scope: %s)\n", displayQuery, searchScope)
			return nil
		}

		headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
		titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
		pathStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
		scopeBadgeStyle := lipgloss.NewStyle().Background(lipgloss.Color("63")).Foreground(lipgloss.Color("255")).Padding(0, 1)
		snipStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))

		fmt.Printf("%s %s %s\n\n",
			headerStyle.Render(fmt.Sprintf("🔍 Results for '%s':", displayQuery)),
			pathStyle.Render(fmt.Sprintf("(%d found in %v)", len(results), elapsed)),
			scopeBadgeStyle.Render(strings.ToUpper(searchScope)),
		)

		for i, res := range results {
			fmt.Printf("%2d. %s  %s\n",
				i+1,
				titleStyle.Render(res.Title),
				pathStyle.Render("("+res.RelPath+")"),
			)

			if res.Description != "" {
				fmt.Printf("    📝 %s\n", res.Description)
			}

			if res.Snippet != "" {
				cleanSnip := strings.ReplaceAll(res.Snippet, "==", "\033[1;33m")
				cleanSnip = strings.ReplaceAll(cleanSnip, "\033[1;33m", "\033[1;33m") // highlight
				cleanSnip = strings.ReplaceAll(cleanSnip, "\n", " ")
				fmt.Printf("    💡 %s\n", snipStyle.Render(cleanSnip))
			}
			fmt.Println()
		}

		return nil
	},
}

func init() {
	searchCmd.Flags().StringVarP(&searchScope, "scope", "s", "all", "Filter by scope: all, tech, meta, daily, inbox, sources, public")
	_ = searchCmd.RegisterFlagCompletionFunc("scope", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) { //nolint:revive
		return []string{"all", "tech", "meta", "daily", "inbox", "sources", "public"}, cobra.ShellCompDirectiveNoFileComp
	})
	searchCmd.Flags().StringVarP(&searchType, "type", "t", "", "Filter by OKF note type (e.g. rfc, adr, arquitetura, person)")
	searchCmd.Flags().StringVar(&searchTag, "tag", "", "Filter by specific tag (e.g. kafka, ci-cd)")
	searchCmd.Flags().IntVarP(&searchLimit, "limit", "l", 10, "Maximum number of results to return")
	searchCmd.Flags().BoolVarP(&searchReindex, "reindex", "r", false, "Force complete reindexing of all notes")
	RootCmd.AddCommand(searchCmd)
}
