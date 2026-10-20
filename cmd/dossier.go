//nolint:revive // package comment not required
package cmd

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/rjfonseca/daneel/internal/ops/dossier"
)

var (
	dossierYear     int
	dossierSemester int
	dossierOutput   string
)

var dossierCmd = &cobra.Command{
	Use:     "dossier",
	Aliases: []string{"avd", "staff-avd", "report"},
	Short:   "Compiles the Staff Engineer Performance Review Executive Dossier",
	RunE: func(cmd *cobra.Command, args []string) error { //nolint:revive
		if dossierYear == 0 {
			dossierYear = time.Now().Year()
		}

		compiler := dossier.NewCompiler(cfg.VaultPath)
		dossierOutputData, err := compiler.Compile(dossierYear, dossierSemester, dossierOutput)
		if err != nil {
			return fmt.Errorf("failed to compile dossier: %w", err)
		}

		if jsonOut {
			bytes, err := json.MarshalIndent(dossierOutputData, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(bytes))
			return nil
		}

		headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
		passStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("82"))
		numStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
		pathStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))

		dossierTitle := "Staff Engineering Dossier"
		if cfg != nil && cfg.Evaluation.DossierTitle != "" {
			dossierTitle = cfg.Evaluation.DossierTitle
		}

		fmt.Println(headerStyle.Render(fmt.Sprintf("🏆 %s Compiled Successfully!", dossierTitle)))
		fmt.Printf("📄 File: %s\n\n", pathStyle.Render(dossierOutputData.OutputFile))

		fmt.Println("📊 Consolidated Time Telemetry:")
		if cfg != nil {
			for _, arch := range cfg.GetTaxonomyArchetypes() {
				h := dossierOutputData.Telemetry.Hours[arch.ID]
				p := dossierOutputData.Telemetry.Percentages[arch.ID]
				fmt.Printf("   • [%s] %-20s %s (%.1f%%)\n", arch.ID, arch.Name, numStyle.Render(fmt.Sprintf("%.1fh", h)), p)
			}
		}
		fmt.Printf("   • Total Consolidated:        %s across %d logged days\n\n", passStyle.Render(fmt.Sprintf("%.1fh", dossierOutputData.Telemetry.TotalHours)), dossierOutputData.Telemetry.LoggedDays)

		fmt.Printf("🎯 Individual Goals Matched: %d goals\n", len(dossierOutputData.Metas))
		fmt.Printf("🤝 Leadership & Supported Peers: %d colleagues with recorded history\n\n", len(dossierOutputData.PeopleImpacted))

		return nil
	},
}

func init() {
	dossierCmd.Flags().IntVarP(&dossierYear, "year", "y", 0, "Reference year (default: current year)")
	dossierCmd.Flags().IntVarP(&dossierSemester, "semester", "s", 0, "Specific semester (1 or 2)")
	dossierCmd.Flags().StringVarP(&dossierOutput, "output", "o", "", "Custom path to save the Markdown dossier")
	RootCmd.AddCommand(dossierCmd)
}
