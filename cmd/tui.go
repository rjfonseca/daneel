//nolint:revive // package comment not required
package cmd

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/rjfonseca/daneel/internal/search"
	"github.com/rjfonseca/daneel/internal/tui"
)

var tuiCmd = &cobra.Command{
	Use:     "tui",
	Aliases: []string{"ui", "browse", "dashboard"},
	Short:   "Opens the interactive Terminal User Interface (TUI) for the Second Brain",
	Long: `Opens the comprehensive Terminal User Interface (TUI) for the Second Brain,
providing instant full-text search with Markdown preview, Staff Radar visualization,
workspace repository catalog, people briefings, and Dream Cycle handoff.`,
	RunE: func(cmd *cobra.Command, args []string) error { //nolint:revive
		return runTUI()
	},
}

func runTUI() error {
	vaultRoot := cfg.VaultPath
	if vaultRoot == "" {
		cwd, _ := os.Getwd()
		vaultRoot = cwd
	}

	searchEngine, err := search.Open(cfg.DBPath, vaultRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "⚠️ Warning: failed to initialize search engine: %v\n", err)
	}
	defer func() {
		if searchEngine != nil {
			searchEngine.Close() //nolint:errcheck,gosec // fully read or safe to ignore
		}
	}()

	appModel := tui.NewAppModel(vaultRoot, searchEngine, cfg.GetTuiConfig())

	p := tea.NewProgram(
		appModel,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	tui.StartMCPServer(p)

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running TUI: %w", err)
	}

	return nil
}

func init() {
	RootCmd.AddCommand(tuiCmd)
}
