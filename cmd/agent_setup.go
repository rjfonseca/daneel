//nolint:revive // package comment not required
package cmd

import (
	"bufio"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/rjfonseca/daneel/internal/config"
)

type knownAgentCandidate struct {
	Name    string
	Command string
	Desc    string
	Env     map[string]string
}

var defaultAgentCandidates = []knownAgentCandidate{
	{
		Name:    "agy",
		Command: "agy",
		Desc:    "Antigravity CLI (agy)",
		Env:     map[string]string{"TERM": "xterm-256color"},
	},
	{
		Name:    "claude",
		Command: "claude",
		Desc:    "Claude Code CLI",
	},
	{
		Name:    "codex",
		Command: "codex",
		Desc:    "OpenAI Codex CLI",
	},
	{
		Name:    "gemini",
		Command: "gemini",
		Desc:    "Gemini CLI",
	},
	{
		Name:    "aider",
		Command: "aider",
		Desc:    "Aider AI Pair Programming",
	},
}

func detectInstalledAgentCandidates() []knownAgentCandidate {
	return detectAgentCandidatesWithLookup(exec.LookPath)
}

func detectAgentCandidatesWithLookup(lookPath func(string) (string, error)) []knownAgentCandidate {
	var found []knownAgentCandidate
	for _, c := range defaultAgentCandidates {
		if _, err := lookPath(c.Command); err == nil {
			found = append(found, c)
		}
	}
	return found
}

func buildNonInteractiveChatConfig(detected []knownAgentCandidate) config.ChatConfig {
	if len(detected) == 0 {
		return config.ChatConfig{}
	}

	profiles := make(map[string]config.ProfileConfig, len(detected))
	for _, c := range detected {
		profiles[c.Name] = config.ProfileConfig{
			Command: c.Command,
			Env:     c.Env,
		}
	}

	defaultName := ""
	if len(detected) == 1 {
		defaultName = detected[0].Name
	}

	return config.ChatConfig{
		Default:  defaultName,
		Profiles: profiles,
	}
}

func promptChatConfig(reader *bufio.Reader, interactive bool) config.ChatConfig {
	detected := detectInstalledAgentCandidates()
	accentStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214"))

	if !interactive {
		chatCfg := buildNonInteractiveChatConfig(detected)
		if len(detected) > 1 {
			fmt.Println(warnStyle.Render("⚠️  Multiple AI agent CLIs detected; leaving chat.default empty. Set chat.default in .daneel.yaml or run 'daneel chat'."))
		}
		return chatCfg
	}

	chatCfg := buildNonInteractiveChatConfig(detected)

	if len(detected) == 0 {
		fmt.Println(mutedStyle.Render("No known AI agent CLI detected in PATH (agy, claude, codex, gemini, aider)."))
		customCmd := promptString(reader, "🤖 AI Agent Command (leave empty to configure later)", "")
		if customCmd == "" {
			return config.ChatConfig{}
		}
		name := customProfileName(customCmd)
		return config.ChatConfig{
			Default: name,
			Profiles: map[string]config.ProfileConfig{
				name: {Command: customCmd},
			},
		}
	}

	if len(detected) == 1 {
		fmt.Printf("%s Detected AI agent CLI: %s\n", accentStyle.Render("🔍"), detected[0].Desc)
		choice := promptString(reader, "🤖 Default AI Agent Profile", detected[0].Name)
		if choice == "" || choice == detected[0].Name {
			chatCfg.Default = detected[0].Name
			return chatCfg
		}
		name := customProfileName(choice)
		chatCfg.Profiles[name] = config.ProfileConfig{Command: choice}
		chatCfg.Default = name
		return chatCfg
	}

	fmt.Printf("%s Multiple AI agent CLIs detected on your system:\n", accentStyle.Render("🔍"))
	for i, ag := range detected {
		fmt.Printf("   [%d] %s (%s)\n", i+1, ag.Name, ag.Desc)
	}
	choice := promptString(reader, "🤖 Select default AI agent (number, name, or Enter to leave unset)", "")
	if choice == "" {
		chatCfg.Default = ""
		return chatCfg
	}

	if idx, err := strconv.Atoi(choice); err == nil && idx >= 1 && idx <= len(detected) {
		chatCfg.Default = detected[idx-1].Name
		return chatCfg
	}

	if _, exists := chatCfg.Profiles[choice]; exists {
		chatCfg.Default = choice
		return chatCfg
	}

	name := customProfileName(choice)
	chatCfg.Profiles[name] = config.ProfileConfig{Command: choice}
	chatCfg.Default = name
	return chatCfg
}

func customProfileName(cmd string) string {
	fields := strings.Fields(cmd)
	if len(fields) == 0 {
		return "custom"
	}
	return fields[0]
}
