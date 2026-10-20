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

var resolveCmd = &cobra.Command{
	Use:     "resolve <query>",
	Aliases: []string{"res", "which"},
	Short:   "Fuzzy resolve any note title, slug, or alias to its canonical vault path in < 2ms",
	Args:    cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := strings.Join(args, " ")

		engine, err := search.Open(cfg.DBPath, cfg.VaultPath)
		if err != nil {
			return fmt.Errorf("failed to open search engine: %w", err)
		}
		defer engine.Close() //nolint:errcheck,gosec

		res, err := engine.ResolveNote(cmd.Context(), query)
		if err != nil {
			return fmt.Errorf("resolution failed: %w", err)
		}

		if jsonOut {
			bytes, err := json.MarshalIndent(res, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(bytes))
			return nil
		}

		if !res.Found || res.Match == nil {
			fmt.Printf("🔍 No matching note found for: '%s'\n", query)
			return nil
		}

		pathStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
		metaStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))

		fmt.Printf("%s %s\n",
			pathStyle.Render(res.Match.RelPath),
			metaStyle.Render(fmt.Sprintf("(Title: %q, Score: %.2f)", res.Match.Title, res.Match.Score)),
		)

		return nil
	},
}

func init() {
	RootCmd.AddCommand(resolveCmd)
}
