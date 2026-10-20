//nolint:revive // package comment not required
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var (
	installAgent  string
	installDryRun bool
)

// AgentHookAdapter defines a pluggable adapter for configuring hooks in an AI agent runner.
type AgentHookAdapter interface {
	Name() string
	Aliases() []string
	Install(dryRun bool) (string, error)
	Uninstall(dryRun bool) (string, error)
}

type claudeAdapter struct{}

func (c *claudeAdapter) Name() string      { return "claude-code" }
func (c *claudeAdapter) Aliases() []string { return []string{"claude", "claude-code"} }
func (c *claudeAdapter) Install(dryRun bool) (string, error) {
	return installClaudeHooks(defaultClaudeSettingsPath(), dryRun)
}
func (c *claudeAdapter) Uninstall(dryRun bool) (string, error) {
	return uninstallClaudeHooks(defaultClaudeSettingsPath(), dryRun)
}

type antigravityAdapter struct{}

func (a *antigravityAdapter) Name() string      { return "antigravity" }
func (a *antigravityAdapter) Aliases() []string { return []string{"agy", "antigravity"} }
func (a *antigravityAdapter) Install(dryRun bool) (string, error) {
	return installAntigravityHooks(defaultAntigravityHooksPath(), dryRun)
}
func (a *antigravityAdapter) Uninstall(dryRun bool) (string, error) {
	return uninstallAntigravityHooks(defaultAntigravityHooksPath(), dryRun)
}

type codexAdapter struct{}

func (c *codexAdapter) Name() string      { return "codex" }
func (c *codexAdapter) Aliases() []string { return []string{"codex", "openai-codex"} }
func (c *codexAdapter) Install(dryRun bool) (string, error) {
	return installCodexHooks(defaultCodexHooksPath(), dryRun)
}
func (c *codexAdapter) Uninstall(dryRun bool) (string, error) {
	return uninstallCodexHooks(defaultCodexHooksPath(), dryRun)
}

var registeredAdapters = []AgentHookAdapter{
	&claudeAdapter{},
	&antigravityAdapter{},
	&codexAdapter{},
}

// getRegisteredAdapters returns all currently registered adapters.
func getRegisteredAdapters() []AgentHookAdapter {
	return registeredAdapters
}

// RegisterAgentHookAdapter registers a new agent adapter for Daneel hooks.
func RegisterAgentHookAdapter(adapter AgentHookAdapter) {
	registeredAdapters = append(registeredAdapters, adapter)
}

func resolveAdapters(target string) []AgentHookAdapter {
	target = strings.ToLower(strings.TrimSpace(target))
	if target == "all" || target == "" {
		return registeredAdapters
	}

	var matches []AgentHookAdapter
	for _, a := range registeredAdapters {
		if strings.EqualFold(a.Name(), target) {
			matches = append(matches, a)
			continue
		}
		for _, alias := range a.Aliases() {
			if strings.EqualFold(alias, target) {
				matches = append(matches, a)
				break
			}
		}
	}
	return matches
}

var hookInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Automatically configure agent runners (Claude Code, Antigravity, Codex) with Daneel hooks",
	RunE: func(cmd *cobra.Command, args []string) error {
		adapters := resolveAdapters(installAgent)
		if len(adapters) == 0 {
			names := make([]string, 0, len(registeredAdapters))
			for _, a := range registeredAdapters {
				names = append(names, a.Name())
			}
			return fmt.Errorf("unknown agent %q. Available: %s, all", installAgent, strings.Join(names, ", "))
		}

		var logs []string
		for _, a := range adapters {
			res, err := a.Install(installDryRun)
			if err != nil {
				return fmt.Errorf("%s install error: %w", a.Name(), err)
			}
			logs = append(logs, res)
		}

		for _, l := range logs {
			fmt.Printf("✔ %s\n", l)
		}
		return nil
	},
}

var hookUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove Daneel hooks from agent configurations",
	RunE: func(cmd *cobra.Command, args []string) error {
		adapters := resolveAdapters(installAgent)
		if len(adapters) == 0 {
			names := make([]string, 0, len(registeredAdapters))
			for _, a := range registeredAdapters {
				names = append(names, a.Name())
			}
			return fmt.Errorf("unknown agent %q. Available: %s, all", installAgent, strings.Join(names, ", "))
		}

		var logs []string
		for _, a := range adapters {
			res, err := a.Uninstall(installDryRun)
			if err != nil {
				return fmt.Errorf("%s uninstall error: %w", a.Name(), err)
			}
			logs = append(logs, res)
		}

		for _, l := range logs {
			fmt.Printf("✔ %s\n", l)
		}
		return nil
	},
}

func init() {
	hookInstallCmd.Flags().StringVarP(&installAgent, "agent", "a", "all", "Agent to install hooks for (claude-code, antigravity, codex, all)")
	hookInstallCmd.Flags().BoolVar(&installDryRun, "dry-run", false, "Preview modifications without writing to disk")

	hookUninstallCmd.Flags().StringVarP(&installAgent, "agent", "a", "all", "Agent to uninstall hooks from (claude-code, antigravity, codex, all)")
	hookUninstallCmd.Flags().BoolVar(&installDryRun, "dry-run", false, "Preview modifications without writing to disk")

	hookCmd.AddCommand(hookInstallCmd)
	hookCmd.AddCommand(hookUninstallCmd)
}

func defaultClaudeSettingsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude", "settings.json")
}

func defaultAntigravityHooksPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	cfgDir := filepath.Join(home, ".gemini", "config")
	if stat, err := os.Stat(cfgDir); err == nil && stat.IsDir() {
		return filepath.Join(cfgDir, "hooks.json")
	}
	return filepath.Join(home, ".gemini", "antigravity-cli", "hooks.json")
}

func defaultCodexHooksPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".codex", "hooks.json")
}

func toInterfaceSlice(val interface{}) []interface{} {
	if val == nil {
		return nil
	}
	if s, ok := val.([]interface{}); ok {
		return s
	}
	return nil
}

func appendMatcherHookIfMissing(existing []interface{}, matcher, command string) []interface{} {
	hookEntry := map[string]interface{}{
		"type":    "command",
		"command": command,
	}

	var cleaned []interface{}
	alreadyPresent := false

	for _, item := range existing {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		// 1. Migrate legacy flat format { "matcher": ..., "command": ... }
		if flatCmd, ok := m["command"].(string); ok {
			delete(m, "command")
			if strings.TrimSpace(flatCmd) == strings.TrimSpace(command) {
				m["hooks"] = append(toInterfaceSlice(m["hooks"]), hookEntry)
				alreadyPresent = true
			} else if !strings.Contains(flatCmd, "daneel hook") {
				m["hooks"] = append(toInterfaceSlice(m["hooks"]), map[string]interface{}{
					"type":    "command",
					"command": flatCmd,
				})
			}
		}

		// 2. Check if command is already in m["hooks"]
		hooksList := toInterfaceSlice(m["hooks"])
		for _, h := range hooksList {
			if hm, ok := h.(map[string]interface{}); ok {
				if cmd, ok := hm["command"].(string); ok && strings.TrimSpace(cmd) == strings.TrimSpace(command) {
					alreadyPresent = true
					break
				}
			}
		}

		// 3. If this matcher group matches and we still need to add our hook
		mMatcher, _ := m["matcher"].(string)
		if mMatcher == matcher && !alreadyPresent {
			m["hooks"] = append(hooksList, hookEntry)
			alreadyPresent = true
		}

		if len(toInterfaceSlice(m["hooks"])) > 0 {
			cleaned = append(cleaned, m)
		}
	}

	if !alreadyPresent {
		cleaned = append(cleaned, map[string]interface{}{
			"matcher": matcher,
			"hooks": []interface{}{
				hookEntry,
			},
		})
	}

	return cleaned
}

