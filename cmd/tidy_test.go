//nolint:revive // package comment not required
package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rjfonseca/daneel/internal/config"
)

func TestTidyCmd_Basic(t *testing.T) {
	t.Setenv("DANEEL_VAULT_PATH", "")
	t.Setenv("DANEEL_CONFIG", "")
	tempVault := t.TempDir()

	// Setup basic vault structure
	_ = os.MkdirAll(filepath.Join(tempVault, "notes", "ia"), 0750)
	_ = os.MkdirAll(filepath.Join(tempVault, "notes", "arquitetura"), 0750)
	_ = os.MkdirAll(filepath.Join(tempVault, "daily"), 0750)

	targetNote := filepath.Join(tempVault, "notes", "ia", "rag.md")
	_ = os.WriteFile(targetNote, []byte("---\ntype: tecnologia\ndescription: RAG\n---\n# RAG\n"), 0600)

	sourceNote := filepath.Join(tempVault, "notes", "arquitetura", "microservices.md")
	_ = os.WriteFile(sourceNote, []byte("---\ntype: arquitetura\ndescription: Microservices\n---\n# Microservices\nLink: [[rag|RAG Concept]]\n"), 0600)

	cfg = &config.Config{
		VaultPath: tempVault,
		DBPath:    filepath.Join(tempVault, ".daneel", "cache", "vault_index.db"),
		Vault:     config.VaultConfig{IndexedDirs: []string{"notes", "daily"}},
	}

	tidyDate = "2026-08-19"
	tidyNoFix = false
	tidyLearn = false

	err := tidyCmd.RunE(tidyCmd, []string{})
	if err != nil {
		t.Fatalf("tidyCmd failed: %v", err)
	}

	// Verify WikiLink was converted to relative markdown
	data, err := os.ReadFile(sourceNote) //nolint:gosec // intentional
	if err != nil {
		t.Fatalf("failed to read source note: %v", err)
	}

	expectedRel := "[RAG Concept](../ia/rag.md)"
	if !os.IsNotExist(err) && !stringContains(string(data), expectedRel) {
		t.Errorf("expected source note to contain %s, got:\n%s", expectedRel, string(data))
	}
}

func stringContains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || (len(s) > 0 && len(substr) > 0 && filepath.ToSlash(s) != "" && containsHelper(s, substr)))
}

func containsHelper(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
