//nolint:revive // package comment not required
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/rjfonseca/daneel/internal/daemon"
	"github.com/rjfonseca/daneel/internal/search"
	"github.com/rjfonseca/daneel/internal/toon"
)

var (
	searchScope      string
	searchType       string
	searchTag        string
	searchLimit      int
	searchReindex    bool
	searchNoSync     bool
	searchForceSync  bool
	searchMinSyncAge time.Duration
	searchSemantic   bool
)

const defaultMinimumSyncAge = 2*time.Hour + 30*time.Minute

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
		if searchNoSync && searchForceSync {
			return fmt.Errorf("--no-sync and --force-sync cannot be used together")
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
		} else if searchForceSync {
			idx, upd, syncErr := engine.IndexVault(cmd.Context(), cfg.GetIndexedDirs(), false)
			if syncErr != nil {
				slog.Debug("forced incremental sync failed", "error", syncErr)
			} else {
				slog.Debug("forced incremental sync completed", "indexed", idx, "updated", upd)
			}
		} else if !searchNoSync {
			idx, upd, skipped, syncErr := engine.IndexVaultIfStale(cmd.Context(), cfg.GetIndexedDirs(), searchMinSyncAge)
			if syncErr != nil {
				slog.Debug("stale check incremental sync failed", "error", syncErr)
			} else {
				slog.Debug("smart sync evaluated", "skipped", skipped, "indexed", idx, "updated", upd, "min_age", searchMinSyncAge)
			}
		}

		start := time.Now()
		searchOpts := search.SearchOptions{
			Query:    queryStr,
			Scope:    searchScope,
			NoteType: searchType,
			Tag:      searchTag,
			Limit:    searchLimit,
		}
		var results []search.SearchResult
		if resolveEmbeddingsEnabled(cmd, "semantic", searchSemantic, cfg) {
			var usedDaemon bool
			if !searchReindex && cfg != nil && daemon.IsDaemonRunning() {
				if dRes, dErr := tryDaemonHybridSearch(cmd.Context(), cfg.VaultPath, cfg.DBPath, searchOpts); dErr == nil {
					results = dRes
					usedDaemon = true
					slog.Debug("hybrid search served via daemon")
				} else {
					slog.Debug("daemon hybrid search fallback to local", "error", dErr)
				}
			}
			if !usedDaemon {
				var embedder search.Embedder
				if cfg != nil {
					embCfg := effectiveEmbeddingsConfig(cmd, "semantic", searchSemantic, cfg)
					emb, embErr := newEmbedderFromConfig(embCfg)
					if embErr != nil {
						slog.Warn("failed to initialize semantic embedder, falling back to lexical search", "error", embErr)
					} else if emb != nil {
						embedder = emb
						defer embedder.Close() //nolint:errcheck,gosec
					}
				}
				results, err = engine.SearchHybrid(cmd.Context(), searchOpts, embedder)
			}
		} else {
			results, err = engine.SearchFaceted(cmd.Context(), searchOpts)
		}
		if err != nil {
			return fmt.Errorf("search error: %w", err)
		}
		elapsed := time.Since(start)

		if toonOut {
			headers := []string{"title", "path", "scope", "type", "score"}
			rows := make([][]string, len(results))
			for i, r := range results {
				rows[i] = []string{
					r.Title,
					r.RelPath,
					r.Scope,
					r.Type,
					fmt.Sprintf("%.2f", r.Score),
				}
			}
			fmt.Print(toon.EncodeTabular("results", headers, rows))
			return nil
		}

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
	searchCmd.Flags().BoolVar(&searchNoSync, "no-sync", false, "Never run incremental indexing before searching")
	searchCmd.Flags().BoolVar(&searchForceSync, "force-sync", false, "Always run incremental indexing before searching, ignoring --minimum-sync-age")
	searchCmd.Flags().DurationVar(&searchMinSyncAge, "minimum-sync-age", defaultMinimumSyncAge, "Minimum age since last sync before running incremental indexing")
	searchCmd.Flags().BoolVar(&searchSemantic, "semantic", false, "Enable hybrid semantic + BM25 search using local vector embeddings")
	RootCmd.AddCommand(searchCmd)
}

func tryDaemonHybridSearch(ctx context.Context, vaultPath, dbPath string, opts search.SearchOptions) ([]search.SearchResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	client, err := daemon.NewClientContext(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Close() }()

	return client.SearchHybridContext(ctx, daemon.SearchHybridPayload{
		VaultPath: vaultPath,
		DBPath:    dbPath,
		Query:     opts.Query,
		Scope:     opts.Scope,
		NoteType:  opts.NoteType,
		Tag:       opts.Tag,
		Limit:     opts.Limit,
	})
}
