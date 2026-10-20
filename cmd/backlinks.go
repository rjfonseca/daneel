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

var backlinksCmd = &cobra.Command{
	Use:     "backlinks <rel-path>",
	Aliases: []string{"links", "b"},
	Short:   "Lists all notes that reference a specific document",
	Args:    cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error { //nolint:revive
		target := strings.Join(args, " ")

		engine, err := search.Open(cfg.DBPath, cfg.VaultPath)
		if err != nil {
			return fmt.Errorf("failed to initialize search engine: %w", err)
		}
		defer engine.Close() //nolint:errcheck,gosec // fully read or safe to ignore

		links, err := engine.GetBacklinks(target)
		if err != nil {
			return fmt.Errorf("error fetching backlinks: %w", err)
		}

		if jsonOut {
			bytes, err := json.MarshalIndent(links, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(bytes))
			return nil
		}

		if len(links) == 0 {
			fmt.Printf("🔗 No backlinks found for: '%s'\n", target)
			return nil
		}

		headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
		pathStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
		textStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))

		fmt.Printf("%s %s\n\n",
			headerStyle.Render("🔗 Backlinks and cross-references for:"),
			pathStyle.Render(target),
		)

		for i, l := range links {
			fmt.Printf("%2d. 📄 %s\n    %s\n",
				i+1,
				pathStyle.Render(l.SourceRelPath),
				textStyle.Render(fmt.Sprintf("Link text: \"%s\"", l.LinkText)),
			)
		}

		return nil
	},
}

func init() {
	RootCmd.AddCommand(backlinksCmd)
}
