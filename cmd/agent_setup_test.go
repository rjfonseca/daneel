package cmd

import (
	"errors"
	"testing"
)

func TestDetectAgentCandidatesWithLookup(t *testing.T) {
	installed := map[string]bool{
		"claude": true,
		"codex":  true,
	}
	found := detectAgentCandidatesWithLookup(func(name string) (string, error) {
		if installed[name] {
			return "/usr/bin/" + name, nil
		}
		return "", errors.New("not found")
	})

	if len(found) != 2 {
		t.Fatalf("expected 2 detected agents, got %d", len(found))
	}
	if found[0].Name != "claude" || found[1].Name != "codex" {
		t.Errorf("unexpected detected agents: %+v", found)
	}
}

func TestBuildNonInteractiveChatConfig(t *testing.T) {
	t.Run("zero agents returns empty config", func(t *testing.T) {
		cfg := buildNonInteractiveChatConfig(nil)
		if cfg.Default != "" || len(cfg.Profiles) != 0 {
			t.Errorf("expected empty config, got %+v", cfg)
		}
	})

	t.Run("single agent sets it as default", func(t *testing.T) {
		cfg := buildNonInteractiveChatConfig([]knownAgentCandidate{
			{Name: "claude", Command: "claude"},
		})
		if cfg.Default != "claude" {
			t.Errorf("expected default 'claude', got %q", cfg.Default)
		}
		if len(cfg.Profiles) != 1 || cfg.Profiles["claude"].Command != "claude" {
			t.Errorf("unexpected profiles: %+v", cfg.Profiles)
		}
	})

	t.Run("multiple agents populate profiles but leave default empty", func(t *testing.T) {
		cfg := buildNonInteractiveChatConfig([]knownAgentCandidate{
			{Name: "agy", Command: "agy"},
			{Name: "claude", Command: "claude"},
		})
		if cfg.Default != "" {
			t.Errorf("expected empty default when multiple agents are present, got %q", cfg.Default)
		}
		if len(cfg.Profiles) != 2 {
			t.Errorf("expected 2 profiles populated, got %+v", cfg.Profiles)
		}
	})
}
