//nolint:revive // package comment not required
package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/rjfonseca/daneel/internal/ops/clone"
)

var (
	cloneWorkDir string
	cloneDream   bool
	cloneNoNote  bool
	cloneFetch   bool
)

var cloneCmd = &cobra.Command{
	Use:   "clone <url-or-slug>",
	Short: "Clones a repository to ~/Work, inspects its stack, and creates a technical note in the Second Brain",
	Long: `🧠 Daneel Clone: Cognitive Repository Onboarding & Cataloging

Clones a Git repository into a canonical path (~/Work/<host>/<org>/<repo>).
Statically analyzes technology stacks, local build commands, and documentation,
then proactively generates or updates a technical note in notes/sistemas/ and Maps of Content (MOCs).

Usage Examples:
  $ daneel clone git@github.com:charmbracelet/lipgloss.git
  $ daneel clone https://gitlab.com/my-org/core-squad/api-service.git
  $ daneel clone charmbracelet/bubbletea
  $ daneel clone rjfonseca/daneel --dream
`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rawInput := args[0]

		workDir := cloneWorkDir
		if workDir == "" {
			workDir = cfg.GetWorkDir()
		}

		cloner := clone.NewCloner()
		opts := clone.CloneOptions{
			WorkRoot:   workDir,
			VaultRoot:  cfg.VaultPath,
			DBPath:     cfg.DBPath,
			ForceFetch: cloneFetch,
			RunDream:   cloneDream,
			SkipNote:   cloneNoNote,
		}

		res, err := cloner.Clone(cmd.Context(), rawInput, opts)
		if err != nil {
			return fmt.Errorf("clone failed: %w", err)
		}

		if jsonOut {
			bytes, err := json.MarshalIndent(res, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(bytes))
			return nil
		}

		// Styled Lipgloss Terminal Output
		headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
		passStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("82"))
		pathStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
		tagStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
		tipStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
		warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214"))

		fmt.Println(headerStyle.Render("🧠 Daneel Clone & Cognitive Onboarding"))
		fmt.Println(strings.Repeat("=", 60))

		if res.AlreadyExisted {
			fmt.Printf("%s Repository already exists in local workspace.\n", warnStyle.Render("📂 [Existing]"))
		} else {
			fmt.Printf("%s Repository successfully cloned into workspace.\n", passStyle.Render("📦 [Cloned]"))
		}

		fmt.Printf("   • Local Path:   %s\n", pathStyle.Render(res.LocalPath))
		fmt.Printf("   • Active Branch: %s\n", tagStyle.Render(res.Branch))
		fmt.Printf("   • Tech Stack:   %s\n", tagStyle.Render(res.Stack))
		if res.Description != "" {
			fmt.Printf("   • Purpose:      %s\n", pathStyle.Render(res.Description))
		}
		fmt.Println()

		if res.VaultNotePath != "" {
			fmt.Printf("%s System technical note registered:\n", passStyle.Render("📝 [Second Brain]"))
			fmt.Printf("   • Technical Note: %s\n", pathStyle.Render(res.VaultNotePath))
			if res.MOCUpdated {
				fmt.Printf("   • MOC Updated:    %s\n", pathStyle.Render("notes/sistemas/moc-sistemas.md"))
			}
			if res.Indexed {
				fmt.Printf("   • Search FTS5:    %s\n", passStyle.Render("Instantly indexed in SQLite"))
			}
			fmt.Println()
		}

		if res.DreamExecuted {
			fmt.Printf("%s Dream Cycle cognitive routine executed successfully!\n\n", passStyle.Render("💤 [Dream Cycle]"))
		} else if !cloneDream && res.VaultNotePath != "" {
			fmt.Println(tipStyle.Render("💡 Tip: Run 'daneel dream' to induce and enrich cross-domain cognitive links."))
			fmt.Println()
		}

		return nil
	},
}

func init() {
	cloneCmd.Flags().StringVarP(&cloneWorkDir, "dir", "d", "", "Target base directory for cloning (default: ~/Work)")
	cloneCmd.Flags().BoolVar(&cloneDream, "dream", false, "Run Dream Cycle immediately after cloning")
	cloneCmd.Flags().BoolVar(&cloneNoNote, "no-note", false, "Skip generating/updating the system note in the Vault")
	cloneCmd.Flags().BoolVar(&cloneFetch, "fetch", false, "Run 'git fetch' if repository already exists locally")
	RootCmd.AddCommand(cloneCmd)
}
