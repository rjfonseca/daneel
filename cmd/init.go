//nolint:revive // package comment not required
package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/rjfonseca/daneel/internal/config"
)

var (
	initGlobal bool
	initLocal  bool
	initYes    bool
)

var initCmd = &cobra.Command{
	Use:   "init [directory]",
	Short: "Initializes a Second Brain Vault with agentic auto-discovery and interactive wizard",
	Long: `🧠 Daneel Init: Agentic Second Brain Onboarding & Initialization

Inspects the local development environment (git config, ~/Work workspace, and repository remotes)
to automatically infer identity, tech stacks, privacy guardrails, and telemetry rules.
Renders an interactive wizard for verification and scaffolds the complete OKF note directory structure,
master MOCs, and semantic .daneel.yaml configuration.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error { //nolint:revive
		var targetDir string

		if initGlobal {
			if len(args) > 0 {
				targetDir = args[0]
			} else {
				targetDir = config.DefaultGlobalVaultPath()
			}
		} else {
			if len(args) > 0 {
				targetDir = args[0]
			} else {
				targetDir = "."
			}
		}

		absTarget, err := filepath.Abs(targetDir)
		if err != nil {
			return err
		}

		// 1. Agentic Inference Phase (Auto-Discovery)
		inferredName := getGitConfig("user.name", "Engineer")
		inferredRole := "Staff Software Engineer"
		inferredWorkDir := inferWorkDir()
		inferredGitHost := "github.com"
		inferredBlockedDomains, inferredBlockedKeywords := inferBlockedEntities(absTarget, inferredWorkDir)

		// 2. Interactive Wizard (if interactive and !initYes)
		isInteractive := !initYes && isTerminal(os.Stdin)

		headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
		accentStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
		mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
		successStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("82"))

		if isInteractive {
			fmt.Println()
			fmt.Println(headerStyle.Render("🧠 Daneel Vault Initialization Wizard"))
			fmt.Println(mutedStyle.Render("Semantic configuration and scaffolding for your Second Brain."))
			fmt.Println(strings.Repeat("─", 65))
			fmt.Println()

			reader := bufio.NewReader(os.Stdin)

			inferredName = promptString(reader, "👤 Owner Name", inferredName)
			inferredRole = promptString(reader, "💼 Role / Title", inferredRole)
			inferredWorkDir = promptString(reader, "📂 Code Workspace Directory", inferredWorkDir)

			domainPrompt := strings.Join(inferredBlockedDomains, ", ")
			domainInput := promptString(reader, "🛡️  Corporate Blocked Domains for Public Notes", domainPrompt)
			if domainInput != "" {
				inferredBlockedDomains = splitCommaOrSpace(domainInput)
			}

			kwPrompt := strings.Join(inferredBlockedKeywords, ", ")
			kwInput := promptString(reader, "🔒 Company Names / Confidential Keywords", kwPrompt)
			if kwInput != "" {
				inferredBlockedKeywords = splitCommaOrSpace(kwInput)
			}
			fmt.Println()
		}

		// 3. Create standard OKF folder structure
		categories := config.DefaultCategories()
		dirs := []string{
			"daily",
			"inbox",
			"meta/adrs",
			"meta/guias",
			"notes/carreira/avaliacoes",
			"notes/carreira/retrospectivas",
			"sources/reunioes",
			"sources/relatorios",
			"sources/governanca",
			"templates",
			"public/content",
		}

		for _, cat := range categories {
			dirs = append(dirs, filepath.Join("notes", cat.ID))
		}

		for _, d := range dirs {
			fullPath := filepath.Join(absTarget, d)
			if err := os.MkdirAll(fullPath, 0750); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", d, err)
			}
		}

		// 4. Create Master MOC (notes/moc-indice-geral.md)
		mocPath := filepath.Join(absTarget, "notes", "moc-indice-geral.md")
		if _, err := os.Stat(mocPath); os.IsNotExist(err) {
			var sb strings.Builder
			sb.WriteString("---\n")
			sb.WriteString("type: process\n")
			sb.WriteString("description: Master Map of Content (MOC) — Entry point and global index for Second Brain.\n")
			sb.WriteString("tags:\n")
			sb.WriteString("  - notes/moc\n")
			sb.WriteString("  - meta/guides\n")
			sb.WriteString("---\n\n")
			sb.WriteString("# 🧭 Master MOC: Second Brain Global Index\n\n")
			fmt.Fprintf(&sb, "Welcome to the **Second Brain** of %s (%s)!\n\n", inferredName, inferredRole)
			sb.WriteString("## 🏛️ Knowledge & Engineering Domains\n\n")
			sb.WriteString("| Domain | Thematic MOC | Description & Focus |\n")
			sb.WriteString("| :--- | :--- | :--- |\n")
			for _, cat := range categories {
				fmt.Fprintf(&sb, "| **%s** | [%s](%s/moc-%s.md) | %s |\n", cat.Title, cat.Title, cat.ID, cat.ID, cat.Desc)
			}
			sb.WriteString("\n---\n\n")
			sb.WriteString("## 🏢 Leadership, Network & Career\n\n")
			sb.WriteString("- [MOC: Career](carreira/moc-carreira.md)\n")
			sb.WriteString("- [MOC: People](pessoas/moc-pessoas.md)\n")
			sb.WriteString("- [Weekly Summary & Active Backlog](carreira/sumario-semanal-atividades-impactos-2026.md)\n")

			_ = os.WriteFile(mocPath, []byte(sb.String()), 0600)
		}

		// 5. Create category MOCs
		for _, cat := range categories {
			catMoc := filepath.Join(absTarget, "notes", cat.ID, fmt.Sprintf("moc-%s.md", cat.ID))
			if _, err := os.Stat(catMoc); os.IsNotExist(err) {
				catContent := fmt.Sprintf(`---
type: moc
description: Thematic Map of Content for %s.
tags:
  - notes/moc
---

# 🗺️ MOC: %s

> **Description:** %s  
> **Master Index:** [Master MOC](../moc-indice-geral.md)

## 📋 Notes & Articles
`, cat.Title, cat.Title, cat.Desc)
				_ = os.WriteFile(catMoc, []byte(catContent), 0600)
			}
		}

		// 6. Create .gitignore
		gitIgnorePath := filepath.Join(absTarget, ".gitignore")
		if _, err := os.Stat(gitIgnorePath); os.IsNotExist(err) {
			ignoreContent := `# Daneel Local Cache and SQLite Databases
.daneel/cache/
*.db
*.db-wal
*.db-shm

# OS Temporary Files
.DS_Store
Thumbs.db
*.tmp
`
			_ = os.WriteFile(gitIgnorePath, []byte(ignoreContent), 0600)
		}

		// 7. Scaffold AGENTS.md and Default Skills
		if err := scaffoldAgentsAndSkills(absTarget); err != nil {
			return fmt.Errorf("failed to scaffold agents and skills: %w", err)
		}

		// 8. Generate Semantic Config Object
		semanticCfg := config.Config{
			Vault: config.VaultConfig{
				Title:       "Second Brain",
				Owner:       inferredName,
				Role:        inferredRole,
				IndexedDirs: config.DefaultIndexedDirs(),
				Categories:  categories,
			},
			Workspace: config.WorkspaceConfig{
				WorkDir:        inferredWorkDir,
				DefaultGitHost: inferredGitHost,
			},
			Publishing: config.PublishingConfig{
				ContentDir: "public/content",
				Block: config.BlockConfig{
					Domains:  inferredBlockedDomains,
					Keywords: inferredBlockedKeywords,
				},
			},
			Telemetry: config.TelemetryConfig{
				Taxonomy: config.DefaultTaxonomyArchetypes(),
			},
			Radar: config.DefaultRadarItems(),
			Evaluation: config.EvaluationConfig{
				DossierTitle: "Performance Review & Impact Dossier",
				StrategicDeliveries: []string{
					"Delivery 1 (Customize me)",
					"Delivery 2 (Customize me)",
				},
			},
			Dream: config.DreamConfig{
				AutoFix: true,
				Routing: config.DefaultRoutingMap(),
			},
			Chat: config.DefaultChatConfig(),
		}

		outYAML, err := yaml.Marshal(semanticCfg)
		if err != nil {
			return fmt.Errorf("failed to serialize YAML configuration: %w", err)
		}

		if initGlobal {
			cfgDir := config.XDGConfigDir()
			_ = os.MkdirAll(cfgDir, 0750)
			globalFile := config.GlobalConfigFile()
			semanticCfg.VaultPath = absTarget
			outYAML, _ = yaml.Marshal(semanticCfg)
			_ = os.WriteFile(globalFile, outYAML, 0600)

			fmt.Println(successStyle.Render("✨ Global Vault initialized and configured successfully!"))
			fmt.Printf("  %s %s\n", accentStyle.Render("📂 Vault Root:"), absTarget)
			fmt.Printf("  %s %s\n", accentStyle.Render("📄 Config:    "), globalFile)
			fmt.Printf("  %s %s\n\n", accentStyle.Render("🗄️  Database:  "), filepath.Join(config.XDGCacheDir(), "vault_index.db"))
		} else {
			_ = os.MkdirAll(filepath.Join(absTarget, ".daneel", "cache"), 0750)
			daneelYAML := filepath.Join(absTarget, ".daneel.yaml")
			_ = os.WriteFile(daneelYAML, outYAML, 0600)

			fmt.Println(successStyle.Render("✨ Local Vault initialized successfully!"))
			fmt.Printf("  %s %s\n", accentStyle.Render("📂 Vault Root:"), absTarget)
			fmt.Printf("  %s %s\n", accentStyle.Render("📄 Config:    "), daneelYAML)
			fmt.Printf("  %s %s\n\n", accentStyle.Render("🗄️  Database:  "), filepath.Join(absTarget, ".daneel", "cache", "vault_index.db"))
		}

		fmt.Println("• OKF folder structure generated with thematic MOCs")
		fmt.Println("• AI Persona (AGENTS.md) and default skills scaffolded")
		fmt.Println("• Semantic configuration written (.daneel.yaml)")
		fmt.Println("• Isolated SQLite cache ready for fast indexing")
		fmt.Println()
		fmt.Println(mutedStyle.Render("Run 'daneel search' or launch the MCP server with 'daneel mcp'."))
		return nil
	},
}

func getGitConfig(key, fallback string) string {
	cmd := exec.Command("git", "config", "--get", key) //nolint:gosec // intentional
	if out, err := cmd.Output(); err == nil {
		val := strings.TrimSpace(string(out))
		if val != "" {
			return val
		}
	}
	return fallback
}

func inferWorkDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "~/Work"
	}
	candidates := []string{"Work", "Projects", "dev", "workspace", "src"}
	for _, cand := range candidates {
		p := filepath.Join(home, cand)
		if stat, err := os.Stat(p); err == nil && stat.IsDir() {
			return fmt.Sprintf("~/%s", cand)
		}
	}
	return "~/Work"
}

func inferBlockedEntities(targetDir, workDir string) ([]string, []string) { //nolint:revive
	var domains []string
	var keywords []string

	// Check git remotes from current targetDir
	cmd := exec.Command("git", "-C", targetDir, "remote", "-v") //nolint:gosec // intentional
	if out, err := cmd.Output(); err == nil {
		extractEntitiesFromRemotes(string(out), &domains, &keywords)
	}

	// If none found, fallback to defaults
	if len(domains) == 0 {
		domains = []string{"internal.corp", "gitlab.com/internal-org"}
	}
	if len(keywords) == 0 {
		keywords = []string{"confidential", "internal-project"}
	}

	return domains, keywords
}

func extractEntitiesFromRemotes(remotes string, domains *[]string, keywords *[]string) {
	re := regexp.MustCompile(`(?:git@|https://)([^:/]+)[:/]([^/\s]+)`)
	matches := re.FindAllStringSubmatch(remotes, -1)
	for _, m := range matches {
		if len(m) >= 3 {
			host := m[1]
			org := m[2]
			if host != "github.com" && !contains(*domains, host) {
				*domains = append(*domains, host)
			}
			if org != "" && org != "github" && !contains(*keywords, org) {
				*keywords = append(*keywords, org)
			}
		}
	}
}

func promptString(r *bufio.Reader, label, defaultValue string) string {
	accent := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))

	fmt.Printf("%s %s: ", accent.Render("?"), label)
	if defaultValue != "" {
		fmt.Printf("%s ", muted.Render(fmt.Sprintf("[%s]", defaultValue)))
	}

	text, _ := r.ReadString('\n')
	text = strings.TrimSpace(text)
	if text == "" {
		return defaultValue
	}
	return text
}

func splitCommaOrSpace(s string) []string {
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == ';'
	})
	var result []string
	for _, p := range parts {
		clean := strings.TrimSpace(p)
		if clean != "" {
			result = append(result, clean)
		}
	}
	return result
}

func contains(slice []string, val string) bool {
	for _, s := range slice {
		if strings.EqualFold(s, val) {
			return true
		}
	}
	return false
}

func isTerminal(f *os.File) bool {
	stat, err := f.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
}

func init() {
	initCmd.Flags().BoolVarP(&initGlobal, "global", "g", false, "Initialize as default global vault (registers in ~/.config/daneel/config.yaml)")
	initCmd.Flags().BoolVarP(&initLocal, "local", "l", false, "Initialize as local vault (creates .daneel.yaml in target directory)")
	initCmd.Flags().BoolVarP(&initYes, "yes", "y", false, "Non-interactive mode (applies auto-inferred values and defaults)")
	RootCmd.AddCommand(initCmd)
}
