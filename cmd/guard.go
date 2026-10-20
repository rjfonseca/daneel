//nolint:revive // package comment not required
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/rjfonseca/daneel/internal/guard"
	"github.com/rjfonseca/daneel/internal/lore"
)

var (
	guardCheckAll bool
	guardFile     string
)

var guardCmd = &cobra.Command{
	Use:     "guard [file]",
	Aliases: []string{"audit-public", "validate-export", "laws"},
	Short:   "Audits public notes against security firewall rules under the Four Laws of Robotics",
	RunE: func(cmd *cobra.Command, args []string) error { //nolint:revive
		customRules := guard.BuildCustomPatterns(cfg.GetBlockedDomains(), cfg.GetBlockedKeywords())
		val := guard.NewValidator(cfg.VaultPath, customRules...)

		targetFile := guardFile
		if targetFile == "" && len(args) > 0 {
			targetFile = args[0]
		}

		headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("75"))
		passStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("82"))
		warnStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196"))
		descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
		tipStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))

		lawZerothBadge := lipgloss.NewStyle().Background(lipgloss.Color("196")).Foreground(lipgloss.Color("16")).Bold(true).Padding(0, 1).Render("ZEROTH LAW")
		lawFirstBadge := lipgloss.NewStyle().Background(lipgloss.Color("208")).Foreground(lipgloss.Color("16")).Bold(true).Padding(0, 1).Render("FIRST LAW")
		lawSecondBadge := lipgloss.NewStyle().Background(lipgloss.Color("75")).Foreground(lipgloss.Color("16")).Bold(true).Padding(0, 1).Render("SECOND LAW")
		lawThirdBadge := lipgloss.NewStyle().Background(lipgloss.Color("141")).Foreground(lipgloss.Color("16")).Bold(true).Padding(0, 1).Render("THIRD LAW")

		renderLawBadge := func(l guard.LawType) string {
			switch l {
			case guard.ZerothLaw:
				return lawZerothBadge
			case guard.FirstLaw:
				return lawFirstBadge
			case guard.SecondLaw:
				return lawSecondBadge
			case guard.ThirdLaw:
				return lawThirdBadge
			default:
				return lawZerothBadge
			}
		}

		if targetFile != "" {
			fileRep, err := val.CheckFile(targetFile)
			if err != nil {
				return fmt.Errorf("failed to audit note: %w", err)
			}

			if jsonOut {
				bytes, err := json.MarshalIndent(fileRep, "", "  ")
				if err != nil {
					return err
				}
				fmt.Println(string(bytes))
				return nil
			}

			fmt.Printf("%s %s\n\n", headerStyle.Render("🛡️ Positronic Guardrail Audit (Four Laws of Robotics):"), descStyle.Render(fileRep.RelPath))

			if len(fileRep.Issues) == 0 {
				fmt.Printf("%s Note complies 100%% with the Zeroth, First, and Second Laws.\n\n", passStyle.Render("✅ Passed!"))
				fmt.Println(lore.FormatQuote(lore.GetRandomQuote()))
				fmt.Println()
				return nil
			}

			fmt.Printf("%s Found %d violations under robotic security axioms:\n\n", warnStyle.Render("❌ Infringements Detected:"), len(fileRep.Issues))
			for i, iss := range fileRep.Issues {
				fmt.Printf("%2d. %s [%s] %s\n    %s\n",
					i+1,
					renderLawBadge(iss.Law),
					warnStyle.Render(string(iss.Type)),
					descStyle.Render(iss.Detail),
					tipStyle.Render(fmt.Sprintf("💡 Directive: %s", iss.Suggestion)),
				)
			}
			fmt.Println()
			os.Exit(1)
			return nil
		}

		// Audit whole public/content/
		auditRep, err := val.AuditPublicVault()
		if err != nil {
			return fmt.Errorf("failed to audit public vault: %w", err)
		}

		if jsonOut {
			bytes, err := json.MarshalIndent(auditRep, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(bytes))
			return nil
		}

		fmt.Println(headerStyle.Render("🛡️ Public Repository Security Audit (Four Laws of Robotics)"))
		fmt.Println(strings.Repeat("─", 75))
		fmt.Printf("📄 Total notes audited: %d\n", auditRep.TotalFilesAudited)

		if auditRep.Passed {
			fmt.Printf("\n%s All notes in public/ adhere strictly to the Zeroth Law. Zero leaks detected.\n\n", passStyle.Render("✅ Integrity Verified!"))
			fmt.Println(lore.FormatQuote(lore.GetRandomQuote()))
			fmt.Println()
			return nil
		}

		fmt.Printf("\n%s Detected %d security and integrity violations:\n\n", warnStyle.Render("⚠️ Violations Found:"), auditRep.TotalViolations)
		for _, rep := range auditRep.Reports {
			fmt.Printf("📁 %s (%d alerts):\n", headerStyle.Render(rep.RelPath), len(rep.Issues))
			for _, iss := range rep.Issues {
				fmt.Printf("   • %s [%s] %s\n     %s\n",
					renderLawBadge(iss.Law),
					warnStyle.Render(string(iss.Type)),
					descStyle.Render(iss.Detail),
					tipStyle.Render(fmt.Sprintf("💡 Directive: %s", iss.Suggestion)),
				)
			}
			fmt.Println()
		}

		fmt.Println("🚫 Under the Zeroth Law, DO NOT commit or push to public repository until all alerts are sanitized.")
		os.Exit(1)
		return nil
	},
}

func init() {
	guardCmd.Flags().BoolVarP(&guardCheckAll, "check", "c", false, "Audit all notes in public/content/")
	guardCmd.Flags().StringVarP(&guardFile, "file", "f", "", "Validate a specific candidate note before copying to public/")
	RootCmd.AddCommand(guardCmd)
}
