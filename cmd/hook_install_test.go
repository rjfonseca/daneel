package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rjfonseca/daneel/internal/spooler"
)

func TestHookInstall_ClaudeCode(t *testing.T) {
	tempHome := t.TempDir()
	claudeDir := filepath.Join(tempHome, ".claude")
	if err := os.MkdirAll(claudeDir, 0750); err != nil {
		t.Fatalf("failed to create fake .claude dir: %v", err)
	}

	initialSettings := `{"theme": "dark"}`
	settingsPath := filepath.Join(claudeDir, "settings.json")
	if err := os.WriteFile(settingsPath, []byte(initialSettings), 0600); err != nil {
		t.Fatalf("failed to write initial settings: %v", err)
	}

	// Run install
	res, err := installClaudeHooks(settingsPath, false)
	if err != nil {
		t.Fatalf("installClaudeHooks failed: %v", err)
	}
	if !strings.Contains(res, "installed") {
		t.Errorf("expected result to indicate installation, got %q", res)
	}

	// Verify content
	updated, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("failed to read updated settings: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(updated, &m); err != nil {
		t.Fatalf("corrupted settings JSON: %v", err)
	}

	// Check preserved settings
	if m["theme"] != "dark" {
		t.Errorf("expected theme 'dark' preserved, got %v", m["theme"])
	}

	// Check hooks
	hooks, ok := m["hooks"].(map[string]interface{})
	if !ok {
		t.Fatalf("hooks field missing or not an object")
	}
	for _, evt := range []string{"SessionStart", "PostToolUse", "SessionEnd"} {
		groups, ok := hooks[evt].([]interface{})
		if !ok || len(groups) == 0 {
			t.Fatalf("%s hook missing or not a non-empty slice", evt)
		}
		group, ok := groups[0].(map[string]interface{})
		if !ok {
			t.Fatalf("%s group is not a map", evt)
		}
		if _, hasMatcher := group["matcher"]; !hasMatcher {
			t.Errorf("%s group missing 'matcher' key", evt)
		}
		// In Claude Code schema, 'hooks' MUST be an array of hook entries, NOT top-level 'command'
		if _, hasFlatCmd := group["command"]; hasFlatCmd {
			t.Errorf("%s group has invalid top-level 'command' key", evt)
		}
		innerHooks, ok := group["hooks"].([]interface{})
		if !ok || len(innerHooks) == 0 {
			t.Fatalf("%s group missing valid 'hooks' array", evt)
		}
		hookEntry, ok := innerHooks[0].(map[string]interface{})
		if !ok {
			t.Fatalf("%s inner hook entry is not a map", evt)
		}
		if hookEntry["type"] != "command" {
			t.Errorf("expected type 'command', got %v", hookEntry["type"])
		}
		cmdStr, _ := hookEntry["command"].(string)
		if !strings.Contains(cmdStr, "daneel hook") {
			t.Errorf("expected command to invoke daneel hook, got %q", cmdStr)
		}
	}

	// Test migration from legacy flat format
	legacySettings := `{
		"hooks": {
			"SessionStart": [
				{"matcher": "", "command": "daneel hook handoff"}
			]
		}
	}`
	legacyPath := filepath.Join(claudeDir, "legacy_settings.json")
	if err := os.WriteFile(legacyPath, []byte(legacySettings), 0600); err != nil {
		t.Fatalf("failed to write legacy settings: %v", err)
	}
	if _, err := installClaudeHooks(legacyPath, false); err != nil {
		t.Fatalf("failed to install over legacy settings: %v", err)
	}
	migratedBytes, _ := os.ReadFile(legacyPath)
	var migratedMap map[string]interface{}
	_ = json.Unmarshal(migratedBytes, &migratedMap)
	migratedHooks := migratedMap["hooks"].(map[string]interface{})
	migratedGroups := migratedHooks["SessionStart"].([]interface{})
	if len(migratedGroups) != 1 {
		t.Fatalf("expected 1 group after migration, got %d", len(migratedGroups))
	}
	migratedGroup := migratedGroups[0].(map[string]interface{})
	if _, hasFlat := migratedGroup["command"]; hasFlat {
		t.Errorf("legacy group still has flat command after migration")
	}
	migratedInner := migratedGroup["hooks"].([]interface{})
	if len(migratedInner) != 1 {
		t.Fatalf("expected 1 inner hook after migration, got %d", len(migratedInner))
	}

	// Test Uninstall
	unres, err := uninstallClaudeHooks(settingsPath, false)
	if err != nil {
		t.Fatalf("uninstallClaudeHooks failed: %v", err)
	}
	if !strings.Contains(unres, "uninstalled") {
		t.Errorf("expected result to indicate uninstallation, got %q", unres)
	}

	uninstalledBytes, _ := os.ReadFile(settingsPath)
	var uninstalledMap map[string]interface{}
	_ = json.Unmarshal(uninstalledBytes, &uninstalledMap)
	if uninstalledMap["theme"] != "dark" {
		t.Errorf("theme lost after uninstall")
	}
	if _, ok := uninstalledMap["hooks"]; ok {
		t.Errorf("expected hooks to be cleaned up")
	}
}

