//nolint:revive // package comment not required
package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/rjfonseca/daneel/internal/config"
	"github.com/rjfonseca/daneel/internal/search"
)

var (
	chatProfile string
	chatNoIndex bool
)

var chatCmd = &cobra.Command{
	Use:   "chat [flags] [--] [extra_arguments...]",
	Short: "Launches an interactive session with the configured AI agent in the Vault directory",
	Long: `🧠 Daneel Chat: Native AI Agent Session Runner & Process Replacement

Executes silent, fast pre-indexing of the Vault into SQLite FTS5 (< 5ms),
then replaces the Daneel Go process with the configured AI Agent (e.g. agy, Claude, Codex, etc.)
via syscall.Exec on Linux, ensuring zero memory overhead and 100% native TTY signal passthrough.

Usage Examples:
  $ daneel chat
  $ daneel chat -p claude
  $ daneel chat "what is the status of the Gateway API migration?"
  $ daneel chat -- --dangerously-skip-permissions
`,
	DisableFlagParsing: false,
	RunE: func(cmd *cobra.Command, args []string) error {
		// 1. Silent Fast Indexing (timeout 500ms so it never blocks startup)
		if !chatNoIndex {
			cmdCtx := cmd.Context()
			if cmdCtx == nil {
				cmdCtx = context.Background()
			}
			ctx, cancel := context.WithTimeout(cmdCtx, 500*time.Millisecond)
			if engine, err := search.Open(cfg.DBPath, cfg.VaultPath); err == nil {
				_, _, _ = engine.IndexVault(ctx, cfg.GetIndexedDirs(), false)
				engine.Close() //nolint:errcheck,gosec // fully read or safe to ignore
			}
			cancel()
		}

		// 2. Resolve or Configure Chat Profile
		if len(cfg.Chat.Profiles) == 0 && len(cfg.LegacyAssistant.Profiles) == 0 {
			if isTerminal(os.Stdin) {
				if err := runChatSetupWizard(); err != nil {
					return err
				}
				// Reload config
				if reloaded, err := config.LoadConfig(cfg.VaultPath, cfg.ConfigFile, cfg.IsGlobal); err == nil {
					cfg = reloaded
				}
			}
		}

		prof, activeName, err := cfg.GetChatProfile(chatProfile)
		if err != nil {
			return fmt.Errorf("failed to resolve chat profile: %w", err)
		}

		if strings.TrimSpace(prof.Command) == "" {
			return fmt.Errorf("command for profile '%s' is empty in .daneel.yaml", activeName)
		}

		// 3. Tokenize Command Line and Append User Arguments
		commandTokens, err := splitCommand(prof.Command)
		if err != nil {
			return fmt.Errorf("failed to parse command for profile '%s': %w", activeName, err)
		}
		if len(commandTokens) == 0 {
			return fmt.Errorf("no command specified in profile '%s'", activeName)
		}

		// Append extra args passed from CLI
		if len(args) > 0 {
			commandTokens = append(commandTokens, args...)
		}

		// Expand ~ in command tokens
		for i, tok := range commandTokens {
			commandTokens[i] = expandHomeInToken(tok)
		}

		binName := commandTokens[0]
		binPath, err := exec.LookPath(binName)
		if err != nil {
			return fmt.Errorf("executable '%s' for profile '%s' not found in PATH", binName, activeName)
		}

		// 4. Prepare Environment Variables
		envMap := make(map[string]string)
		for _, e := range os.Environ() {
			parts := strings.SplitN(e, "=", 2)
			if len(parts) == 2 {
				envMap[parts[0]] = parts[1]
			}
		}

		// Apply custom profile env vars
		for k, v := range prof.Env {
			envMap[k] = v
		}

		var envList []string
		for k, v := range envMap {
			envList = append(envList, fmt.Sprintf("%s=%s", k, v))
		}

		// 5. Change Working Directory to Vault Root
		if cfg.VaultPath != "" {
			if err := os.Chdir(cfg.VaultPath); err != nil {
				return fmt.Errorf("failed to change to vault directory (%s): %w", cfg.VaultPath, err)
			}
		}

		// 6. Execute via syscall.Exec (Process Replacement)
		partnerName := "Partner"
		if cfg != nil && cfg.Vault.Owner != "" {
			partnerName = fmt.Sprintf("Partner %s", strings.Fields(cfg.Vault.Owner)[0])
		}
		greetingStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("75")).Bold(true)
		fmt.Printf("%s Initializing positronic session for %s with profile '%s'...\n\n",
			greetingStyle.Render("🧠 [R. DANEEL OLIVAW]"), partnerName, activeName)

		execErr := syscall.Exec(binPath, commandTokens, envList) //nolint:gosec // intentional
		if execErr != nil {
			// Fallback to os/exec.Command if syscall.Exec fails (e.g. non-unix)
			fallbackCmd := exec.Command(binPath, commandTokens[1:]...) //nolint:gosec // intentional
			fallbackCmd.Env = envList
			fallbackCmd.Stdin = os.Stdin
			fallbackCmd.Stdout = os.Stdout
			fallbackCmd.Stderr = os.Stderr
			return fallbackCmd.Run()
		}

		return nil
	},
}

