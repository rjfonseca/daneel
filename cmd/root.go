//nolint:revive // package comment not required
package cmd

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/rjfonseca/daneel/internal/config"
)

var (
	cfgFile    string
	vaultPath  string
	globalFlag bool
	jsonOut    bool
	verbose    bool
	cfg        *config.Config
)

// RootCmd represents the base command when called without any subcommands.
var RootCmd = &cobra.Command{
	Use:   "daneel",
	Short: "Daneel: The AI-First Engineering Second Brain CLI & MCP Server in Go",
	Long: `🧠 Daneel (daneel)
The AI-First Engineering Second Brain CLI & Model Context Protocol (MCP) Server.
Combines local full-text retrieval (SQLite FTS5/BM25), autonomous memory
consolidation (Dream Cycle), engineering leadership copilots, and native MCP tools for AI agents.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error { //nolint:revive
		// Initialize standard structured logger (slog) writing to stderr
		logLevel := slog.LevelWarn
		if verbose {
			logLevel = slog.LevelDebug
		}
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: logLevel,
		}))
		slog.SetDefault(logger)

		var err error
		cfg, err = config.LoadConfig(vaultPath, cfgFile, globalFlag)
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error { //nolint:revive
		// If launched without subcommands, open TUI
		return runTUI()
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "Custom path to .daneel.yaml or config.yaml")
	RootCmd.PersistentFlags().StringVar(&vaultPath, "vault-path", "", "Explicit path to the Vault root directory")
	RootCmd.PersistentFlags().BoolVarP(&globalFlag, "global", "g", false, "Force global vault mode (~/.config/daneel/config.yaml), ignoring local vaults")
	RootCmd.PersistentFlags().BoolVar(&jsonOut, "json", false, "Output results in JSON format")
	RootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose / debug output")
}
