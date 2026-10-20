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
	peopleSquad string
	peopleRole  string
	peopleEmail string
)

var peopleCmd = &cobra.Command{
	Use:     "people [query]",
	Aliases: []string{"person", "who"},
	Short:   "Search and discover colleagues and leadership profiles with compound name matching and squad/role filters",
	RunE: func(cmd *cobra.Command, args []string) error {
		query := strings.Join(args, " ")

		engine, err := search.Open(cfg.DBPath, cfg.VaultPath)
		if err != nil {
			return fmt.Errorf("failed to open search engine: %w", err)
		}
		defer engine.Close() //nolint:errcheck,gosec

		results, err := engine.SearchPeople(cmd.Context(), query, peopleSquad, peopleRole, peopleEmail)
		if err != nil {
			return fmt.Errorf("people search failed: %w", err)
		}

		if jsonOut {
			bytes, err := json.MarshalIndent(results, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(bytes))
			return nil
		}

		if len(results) == 0 {
			filterDesc := ""
			if query != "" {
				filterDesc = fmt.Sprintf("for '%s' ", query)
			}
			fmt.Printf("👤 No colleague profiles found %sin Vault.\n", filterDesc)
			return nil
		}

		nameStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
		roleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
		squadStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
		pathStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
		emailStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("75"))

		for i, p := range results {
			var header strings.Builder
			fmt.Fprintf(&header, "👤 %s", nameStyle.Render(p.Name))
			if p.Role != "" {
				fmt.Fprintf(&header, " (%s)", roleStyle.Render(p.Role))
			}
			if p.Squad != "" {
				fmt.Fprintf(&header, " — %s", squadStyle.Render(p.Squad))
			}
			fmt.Println(header.String())
			fmt.Printf("   📄 %s\n", pathStyle.Render(p.RelPath))
			if p.Email != "" {
				fmt.Printf("   📧 %s\n", emailStyle.Render(p.Email))
			}
			if i < len(results)-1 {
				fmt.Println()
			}
		}

		return nil
	},
}

func init() {
	peopleCmd.Flags().StringVar(&peopleSquad, "squad", "", "Filter by squad or tribe")
	peopleCmd.Flags().StringVar(&peopleRole, "role", "", "Filter by professional role or title")
	peopleCmd.Flags().StringVar(&peopleEmail, "email", "", "Filter by corporate email address")
	RootCmd.AddCommand(peopleCmd)
}