func runChatSetupWizard() error {
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	accentStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	successStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("82"))

	fmt.Println()
	fmt.Println(headerStyle.Render("🤖 Daneel Chat Setup Wizard"))
	fmt.Println(mutedStyle.Render("No chat profile configured in .daneel.yaml. Setting up default assistant profile..."))
	fmt.Println()

	detected := detectInstalledAgents()
	defaultProfile := config.DefaultChatConfig()

	if len(detected) > 0 {
		fmt.Printf("%s Detected AI agents on your system:\n", accentStyle.Render("🔍"))
		for _, ag := range detected {
			fmt.Printf("   • %s\n", ag)
		}
		fmt.Println()
	}

	// Write default configuration into .daneel.yaml if present
	if cfg.ConfigFile != "" {
		data, err := os.ReadFile(cfg.ConfigFile)
		if err == nil {
			var rawMap map[string]interface{}
			if err := yaml.Unmarshal(data, &rawMap); err == nil {
				rawMap["chat"] = defaultProfile
				if out, err := yaml.Marshal(rawMap); err == nil {
					_ = os.WriteFile(cfg.ConfigFile, out, 0600)
					fmt.Println(successStyle.Render("✅ Chat configuration saved successfully to .daneel.yaml!"))
					fmt.Println()
				}
			}
		}
	}

	return nil
}

func detectInstalledAgents() []string {
	var found []string
	candidates := []struct {
		name string
		desc string
	}{
		{"agy", "Antigravity CLI (agy)"},
		{"claude", "Claude Code CLI"},
		{"codex", "OpenAI Codex CLI"},
		{"aider", "Aider AI Pair Programming"},
		{"gh", "GitHub Copilot CLI"},
	}

	for _, c := range candidates {
		if _, err := exec.LookPath(c.name); err == nil {
			found = append(found, c.desc)
		}
	}
	return found
}

func splitCommand(s string) ([]string, error) {
	var tokens []string
	var cur strings.Builder
	inSingle := false
	inDouble := false
	escaped := false

	for i := 0; i < len(s); i++ {
		c := s[i]

		if escaped {
			cur.WriteByte(c)
			escaped = false
			continue
		}

		if c == '\\' && !inSingle {
			escaped = true
			continue
		}

		if c == '\'' && !inDouble {
			inSingle = !inSingle
			continue
		}

		if c == '"' && !inSingle {
			inDouble = !inDouble
			continue
		}

		if (c == ' ' || c == '\t' || c == '\n') && !inSingle && !inDouble {
			if cur.Len() > 0 {
				tokens = append(tokens, cur.String())
				cur.Reset()
			}
			continue
		}

		cur.WriteByte(c)
	}

	if cur.Len() > 0 {
		tokens = append(tokens, cur.String())
	}

	return tokens, nil
}

func expandHomeInToken(token string) string {
	if strings.HasPrefix(token, "--") && strings.Contains(token, "=") {
		parts := strings.SplitN(token, "=", 2)
		val := parts[1]
		if strings.HasPrefix(val, "\"") && strings.HasSuffix(val, "\"") {
			val = strings.Trim(val, "\"")
		}
		if strings.HasPrefix(val, "'") && strings.HasSuffix(val, "'") {
			val = strings.Trim(val, "'")
		}
		val = expandHome(val)
		return fmt.Sprintf("%s=%s", parts[0], val)
	}

	return expandHome(token)
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") || path == "~" {
		home, err := os.UserHomeDir()
		if err == nil {
			if path == "~" {
				return home
			}
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

func init() {
	chatCmd.Flags().StringVarP(&chatProfile, "profile", "p", "", "Name of the chat profile to execute (e.g. agy, claude, plain_agy)")
	chatCmd.Flags().BoolVar(&chatNoIndex, "no-index", false, "Skip instant SQLite pre-indexing before launching chat")
	RootCmd.AddCommand(chatCmd)
}
