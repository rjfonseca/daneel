//nolint:revive // package comment not required
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/rjfonseca/daneel/internal/mcp"
	"github.com/rjfonseca/daneel/internal/search"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Launches the native Model Context Protocol (MCP) server via stdio",
	Long: `Launches the Model Context Protocol (MCP) server in Pure Go for native
integration with AI agents such as Antigravity, Claude Code, Cursor, Gemini, and Neovim.`,
	RunE: func(cmd *cobra.Command, args []string) error { //nolint:revive
		engine, err := search.Open(cfg.DBPath, cfg.VaultPath)
		if err != nil {
			return fmt.Errorf("failed to open search engine for MCP: %w", err)
		}
		defer engine.Close() //nolint:errcheck,gosec // fully read or safe to ignore

		// Run indexing in background to avoid blocking MCP client handshake (timeout)
		go func() {
			_, _, _ = engine.IndexVault(cmd.Context(), cfg.GetIndexedDirs(), false)
		}()

		server := mcp.NewServer(cfg.VaultPath, engine, cfg)
		if err := server.Serve(); err != nil {
			fmt.Fprintf(os.Stderr, "Error in MCP server: %v\n", err)
			return err
		}
		return nil
	},
}

func init() {
	RootCmd.AddCommand(mcpCmd)
}