func removeDaneelHooksFromMatcherGroups(existing []interface{}) []interface{} {
	var remaining []interface{}
	for _, item := range existing {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		// Clean up legacy flat command if it's daneel
		if flatCmd, ok := m["command"].(string); ok {
			delete(m, "command")
			if !strings.Contains(flatCmd, "daneel hook") {
				m["hooks"] = append(toInterfaceSlice(m["hooks"]), map[string]interface{}{
					"type":    "command",
					"command": flatCmd,
				})
			}
		}

		// Filter the nested "hooks" slice
		hooksList := toInterfaceSlice(m["hooks"])
		var keptHooks []interface{}
		for _, h := range hooksList {
			hm, ok := h.(map[string]interface{})
			if !ok {
				continue
			}
			if cmd, ok := hm["command"].(string); ok && strings.Contains(cmd, "daneel hook") {
				continue
			}
			keptHooks = append(keptHooks, h)
		}

		if len(keptHooks) > 0 {
			m["hooks"] = keptHooks
			remaining = append(remaining, m)
		}
	}
	return remaining
}

func installClaudeHooks(path string, dryRun bool) (string, error) {
	if path == "" {
		return "", fmt.Errorf("could not determine claude settings path")
	}

	data := map[string]interface{}{}
	if b, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(b, &data)
	}

	hooks, _ := data["hooks"].(map[string]interface{})
	if hooks == nil {
		hooks = make(map[string]interface{})
	}

	hooks["SessionStart"] = appendMatcherHookIfMissing(toInterfaceSlice(hooks["SessionStart"]), "", "daneel hook handoff")
	hooks["PostToolUse"] = appendMatcherHookIfMissing(toInterfaceSlice(hooks["PostToolUse"]), "", "daneel hook capture PostToolUse --agent claude-code")
	hooks["SessionEnd"] = appendMatcherHookIfMissing(toInterfaceSlice(hooks["SessionEnd"]), "", "daneel hook capture SessionEnd --agent claude-code")

	data["hooks"] = hooks

	if dryRun {
		return fmt.Sprintf("[dry-run] Claude Code hooks configured at %s", path), nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return "", err
	}

	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(path, append(b, '\n'), 0600); err != nil {
		return "", err
	}

	return fmt.Sprintf("Claude Code hooks installed at %s", path), nil
}

func uninstallClaudeHooks(path string, dryRun bool) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Sprintf("Claude settings not found at %s", path), nil
		}
		return "", err
	}

	var data map[string]interface{}
	if err := json.Unmarshal(b, &data); err != nil {
		return "", err
	}

	hooks, ok := data["hooks"].(map[string]interface{})
	if ok && hooks != nil {
		for _, key := range []string{"SessionStart", "PostToolUse", "SessionEnd"} {
			if slice, ok := hooks[key].([]interface{}); ok {
				cleaned := removeDaneelHooksFromMatcherGroups(slice)
				if len(cleaned) == 0 {
					delete(hooks, key)
				} else {
					hooks[key] = cleaned
				}
			}
		}

		if len(hooks) == 0 {
			delete(data, "hooks")
		}
	}

	if dryRun {
		return fmt.Sprintf("[dry-run] Claude Code hooks removed from %s", path), nil
	}

	out, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(path, append(out, '\n'), 0600); err != nil {
		return "", err
	}

	return fmt.Sprintf("Claude Code hooks uninstalled from %s", path), nil
}

func installAntigravityHooks(path string, dryRun bool) (string, error) {
	if path == "" {
		return "", fmt.Errorf("could not determine antigravity hooks path")
	}

	data := map[string]interface{}{}
	if b, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(b, &data)
	}

	delete(data, "daneel-hooks")

	// Antigravity official specification (agy-customizations / docs/hooks.md):
	// Top-level key is the hook name (e.g. "daneel").
	// PreInvocation: flat list of handler objects.
	// PostToolUse: grouped list of matcher objects.
	// Stop: flat list of handler objects.
	data["daneel"] = map[string]interface{}{
		"PreInvocation": []interface{}{
			map[string]interface{}{
				"type":    "command",
				"command": "daneel hook handoff",
			},
		},
		"PostToolUse": []interface{}{
			map[string]interface{}{
				"matcher": "*",
				"hooks": []interface{}{
					map[string]interface{}{
						"type":    "command",
						"command": "daneel hook capture PostToolUse --agent antigravity",
					},
				},
			},
		},
		"Stop": []interface{}{
			map[string]interface{}{
				"type":    "command",
				"command": "daneel hook capture SessionEnd --agent antigravity",
			},
		},
	}

	if dryRun {
		return fmt.Sprintf("[dry-run] Antigravity hooks configured at %s", path), nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return "", err
	}

	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(path, append(b, '\n'), 0600); err != nil {
		return "", err
	}

	return fmt.Sprintf("Antigravity hooks installed at %s", path), nil
}