func TestHookInstall_Antigravity(t *testing.T) {
	tempHome := t.TempDir()
	geminiDir := filepath.Join(tempHome, ".gemini", "config")
	if err := os.MkdirAll(geminiDir, 0750); err != nil {
		t.Fatalf("failed to create gemini dir: %v", err)
	}

	hooksPath := filepath.Join(geminiDir, "hooks.json")
	res, err := installAntigravityHooks(hooksPath, false)
	if err != nil {
		t.Fatalf("installAntigravityHooks failed: %v", err)
	}
	if !strings.Contains(res, "installed") {
		t.Errorf("expected install message, got %q", res)
	}

	data, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatalf("failed to read hooks.json: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("invalid hooks.json: %v", err)
	}
	daneelHook, ok := m["daneel"].(map[string]interface{})
	if !ok {
		t.Fatalf("daneel hook namespace missing from hooks.json: %v", m)
	}

	// Verify PreInvocation (flat)
	preInvocations, ok := daneelHook["PreInvocation"].([]interface{})
	if !ok || len(preInvocations) == 0 {
		t.Fatalf("expected PreInvocation array in daneel hook")
	}
	firstPre := preInvocations[0].(map[string]interface{})
	if firstPre["command"] != "daneel hook handoff" {
		t.Errorf("expected handoff command, got %v", firstPre["command"])
	}

	// Verify PostToolUse (grouped)
	postTools, ok := daneelHook["PostToolUse"].([]interface{})
	if !ok || len(postTools) == 0 {
		t.Fatalf("expected PostToolUse array in daneel hook")
	}
	firstPostGroup := postTools[0].(map[string]interface{})
	if firstPostGroup["matcher"] != "*" {
		t.Errorf("expected matcher '*', got %v", firstPostGroup["matcher"])
	}
	postInnerHooks := firstPostGroup["hooks"].([]interface{})
	firstPostHook := postInnerHooks[0].(map[string]interface{})
	if !strings.Contains(firstPostHook["command"].(string), "capture PostToolUse --agent antigravity") {
		t.Errorf("unexpected command in PostToolUse: %v", firstPostHook["command"])
	}

	// Verify Stop (flat)
	stops, ok := daneelHook["Stop"].([]interface{})
	if !ok || len(stops) == 0 {
		t.Fatalf("expected Stop array in daneel hook")
	}
	firstStop := stops[0].(map[string]interface{})
	if !strings.Contains(firstStop["command"].(string), "capture SessionEnd --agent antigravity") {
		t.Errorf("unexpected command in Stop: %v", firstStop["command"])
	}

	// Test uninstall
	_, err = uninstallAntigravityHooks(hooksPath, false)
	if err != nil {
		t.Fatalf("uninstallAntigravityHooks failed: %v", err)
	}
	if _, err := os.Stat(hooksPath); !os.IsNotExist(err) {
		uninstalledData, _ := os.ReadFile(hooksPath)
		var uninstalledMap map[string]interface{}
		_ = json.Unmarshal(uninstalledData, &uninstalledMap)
		if _, ok := uninstalledMap["daneel"]; ok {
			t.Errorf("daneel hook still present after uninstall")
		}
	}
}

