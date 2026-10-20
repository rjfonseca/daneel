//nolint:revive // package comment not required
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/rjfonseca/daneel/internal/config"
)

var configCmd = &cobra.Command{
	Use:     "config",
	Aliases: []string{"info", "env"},
	Short:   "Displays active configuration, detected paths, and XDG standards for Daneel",
	Long: `Inspects and displays runtime configuration for Daneel,
including detected Vault root, XDG paths (~/.config, ~/.cache),
SQLite FTS5 database location, and environment variables.`,
	RunE: func(cmd *cobra.Command, args []string) error { //nolint:revive
		type ConfigInfo struct {
			VaultPath     string              `json:"vault_path"`
			VaultSource   string              `json:"vault_source"`
			ConfigFile    string              `json:"config_file,omitempty"`
			ConfigSource  string              `json:"config_source"`
			ConfigFound   bool                `json:"config_found"`
			DBPath        string              `json:"db_path"`
			DBExists      bool                `json:"db_exists"`
			IndexedDirs   []string            `json:"indexed_dirs"`
			TaxonomyRules map[string][]string `json:"taxonomy"`
			XDG           map[string]string   `json:"xdg_paths"`
			Environment   map[string]string   `json:"environment"`
		}

		// Check DB file status
		_, dbErr := os.Stat(cfg.DBPath)
		dbExists := dbErr == nil

		info := ConfigInfo{
			VaultPath:     cfg.VaultPath,
			VaultSource:   cfg.VaultSource,
			ConfigFile:    cfg.ConfigFile,
			ConfigSource:  cfg.ConfigSource,
			ConfigFound:   cfg.ConfigFile != "",
			DBPath:        cfg.DBPath,
			DBExists:      dbExists,
			IndexedDirs:   cfg.GetIndexedDirs(),
			TaxonomyRules: cfg.GetTaxonomyMap(),
			XDG: map[string]string{
				"config_dir":   config.XDGConfigDir(),
				"cache_dir":    config.XDGCacheDir(),
				"data_dir":     config.XDGDataDir(),
				"global_vault": config.DefaultGlobalVaultPath(),
			},
			Environment: map[string]string{
				"DANEEL_VAULT_PATH": os.Getenv("DANEEL_VAULT_PATH"),
				"DANEEL_CONFIG":     os.Getenv("DANEEL_CONFIG"),
				"DANEEL_DB_PATH":    os.Getenv("DANEEL_DB_PATH"),
			},
		}

		if jsonOut {
			bytes, err := json.MarshalIndent(info, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(bytes))
			return nil
		}

		titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
		keyStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
		valStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
		mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
		sourceBadge := lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)

		fmt.Println(titleStyle.Render("⚙️  Active Daneel Configuration (XDG Standard):"))
		fmt.Println()

		// 1. Vault Root & Source
		fmt.Printf("  %s %s %s\n",
			keyStyle.Render("📂 Vault Root:"),
			valStyle.Render(cfg.VaultPath),
			sourceBadge.Render(fmt.Sprintf("[%s]", cfg.VaultSource)),
		)

		// 2. Config File & Source
		cfgFileStr := cfg.ConfigFile
		if cfgFileStr == "" {
			cfgFileStr = config.GlobalConfigFile() + " (not created)"
		}
		fmt.Printf("  %s %s %s\n",
			keyStyle.Render("📄 Config File:"),
			valStyle.Render(cfgFileStr),
			mutedStyle.Render(fmt.Sprintf("[%s]", cfg.ConfigSource)),
		)

		// 3. Database Path
		dbStatus := lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render(" (not indexed yet)")
		if dbExists {
			dbStatus = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render(" (SQLite database ready)")
		}
		fmt.Printf("  %s %s%s\n",
			keyStyle.Render("🗄️  Database:"),
			valStyle.Render(cfg.DBPath),
			dbStatus,
		)

		// 4. Indexed Directories
		fmt.Printf("  %s %s\n",
			keyStyle.Render("📁 Indexed Dirs:"),
			valStyle.Render(fmt.Sprintf("%v", cfg.GetIndexedDirs())),
		)

		// 5. XDG System Paths
		fmt.Println()
		fmt.Println(keyStyle.Render("🐧 Standard XDG Directories:"))
		fmt.Printf("  • Config: %s\n", valStyle.Render(config.XDGConfigDir()))
		fmt.Printf("  • Cache:  %s\n", valStyle.Render(config.XDGCacheDir()))
		fmt.Printf("  • Data:   %s\n", valStyle.Render(config.XDGDataDir()))

		// 6. Environment Variables
		fmt.Println()
		fmt.Println(keyStyle.Render("🌐 Environment Variables:"))
		for k, v := range info.Environment {
			if v != "" {
				fmt.Printf("  • %s: %s\n", mutedStyle.Render(k), valStyle.Render(v))
			} else {
				fmt.Printf("  • %s: %s\n", mutedStyle.Render(k), mutedStyle.Render("(not set)"))
			}
		}

		fmt.Println()
		return nil
	},
}

var setVaultCmd = &cobra.Command{
	Use:   "set-vault <path>",
	Short: "Sets default global Vault path in ~/.config/daneel/config.yaml",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error { //nolint:revive
		targetVault := args[0]
		absVault, err := filepath.Abs(targetVault)
		if err != nil {
			absVault = targetVault
		}

		cfgDir := config.XDGConfigDir()
		if err := os.MkdirAll(cfgDir, 0750); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", cfgDir, err)
		}

		globalCfgFile := config.GlobalConfigFile()

		// Read existing or create new
		cfgMap := make(map[string]interface{})
		if data, err := os.ReadFile(globalCfgFile); err == nil { //nolint:gosec // intentional
			_ = yaml.Unmarshal(data, &cfgMap)
		}

		cfgMap["vault_path"] = absVault

		out, err := yaml.Marshal(cfgMap)
		if err != nil {
			return err
		}

		if err := os.WriteFile(globalCfgFile, out, 0600); err != nil {
			return fmt.Errorf("failed to save configuration in %s: %w", globalCfgFile, err)
		}

		fmt.Printf("✅ Global default vault configured successfully!\n")
		fmt.Printf("   📂 Vault:  %s\n", absVault)
		fmt.Printf("   📄 Config: %s\n", globalCfgFile)
		return nil
	},
}

func init() {
	configCmd.AddCommand(setVaultCmd)
	RootCmd.AddCommand(configCmd)
}
