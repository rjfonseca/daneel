//nolint:revive // package comment not required
package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/rjfonseca/daneel/internal/config"
)

func TestInitCmd_NonInteractive(t *testing.T) {
	tempDir := t.TempDir()

	// Execute initCmd directly in non-interactive mode
	initYes = true
	initGlobal = false
	initLocal = true
	defer func() {
		initYes = false
		initGlobal = false
		initLocal = false
	}()

	err := initCmd.RunE(initCmd, []string{tempDir})
	if err != nil {
		t.Fatalf("initCmd failed: %v", err)
	}

	// 1. Verify .daneel.yaml exists
	yamlPath := filepath.Join(tempDir, ".daneel.yaml")
	if _, err := os.Stat(yamlPath); os.IsNotExist(err) {
		t.Fatalf("expected .daneel.yaml to be created at %s", yamlPath)
	}

	// 2. Verify YAML can be unmarshaled into semantic Config
	data, err := os.ReadFile(yamlPath) //nolint:gosec // intentional
	if err != nil {
		t.Fatalf("failed to read .daneel.yaml: %v", err)
	}

	var parsedCfg config.Config
	if err := yaml.Unmarshal(data, &parsedCfg); err != nil {
		t.Fatalf("failed to unmarshal semantic .daneel.yaml: %v", err)
	}

	if len(parsedCfg.Vault.Categories) == 0 {
		t.Errorf("expected initialized categories in vault.categories")
	}
	if len(parsedCfg.Publishing.Block.Domains) == 0 {
		t.Errorf("expected blocked domains in publishing.block.domains")
	}
	if len(parsedCfg.Telemetry.Taxonomy) == 0 {
		t.Errorf("expected taxonomy in telemetry.taxonomy")
	}
	if len(parsedCfg.Radar) == 0 {
		t.Errorf("expected radar items in radar")
	}
	if !parsedCfg.Embeddings.Enabled {
		t.Errorf("expected embeddings.enabled to default to true in .daneel.yaml")
	}
	if parsedCfg.Embeddings.Provider != "rembed" {
		t.Errorf("expected embeddings.provider=rembed, got %q", parsedCfg.Embeddings.Provider)
	}

	// 3. Verify Master MOC created
	masterMoc := filepath.Join(tempDir, "notes", "moc-indice-geral.md")
	if _, err := os.Stat(masterMoc); os.IsNotExist(err) {
		t.Fatalf("expected notes/moc-indice-geral.md to exist")
	}

	mocData, _ := os.ReadFile(masterMoc) //nolint:gosec // intentional
	if !strings.Contains(string(mocData), "Thematic MOC") {
		t.Errorf("expected MOC table in moc-indice-geral.md")
	}

	// 4. Verify category MOC created
	iaMoc := filepath.Join(tempDir, "notes", "ia", "moc-ia.md")
	if _, err := os.Stat(iaMoc); os.IsNotExist(err) {
		t.Fatalf("expected notes/ia/moc-ia.md to exist")
	}
}

func TestInitCmd_NoEmbeddingsFlag(t *testing.T) {
	tempDir := t.TempDir()

	initYes = true
	initGlobal = false
	initLocal = true
	initNoEmbeddings = true
	defer func() {
		initYes = false
		initGlobal = false
		initLocal = false
		initNoEmbeddings = false
	}()

	if err := initCmd.RunE(initCmd, []string{tempDir}); err != nil {
		t.Fatalf("initCmd --no-embeddings failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tempDir, ".daneel.yaml")) //nolint:gosec // intentional
	if err != nil {
		t.Fatalf("failed to read .daneel.yaml: %v", err)
	}
	if !strings.Contains(string(data), "enabled: false") {
		t.Errorf("expected embeddings.enabled: false in .daneel.yaml, got:\n%s", string(data))
	}

	var parsedCfg config.Config
	if err := yaml.Unmarshal(data, &parsedCfg); err != nil {
		t.Fatalf("failed to unmarshal .daneel.yaml: %v", err)
	}
	if parsedCfg.Embeddings.Enabled {
		t.Errorf("expected parsed embeddings.enabled to be false")
	}

	// Verify starter skills & AGENTS.md include cumulative morning handoff and dream-cycle workflows
	for _, rel := range []string{
		"AGENTS.md",
		filepath.Join(".agents", "skills", "morning-briefing", "SKILL.md"),
		filepath.Join(".agents", "skills", "dream-cycle", "SKILL.md"),
	} {
		p := filepath.Join(tempDir, rel)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			t.Errorf("expected %s to be scaffolded by daneel init", rel)
		}
	}
	morningSkill, _ := os.ReadFile(filepath.Join(tempDir, ".agents", "skills", "morning-briefing", "SKILL.md"))
	if !strings.Contains(string(morningSkill), "daneel handoff summary") {
		t.Errorf("expected morning-briefing skill to reference 'daneel handoff summary'")
	}
	dreamSkill, _ := os.ReadFile(filepath.Join(tempDir, ".agents", "skills", "dream-cycle", "SKILL.md"))
	if !strings.Contains(string(dreamSkill), "daneel handoff record") {
		t.Errorf("expected dream-cycle skill to reference 'daneel handoff record'")
	}
}