func TestHookHandoff_ContextAware(t *testing.T) {
	tempCacheDir := t.TempDir()
	tempVaultDir := t.TempDir()
	_ = os.Setenv("XDG_CACHE_HOME", tempCacheDir)

	oldVaultPath := vaultPath
	vaultPath = tempVaultDir
	defer func() { vaultPath = oldVaultPath }()

	sp := spooler.New(filepath.Join(tempCacheDir, "daneel", "spool"))
	targetCWD := "/home/user/work/myrepo"

	// 1. With NO sessions, returns clean empty string
	outEmpty, err := executeHookHandoff(targetCWD)
	if err != nil {
		t.Fatalf("executeHookHandoff failed: %v", err)
	}
	if outEmpty != "" {
		t.Errorf("expected empty string when no recent session, got %q", outEmpty)
	}

	// 2. Add an event and compile it to vault
	_, err = sp.AppendEvent(spooler.Event{
		SessionID: "sess-handoff-test-01",
		Event:     "SessionEnd",
		Agent:     "claude-code",
		CWD:       targetCWD,
		Raw:       []byte(`{"command": "vitest run", "file": "src/app.ts"}`),
		Timestamp: time.Now(),
	})
	if err != nil {
		t.Fatalf("failed to append event: %v", err)
	}

	c := spooler.NewCompiler(tempVaultDir, sp)
	_, err = c.CompileSession("sess-handoff-test-01")
	if err != nil {
		t.Fatalf("failed to compile session: %v", err)
	}

	// 3. Now handoff should return continuity context
	outContext, err := executeHookHandoff(targetCWD)
	if err != nil {
		t.Fatalf("executeHookHandoff with context failed: %v", err)
	}
	if !strings.Contains(outContext, "claude-code") {
		t.Errorf("expected handoff context to mention 'claude-code', got %q", outContext)
	}
	if !strings.Contains(outContext, "vitest run") {
		t.Errorf("expected handoff context to mention 'vitest run', got %q", outContext)
	}
}

func TestHookInstall_Codex(t *testing.T) {
	tempHome := t.TempDir()
	codexDir := filepath.Join(tempHome, ".codex")
	if err := os.MkdirAll(codexDir, 0750); err != nil {
		t.Fatalf("failed to create codex dir: %v", err)
	}

	hooksPath := filepath.Join(codexDir, "hooks.json")
	res, err := installCodexHooks(hooksPath, false)
	if err != nil {
		t.Fatalf("installCodexHooks failed: %v", err)
	}
	if !strings.Contains(res, "installed") {
		t.Errorf("expected install message, got %q", res)
	}

	data, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatalf("failed to read installed hooks: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("corrupted JSON: %v", err)
	}

	hooks, ok := m["hooks"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected 'hooks' map in codex config")
	}

	// Verify standard matcher group format
	for _, key := range []string{"SessionStart", "PostToolUse", "SessionEnd"} {
		groups, ok := hooks[key].([]interface{})
		if !ok || len(groups) == 0 {
			t.Fatalf("expected matcher groups array in %s", key)
		}
		g := groups[0].(map[string]interface{})
		innerList := g["hooks"].([]interface{})
		if len(innerList) == 0 {
			t.Fatalf("expected inner hooks list in %s", key)
		}
		h := innerList[0].(map[string]interface{})
		cmdStr, _ := h["command"].(string)
		if !strings.Contains(cmdStr, "daneel hook") {
			t.Errorf("expected daneel command in %s, got %v", key, cmdStr)
		}
	}

	// Uninstall
	unres, err := uninstallCodexHooks(hooksPath, false)
	if err != nil {
		t.Fatalf("uninstallCodexHooks failed: %v", err)
	}
	if !strings.Contains(unres, "uninstalled") {
		t.Errorf("expected uninstall message, got %q", unres)
	}

	uninstalledData, _ := os.ReadFile(hooksPath)
	var uninstalledMap map[string]interface{}
	_ = json.Unmarshal(uninstalledData, &uninstalledMap)
	if _, ok := uninstalledMap["hooks"]; ok {
		t.Errorf("expected hooks to be removed from codex config")
	}
}

