//nolint:revive // package comment not required
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/spf13/cobra"

	"github.com/rjfonseca/daneel/internal/search"
)

var readCmd = &cobra.Command{
	Use:   "read <path-or-title>",
	Short: "Renders a formatted technical note directly in terminal with syntax highlighting",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		target := strings.Join(args, " ")

		absTarget := target
		if !filepath.IsAbs(absTarget) {
			absTarget = filepath.Join(cfg.VaultPath, target)
		}

		var content string

		if _, err := os.Stat(absTarget); err == nil {
			data, err := os.ReadFile(absTarget) //nolint:gosec // intentional
			if err != nil {
				return fmt.Errorf("failed to read file: %w", err)
			}
			content = string(data)
		} else {
			// Find note via search
			engine, err := search.Open(cfg.DBPath, cfg.VaultPath)
			if err != nil {
				return fmt.Errorf("failed to open search engine: %w", err)
			}
			defer engine.Close() //nolint:errcheck,gosec // fully read or safe to ignore

			results, err := engine.Search(cmd.Context(), target, "all", 1)
			if err != nil || len(results) == 0 {
				return fmt.Errorf("note not found: '%s'", target)
			}

			matchedPath := filepath.Join(cfg.VaultPath, results[0].RelPath)
			data, err := os.ReadFile(matchedPath) //nolint:gosec // intentional
			if err != nil {
				return fmt.Errorf("failed to read note '%s': %w", results[0].RelPath, err)
			}
			content = string(data)
		}

		rendered, err := glamour.Render(content, "auto")
		if err != nil {
			fmt.Println(content)
			return nil
		}

		fmt.Print(rendered)
		return nil
	},
}

func init() {
	RootCmd.AddCommand(readCmd)
}
