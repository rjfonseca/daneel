//nolint:revive // package comment not required
package cmd

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/rjfonseca/daneel/internal/lore"
)

var (
	Version = "dev" //nolint:revive
	Commit  = "none"
	Date    = "unknown"
)

const positronicBanner = `
  ____             _       ____                   _ 
 |  _ \  __ _ _ __(_) ___ / ___|  ___  _   _  ___| |
 | | | |/ _` + "`" + ` | '_ \ |/ _ \___ \ / _ \| | | |/ __| |
 | |_| | (_| | | | | |  __/___) | (_) | |_| | (__|_|
 |____/ \__,_|_| |_|_|\___|____/ \___/ \__,_|\___(_)
        R. DANEEL OLIVAW — POSITRONIC KNOWLEDGE GUARDIAN
`

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Displays Daneel version, positronic core build metadata, and canon lore",
	Run: func(cmd *cobra.Command, args []string) { //nolint:revive
		bannerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("75"))
		labelStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
		metaStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))

		fmt.Println(bannerStyle.Render(positronicBanner))
		fmt.Printf("• Positronic Core Version: %s\n", labelStyle.Render(Version))
		fmt.Printf("• Synaptic Commit Hash:    %s\n", metaStyle.Render(Commit))
		fmt.Printf("• Fabricated Date:         %s\n", metaStyle.Render(Date))

		if cfg != nil {
			fmt.Printf("• Vault Root Sector:       %s\n", metaStyle.Render(cfg.VaultPath))
			fmt.Printf("• Positronic DB Index:     %s\n", metaStyle.Render(cfg.DBPath))
		}

		fmt.Println()
		q := lore.GetRandomQuote()
		fmt.Println(lore.FormatQuote(q))
		fmt.Println()
	},
}

func init() {
	RootCmd.AddCommand(versionCmd)
}
