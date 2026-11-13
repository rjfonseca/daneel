package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootCmd_AllCommandsHaveValidLLMAnnotation(t *testing.T) {
	validModes := map[string]bool{
		llmModeNone:     true,
		llmModeOptional: true,
		llmModeUses:     true,
	}

	for _, c := range RootCmd.Commands() {
		if c.Name() == "help" {
			continue
		}
		mode, ok := c.Annotations[llmAnnotationKey]
		if !ok || !validModes[mode] {
			t.Errorf("command %q is missing a valid %q annotation (got %q); register it in cmd/llm_annotations.go", c.Name(), llmAnnotationKey, mode)
		}
	}
}

func TestHelpOutputIncludesLLMBadgesAndDetails(t *testing.T) {
	var rootHelp bytes.Buffer
	RootCmd.SetOut(&rootHelp)
	RootCmd.SetErr(&rootHelp)
	defer func() {
		RootCmd.SetOut(nil)
		RootCmd.SetErr(nil)
	}()

	if err := RootCmd.Usage(); err != nil {
		t.Fatalf("RootCmd.Usage() failed: %v", err)
	}
	rootOut := rootHelp.String()
	for _, want := range []string{"[No LLM]", "[Optional LLM]", "[Uses LLM]"} {
		if !strings.Contains(rootOut, want) {
			t.Errorf("root usage output missing badge %q", want)
		}
	}

	cases := []struct {
		cmdName string
		want    string
	}{
		{"morning", "No LLM (100% deterministic local Go execution)"},
		{"dream", "Optional LLM —"},
		{"chat", "Uses LLM —"},
	}

	for _, tc := range cases {
		sub, _, err := RootCmd.Find([]string{tc.cmdName})
		if err != nil {
			t.Fatalf("failed to find command %q: %v", tc.cmdName, err)
		}
		var subHelp bytes.Buffer
		sub.SetOut(&subHelp)
		sub.SetErr(&subHelp)
		if err := sub.Usage(); err != nil {
			t.Fatalf("%s.Usage() failed: %v", tc.cmdName, err)
		}
		sub.SetOut(nil)
		sub.SetErr(nil)
		if !strings.Contains(subHelp.String(), tc.want) {
			t.Errorf("%s usage output missing %q\ngot:\n%s", tc.cmdName, tc.want, subHelp.String())
		}
	}
}
