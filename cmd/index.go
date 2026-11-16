//nolint:revive // package comment not required
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rjfonseca/daneel/internal/config"
	"github.com/rjfonseca/daneel/internal/daemon"
	"github.com/rjfonseca/daneel/internal/search"
)

var newEmbedderFromConfig = search.NewEmbedderFromConfig

func resolveEmbeddingsEnabled(cmd *cobra.Command, flagName string, flagValue bool, cfg *config.Config) bool {
	if cmd != nil && cmd.Flags().Changed(flagName) {
		return flagValue
	}
	if cfg == nil {
		return false
	}
	return cfg.Embeddings.Enabled && !strings.EqualFold(strings.TrimSpace(cfg.Embeddings.Provider), "none")
}

func effectiveEmbeddingsConfig(cmd *cobra.Command, flagName string, flagValue bool, cfg *config.Config) config.EmbeddingsConfig {
	if cfg == nil {
		return config.EmbeddingsConfig{}
	}
	embCfg := cfg.Embeddings
	if cmd != nil && cmd.Flags().Changed(flagName) && flagValue && !strings.EqualFold(strings.TrimSpace(embCfg.Provider), "none") {
		embCfg.Enabled = true
	}
	return embCfg
}

var (
	indexForce      bool
	indexEmbed      bool
	indexEmbedScope string
	indexBatchSize  int
	indexEmbedLimit int
)

var indexCmd = &cobra.Command{
	Use:   "index [path]",
	Short: "Indexes the entire vault, or a specific file or directory",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		engine, err := search.Open(cfg.DBPath, cfg.VaultPath)
		if err != nil {
			return fmt.Errorf("failed to open search engine: %w", err)
		}
		defer engine.Close() //nolint:errcheck,gosec // fully read or safe to ignore

		if len(args) == 0 {
			if verbose {
				if indexForce {
					fmt.Println("🔄 Reindexing entire Vault (force=true)...")
				} else {
					fmt.Println("🔄 Indexing entire Vault...")
				}
			}
			dirs := cfg.GetIndexedDirs()
			idx, upd, err := engine.IndexVault(cmd.Context(), dirs, indexForce)
			if err != nil {
				return fmt.Errorf("failed to index vault: %w", err)
			}
			if verbose {
				fmt.Printf("✅ Indexing complete: %d new notes, %d updated.\n", idx, upd)
			} else {
				fmt.Printf("Indexed Vault: %d new notes, %d updated.\n", idx, upd)
			}
		} else {
			targetPath := args[0]
			absPath, err := filepath.Abs(targetPath)
			if err != nil {
				return err
			}

			relPath, err := filepath.Rel(cfg.VaultPath, absPath)
			if err != nil {
				relPath = targetPath // fallback
			}
			relPath = filepath.ToSlash(relPath)

			info, err := os.Stat(absPath)
			if err != nil {
				return fmt.Errorf("path not found: %w", err)
			}

			if info.IsDir() {
				if verbose {
					fmt.Printf("🔄 Indexing directory %s...\n", relPath)
				}
				idx, upd, err := engine.IndexVault(cmd.Context(), []string{relPath}, true)
				if err != nil {
					return err
				}
				if verbose {
					fmt.Printf("✅ Indexing complete for %s: %d new notes, %d updated.\n", relPath, idx, upd)
				} else {
					fmt.Printf("Indexed directory: %s (%d new, %d updated)\n", relPath, idx, upd)
				}
			} else {
				if !strings.HasSuffix(strings.ToLower(info.Name()), ".md") {
					return fmt.Errorf("only .md files are supported for single file indexing")
				}
				if verbose {
					fmt.Printf("🔄 Indexing file %s...\n", relPath)
				}
				err := engine.IndexNote(cmd.Context(), relPath)
				if err != nil {
					return err
				}
				if verbose {
					fmt.Printf("✅ Indexing complete for %s\n", relPath)
				} else {
					fmt.Printf("Indexed file: %s\n", relPath)
				}
			}
		}

		if resolveEmbeddingsEnabled(cmd, "embed", indexEmbed, cfg) {
			embCfg := effectiveEmbeddingsConfig(cmd, "embed", indexEmbed, cfg)
			embedder, err := newEmbedderFromConfig(embCfg)
			if err != nil {
				return fmt.Errorf("failed to initialize embedder: %w", err)
			}
			if embedder == nil {
				fmt.Fprintln(os.Stderr, "Embeddings are disabled in configuration; skipping vector indexing.")
			} else {
				defer embedder.Close() //nolint:errcheck,gosec
				embeddedCount, err := engine.EmbedPendingChunks(
					cmd.Context(),
					embedder,
					indexEmbedScope,
					indexBatchSize,
					indexEmbedLimit,
					func(done, total int) {
						fmt.Fprintf(os.Stderr, "\r🧠 Embedding chunks: %d/%d", done, total)
					},
				)
				if embeddedCount > 0 {
					fmt.Fprintln(os.Stderr)
				}
				if err != nil {
					return fmt.Errorf("failed to embed pending chunks: %w", err)
				}
				fmt.Printf("Embedded %d chunks (%s).\n", embeddedCount, embedder.Model())
			}
		}

		if cli, err := daemon.NewClient(); err == nil {
			_ = cli.PublishUIEvent(4, "", "")
			_ = cli.Close()
		}

		return nil
	},
}

func init() {
	indexCmd.Flags().BoolVarP(&indexForce, "force", "f", false, "Force full reindexing of all notes regardless of timestamp")
	indexCmd.Flags().BoolVar(&indexEmbed, "embed", false, "Compute and store vector embeddings for pending chunks")
	indexCmd.Flags().StringVar(&indexEmbedScope, "scope", "all", "Scope filter for chunk embedding (all, tech, meta, daily, inbox, sources, public)")
	indexCmd.Flags().IntVar(&indexBatchSize, "batch-size", 64, "Batch size for vector embedding generation")
	indexCmd.Flags().IntVar(&indexEmbedLimit, "embed-limit", 0, "Maximum number of pending chunks to embed (0 = unlimited)")
	RootCmd.AddCommand(indexCmd)
}
