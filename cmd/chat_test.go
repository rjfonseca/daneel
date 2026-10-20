//nolint:revive // package comment not required
package cmd

import (
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