func TestHookInstall_CodexSessionEndTimeoutMigration(t *testing.T) {
	const sessionEndCommand = "daneel hook capture SessionEnd --agent codex"
	thirdParty := map[string]interface{}{
		"type": "command", "command": "echo third-party", "timeout": float64(17), "custom": "preserve",
	}

	tests := []struct {
		name  string
		input map[string]interface{}
	}{
		{name: "new file", input: map[string]interface{}{}},
		{
			name: "installed matcher group without timeout",
			input: map[string]interface{}{"hooks": map[string]interface{}{
				"SessionEnd": []interface{}{map[string]interface{}{"matcher": "", "hooks": []interface{}{
					map[string]interface{}{"type": "command", "command": sessionEndCommand, "custom": "preserve"},
				}}},
			}},
		},
		{
			name: "installed matcher group with old timeout",
			input: map[string]interface{}{"hooks": map[string]interface{}{
				"SessionEnd": []interface{}{map[string]interface{}{"matcher": "", "hooks": []interface{}{
					thirdParty,
					map[string]interface{}{"type": "command", "command": sessionEndCommand, "timeout": float64(1), "custom": "preserve"},
				}}},
			}},
		},
		{
			name: "legacy Daneel event string",
			input: map[string]interface{}{"hooks": map[string]interface{}{
				"SessionEnd": sessionEndCommand,
			}},
		},
		{
			name: "legacy flat matcher command",
			input: map[string]interface{}{"hooks": map[string]interface{}{
				"SessionEnd": []interface{}{map[string]interface{}{"matcher": "", "command": sessionEndCommand}},
			}},
		},
		{
			name: "mixed third-party handlers",
			input: map[string]interface{}{"hooks": map[string]interface{}{
				"SessionEnd": []interface{}{map[string]interface{}{"matcher": "", "hooks": []interface{}{
					thirdParty,
					map[string]interface{}{"type": "command", "command": sessionEndCommand, "env": map[string]interface{}{"MODE": "test"}},
				}}},
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hooksPath := filepath.Join(t.TempDir(), "hooks.json")
			if len(tt.input) > 0 {
				initial, err := json.MarshalIndent(tt.input, "", "  ")
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(hooksPath, append(initial, '\n'), 0600); err != nil {
					t.Fatal(err)
				}
			}

			for range 2 {
				if _, err := installCodexHooks(hooksPath, false); err != nil {
					t.Fatalf("installCodexHooks: %v", err)
				}
			}
			installed, err := os.ReadFile(hooksPath)
			if err != nil {
				t.Fatal(err)
			}
			var got map[string]interface{}
			if err := json.Unmarshal(installed, &got); err != nil {
				t.Fatalf("decode installed config: %v", err)
			}
			configHooks := got["hooks"].(map[string]interface{})
			groups := configHooks["SessionEnd"].([]interface{})
			var owned []map[string]interface{}
			for _, group := range groups {
				for _, handler := range group.(map[string]interface{})["hooks"].([]interface{}) {
					h, ok := handler.(map[string]interface{})
					if ok && h["command"] == sessionEndCommand {
						owned = append(owned, h)
					}
				}
			}
			if len(owned) != 1 {
				t.Fatalf("expected exactly one Daneel SessionEnd handler, got %d", len(owned))
			}
			if owned[0]["timeout"] != float64(3) {
				t.Errorf("Daneel SessionEnd timeout = %v, want 3", owned[0]["timeout"])
			}
			if tt.name == "installed matcher group without timeout" && owned[0]["custom"] != "preserve" {
				t.Errorf("owned handler field was not preserved: %#v", owned[0])
			}
			if tt.name == "installed matcher group with old timeout" && owned[0]["custom"] != "preserve" {
				t.Errorf("owned handler field was not preserved while correcting timeout: %#v", owned[0])
			}
			if tt.name == "mixed third-party handlers" || tt.name == "installed matcher group with old timeout" {
				handlers := groups[0].(map[string]interface{})["hooks"].([]interface{})
				if len(handlers) != 2 {
					t.Fatalf("third-party handler count changed: %#v", groups[0])
				}
				if !reflect.DeepEqual(handlers[0], thirdParty) {
					t.Errorf("third-party handler changed: %#v", handlers[0])
				}
				if tt.name == "mixed third-party handlers" && !reflect.DeepEqual(owned[0]["env"], map[string]interface{}{"MODE": "test"}) {
					t.Errorf("owned handler fields changed: %#v", owned[0])
				}
			}
		})
	}

	t.Run("dry run leaves bytes unchanged", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "hooks.json")
		original := []byte("{\"hooks\":{\"SessionEnd\":[{\"hooks\":[{\"type\":\"command\",\"command\":\"" + sessionEndCommand + "\"}]}]}}\n")
		if err := os.WriteFile(path, original, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := installCodexHooks(path, true); err != nil {
			t.Fatalf("dry-run installCodexHooks: %v", err)
		}
		after, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(after, original) {
			t.Errorf("dry-run changed bytes: before %q, after %q", original, after)
		}
	})
}