func uninstallAntigravityHooks(path string, dryRun bool) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Sprintf("Antigravity hooks not found at %s", path), nil
		}
		return "", err
	}

	var data map[string]interface{}
	if err := json.Unmarshal(b, &data); err != nil {
		return "", err
	}

	delete(data, "daneel")
	delete(data, "daneel-hooks")

	if dryRun {
		return fmt.Sprintf("[dry-run] Antigravity hooks removed from %s", path), nil
	}

	if len(data) == 0 {
		_ = os.Remove(path)
		return fmt.Sprintf("Antigravity hooks uninstalled from %s", path), nil
	}

	out, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(path, append(out, '\n'), 0600); err != nil {
		return "", err
	}

	return fmt.Sprintf("Antigravity hooks uninstalled from %s", path), nil
}

func installCodexHooks(path string, dryRun bool) (string, error) {
	if path == "" {
		return "", fmt.Errorf("could not determine codex hooks path")
	}

	data := map[string]interface{}{}
	if b, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(b, &data)
	}

	hooks, _ := data["hooks"].(map[string]interface{})
	if hooks == nil {
		hooks = make(map[string]interface{})
	}

	// Codex CLI official specification uses standard matcher groups under "hooks":
	hooks["SessionStart"] = appendMatcherHookIfMissing(toInterfaceSlice(hooks["SessionStart"]), "", "daneel hook handoff")
	hooks["PostToolUse"] = appendMatcherHookIfMissing(toInterfaceSlice(hooks["PostToolUse"]), "*", "daneel hook capture PostToolUse --agent codex")
	const sessionEndCommand = "daneel hook capture SessionEnd --agent codex"
	sessionEndGroups := appendMatcherHookIfMissing(toInterfaceSlice(hooks["SessionEnd"]), "", sessionEndCommand)
	for _, group := range sessionEndGroups {
		groupMap, ok := group.(map[string]interface{})
		if !ok {
			continue
		}
		for _, handler := range toInterfaceSlice(groupMap["hooks"]) {
			handlerMap, ok := handler.(map[string]interface{})
			if !ok {
				continue
			}
			command, _ := handlerMap["command"].(string)
			if strings.TrimSpace(command) == sessionEndCommand {
				handlerMap["timeout"] = 3
			}
		}
	}
	hooks["SessionEnd"] = sessionEndGroups
	delete(hooks, "ToolUse")

	data["hooks"] = hooks

	if dryRun {
		return fmt.Sprintf("[dry-run] Codex hooks configured at %s", path), nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return "", err
	}

	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(path, append(b, '\n'), 0600); err != nil {
		return "", err
	}

	return fmt.Sprintf("Codex hooks installed at %s", path), nil
}

func uninstallCodexHooks(path string, dryRun bool) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Sprintf("Codex configuration not found at %s", path), nil
		}
		return "", err
	}

	var data map[string]interface{}
	if err := json.Unmarshal(b, &data); err != nil {
		return "", err
	}

	hooks, ok := data["hooks"].(map[string]interface{})
	if ok && hooks != nil {
		for _, key := range []string{"SessionStart", "PostToolUse", "SessionEnd", "ToolUse"} {
			if slice, ok := hooks[key].([]interface{}); ok {
				cleaned := removeDaneelHooksFromMatcherGroups(slice)
				if len(cleaned) == 0 {
					delete(hooks, key)
				} else {
					hooks[key] = cleaned
				}
			} else if cmd, ok := hooks[key].(string); ok && strings.Contains(cmd, "daneel hook") {
				delete(hooks, key)
			}
		}
		if len(hooks) == 0 {
			delete(data, "hooks")
		}
	}

	if dryRun {
		return fmt.Sprintf("[dry-run] Codex hooks removed from %s", path), nil
	}

	out, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(path, append(out, '\n'), 0600); err != nil {
		return "", err
	}

	return fmt.Sprintf("Codex hooks uninstalled from %s", path), nil
}
