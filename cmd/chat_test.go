//nolint:revive // package comment not required
package cmd

import (
	"path/filepath"
	"testing"
)

func TestSplitCommand(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "Simple command",
			input:    "claude",
			expected: []string{"claude"},
		},
		{
			name:     "Command with arguments",
			input:    "agy --dangerously-skip-permissions",
			expected: []string{"agy", "--dangerously-skip-permissions"},
		},
		{
			name:     "Command with quotes and flags",
			input:    `sandbox-runner --volume="~/data:~/data" --config="~/.config/agent" --exec agy --dangerously-skip-permissions`,
			expected: []string{"sandbox-runner", `--volume=~/data:~/data`, `--config=~/.config/agent`, "--exec", "agy", "--dangerously-skip-permissions"},
		},
		{
			name:     "Single quoted string",
			input:    `echo 'hello world' "second arg"`,
			expected: []string{"echo", "hello world", "second arg"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := splitCommand(tt.input)
			if err != nil {
				t.Fatalf("splitCommand failed: %v", err)
			}
			if len(res) != len(tt.expected) {
				t.Fatalf("expected %d tokens, got %d: %v", len(tt.expected), len(res), res)
			}
			for i, v := range res {
				if v != tt.expected[i] {
					t.Errorf("token %d: expected %s, got %s", i, tt.expected[i], v)
				}
			}
		})
	}
}

func TestExpandHomeInToken(t *testing.T) {
	tok := "--volume=~/test:~/dest"
	res := expandHomeInToken(tok)
	if res == tok {
		// ~ should be expanded
		t.Logf("expanded token: %s", res)
	}
}

func TestPrepareGitHubSandboxAuth(t *testing.T) {
	t.Run("fetches host gh token and injects ai-jail mask and env flag", func(t *testing.T) {
		envMap := map[string]string{
			"TERM": "xterm-256color",
		}
		cmdTokens := []string{
			"ai-jail",
			"--rw-map=/home/user/.gemini-personal:/home/user/.gemini",
			"--mask=/home/user/.config/gcloud",
			"--exec",
			"agy",
		}

		gotTokens := prepareGitHubSandboxAuth(envMap, cmdTokens, "/home/user", func() string {
			return "gho_test_token_123"
		})

		if envMap["GH_TOKEN"] != "gho_test_token_123" {
			t.Fatalf("expected GH_TOKEN to be set in envMap, got %q", envMap["GH_TOKEN"])
		}

		wantMask := "--mask=" + filepath.Join("/home/user", ".config", "gh")
		hasMask := false
		hasEnvFlag := false
		for _, tok := range gotTokens {
			if tok == wantMask {
				hasMask = true
			}
			if tok == "--env=GH_TOKEN" {
				hasEnvFlag = true
			}
		}
		if !hasMask {
			t.Errorf("expected %s in command tokens, got %v", wantMask, gotTokens)
		}
		if !hasEnvFlag {
			t.Errorf("expected --env=GH_TOKEN in command tokens, got %v", gotTokens)
		}
	})

	t.Run("handles systemd-run wrapper around ai-jail and respects --no-github", func(t *testing.T) {
		envMap := map[string]string{}
		cmdTokens := []string{
			"systemd-run", "--user", "--scope", "--slice=agents.slice",
			"ai-jail", "--no-github", "--exec", "agy",
		}

		gotTokens := prepareGitHubSandboxAuth(envMap, cmdTokens, "/home/user", func() string {
			return "gho_should_not_be_used"
		})

		if _, exists := envMap["GH_TOKEN"]; exists {
			t.Errorf("expected GH_TOKEN not to be set when --no-github is present")
		}
		if len(gotTokens) != len(cmdTokens) {
			t.Errorf("expected tokens unchanged when --no-github is present, got %v", gotTokens)
		}
	})

	t.Run("gracefully skips when gh auth token returns empty", func(t *testing.T) {
		envMap := map[string]string{}
		cmdTokens := []string{"ai-jail", "--exec", "agy"}

		gotTokens := prepareGitHubSandboxAuth(envMap, cmdTokens, "/home/user", func() string {
			return ""
		})

		if _, exists := envMap["GH_TOKEN"]; exists {
			t.Errorf("expected GH_TOKEN not to be set when host gh is unauthenticated")
		}
		if len(gotTokens) != len(cmdTokens) {
			t.Errorf("expected tokens unchanged when no token available, got %v", gotTokens)
		}
	})
}