func TestHookInstall_RegistryExtensibility(t *testing.T) {
	adapters := getRegisteredAdapters()
	names := make(map[string]bool)
	for _, a := range adapters {
		names[a.Name()] = true
	}

	if !names["claude-code"] {
		t.Errorf("expected claude-code adapter registered")
	}
	if !names["antigravity"] {
		t.Errorf("expected antigravity adapter registered")
	}
	if !names["codex"] {
		t.Errorf("expected codex adapter registered")
	}
}

func TestHookInstall_IdempotencyAndPreservation(t *testing.T) {
	tempHome := t.TempDir()
	claudeDir := filepath.Join(tempHome, ".claude")
	_ = os.MkdirAll(claudeDir, 0750)
	settingsPath := filepath.Join(claudeDir, "settings.json")

	// 1. Initial settings with a pre-existing custom user hook
	initial := `{
  "theme": "dark",
  "hooks": {
    "PostToolUse": [
      {
        "matcher": ".*",
        "hooks": [
          {"type": "command", "command": "echo custom-linter"}
        ]
      }
    ]
  }
}`
	_ = os.WriteFile(settingsPath, []byte(initial), 0600)

	// 2. First install
	_, err := installClaudeHooks(settingsPath, false)
	if err != nil {
		t.Fatalf("first install failed: %v", err)
	}

	// 3. Second install (test strict idempotency: f(f(x)) == f(x))
	_, err = installClaudeHooks(settingsPath, false)
	if err != nil {
		t.Fatalf("second install failed: %v", err)
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("failed to read settings: %v", err)
	}

	var m map[string]interface{}
	_ = json.Unmarshal(data, &m)
	hooks := m["hooks"].(map[string]interface{})
	postTools := hooks["PostToolUse"].([]interface{})

	// Should have EXACTLY 2 matcher groups: custom-linter and daneel hook (no duplicates from run 2)
	if len(postTools) != 2 {
		t.Fatalf("expected 2 PostToolUse groups (preserved custom + daneel), got %d: %v", len(postTools), postTools)
	}

	hasCustom := false
	hasDaneel := false
	for _, item := range postTools {
		obj := item.(map[string]interface{})
		innerHooks := toInterfaceSlice(obj["hooks"])
		for _, h := range innerHooks {
			hm := h.(map[string]interface{})
			cmdStr, _ := hm["command"].(string)
			if cmdStr == "echo custom-linter" {
				hasCustom = true
			}
			if strings.Contains(cmdStr, "daneel hook capture") {
				hasDaneel = true
			}
		}
	}

	if !hasCustom {
		t.Errorf("custom user hook was lost during install!")
	}
	if !hasDaneel {
		t.Errorf("daneel hook was not added!")
	}

	// 4. Uninstall should ONLY remove daneel hooks, preserving custom user hook
	_, err = uninstallClaudeHooks(settingsPath, false)
	if err != nil {
		t.Fatalf("uninstall failed: %v", err)
	}

	uninstalledData, _ := os.ReadFile(settingsPath)
	var uninstalledMap map[string]interface{}
	_ = json.Unmarshal(uninstalledData, &uninstalledMap)

	uninstalledHooks, ok := uninstalledMap["hooks"].(map[string]interface{})
	if !ok {
		t.Fatalf("hooks object was completely deleted, losing user's custom hook!")
	}

	remainingPostTools, ok := uninstalledHooks["PostToolUse"].([]interface{})
	if !ok || len(remainingPostTools) != 1 {
		t.Fatalf("expected 1 remaining custom hook in PostToolUse, got %v", remainingPostTools)
	}

	remObj := remainingPostTools[0].(map[string]interface{})
	remInner := toInterfaceSlice(remObj["hooks"])
	if len(remInner) != 1 {
		t.Fatalf("expected 1 remaining inner hook, got %d", len(remInner))
	}
	firstHook := remInner[0].(map[string]interface{})
	if firstHook["command"] != "echo custom-linter" {
		t.Errorf("expected remaining hook to be 'echo custom-linter', got %v", firstHook["command"])
	}
}
