package harvester

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rjfonseca/daneel/internal/config"
)

func TestPruneSources(t *testing.T) {
	vaultRoot := t.TempDir()

	// Setup directory structure
	// 1. Active project: acme/saas/auth
	authDir := filepath.Join(vaultRoot, "sources", "gitlab", "acme", "saas", "auth")
	_ = os.MkdirAll(filepath.Join(authDir, "docs"), 0750)
	_ = os.WriteFile(filepath.Join(authDir, "index.md"), []byte("---\ntype: repository\nrepository: auth\n---\n# Auth\n"), 0600)
	_ = os.WriteFile(filepath.Join(authDir, "docs", "arch.md"), []byte("# Architecture\n"), 0600)

	// 2. Toxic dir inside active project: auth/node_modules
	toxicDir := filepath.Join(authDir, "node_modules", "chalk")
	_ = os.MkdirAll(toxicDir, 0750)
	_ = os.WriteFile(filepath.Join(toxicDir, "readme.md"), []byte("# Chalk\n"), 0600)

	// 3. Excluded group project: acme/marketing/campaign
	mktDir := filepath.Join(vaultRoot, "sources", "gitlab", "acme", "marketing", "campaign")
	_ = os.MkdirAll(mktDir, 0750)
	_ = os.WriteFile(filepath.Join(mktDir, "index.md"), []byte("---\ntype: repository\nrepository: campaign\n---\n# Campaign\n"), 0600)
	_ = os.WriteFile(filepath.Join(mktDir, "readme.md"), []byte("# Campaign\n"), 0600)

	// 4. Excluded project pattern (*poc*): acme/saas/auth-poc
	pocDir := filepath.Join(vaultRoot, "sources", "gitlab", "acme", "saas", "auth-poc")
	_ = os.MkdirAll(pocDir, 0750)
	_ = os.WriteFile(filepath.Join(pocDir, "index.md"), []byte("---\ntype: repository\nrepository: auth-poc\n---\n# Auth POC\n"), 0600)
	_ = os.WriteFile(filepath.Join(pocDir, "readme.md"), []byte("# Auth POC\n"), 0600)

	// 5. Remote orphan (not in active set): acme/saas/legacy-service
	legacyDir := filepath.Join(vaultRoot, "sources", "gitlab", "acme", "saas", "legacy-service")
	_ = os.MkdirAll(legacyDir, 0750)
	_ = os.WriteFile(filepath.Join(legacyDir, "index.md"), []byte("---\ntype: repository\nrepository: legacy-service\n---\n# Legacy\n"), 0600)
	_ = os.WriteFile(filepath.Join(legacyDir, "readme.md"), []byte("# Legacy\n"), 0600)

	cfg := config.HarvesterConfig{
		ExcludeGroups:   []string{"acme/marketing"},
		ExcludeProjects: []string{"*poc*"},
	}

	activeMap := map[string][]string{
		"gitlab": {"acme/saas/auth"},
	}

	opts := PruneOptions{
		VaultRoot:        vaultRoot,
		Config:           cfg,
		ActiveByPlatform: activeMap,
		DryRun:           false,
	}

	stats, err := PruneSources(opts)
	if err != nil {
		t.Fatalf("PruneSources failed: %v", err)
	}

	if stats.ToxicDirsCount != 1 {
		t.Errorf("expected 1 toxic dir pruned, got %d", stats.ToxicDirsCount)
	}
	if stats.ProjectsPrunedCount != 3 {
		t.Errorf("expected 3 projects pruned (marketing, auth-poc, legacy-service), got %d", stats.ProjectsPrunedCount)
	}

	// Verify filesystem state
	// auth/docs/arch.md must exist
	if _, err := os.Stat(filepath.Join(authDir, "docs", "arch.md")); err != nil {
		t.Errorf("auth/docs/arch.md should exist, but stat returned: %v", err)
	}
	// auth/node_modules must NOT exist
	if _, err := os.Stat(filepath.Join(authDir, "node_modules")); !os.IsNotExist(err) {
		t.Errorf("auth/node_modules should have been pruned")
	}
	// marketing/campaign must NOT exist
	if _, err := os.Stat(mktDir); !os.IsNotExist(err) {
		t.Errorf("marketing/campaign should have been pruned")
	}
	// auth-poc must NOT exist
	if _, err := os.Stat(pocDir); !os.IsNotExist(err) {
		t.Errorf("auth-poc should have been pruned")
	}
	// legacy-service must NOT exist
	if _, err := os.Stat(legacyDir); !os.IsNotExist(err) {
		t.Errorf("legacy-service should have been pruned")
	}
}

func TestPruneQuarantinesAndDryRunDoesNotWrite(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "sources", "github", "acme", "old-poc")
	if err := os.MkdirAll(repo, 0700); err != nil {
		t.Fatal(err)
	}
	content := "---\ntype: repository\nrepository: old-poc\n---\n# Edited locally\n"
	if err := os.WriteFile(filepath.Join(repo, "index.md"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	opts := PruneOptions{VaultRoot: root, Config: config.HarvesterConfig{ExcludeProjects: []string{"*poc"}}, DryRun: true}
	preview, err := PruneSources(opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Items) != 1 {
		t.Fatalf("preview: %+v", preview)
	}
	if _, err := os.Stat(filepath.Join(root, ".daneel")); !os.IsNotExist(err) {
		t.Fatal("dry-run created state")
	}
	opts.DryRun = false
	result, err := PruneSources(opts)
	if err != nil {
		t.Fatal(err)
	}
	copies, _ := filepath.Glob(filepath.Join(root, ".daneel", "quarantine", "*", "sources", "github", "acme", "old-poc", "index.md"))
	if len(copies) != 1 {
		t.Fatalf("missing recovery copy: %+v", result)
	}
	saved, err := os.ReadFile(copies[0])
	if err != nil || string(saved) != content {
		t.Fatalf("local edits lost: %s %v", saved, err)
	}
	if _, err := os.Stat(repo); !os.IsNotExist(err) {
		t.Fatal("pruned source still present")
	}
	// Restoration consists of moving the directory back to its original path.
	if err := os.MkdirAll(filepath.Dir(repo), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Dir(copies[0]), repo); err != nil {
		t.Fatal(err)
	}
	saved, err = os.ReadFile(filepath.Join(repo, "index.md"))
	if err != nil || string(saved) != content {
		t.Fatal("restoration failed")
	}
}

func TestPruneDoesNotFollowExternalSourcesSymlink(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	target := filepath.Join(outside, "github", "acme", "old-poc")
	if err := os.MkdirAll(target, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "index.md"), []byte("---\ntype: repository\n---\n# External\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "sources")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	_, err := PruneSources(PruneOptions{VaultRoot: root, Config: config.HarvesterConfig{ExcludeProjects: []string{"*poc"}}})
	if err == nil {
		t.Fatal("expected refusal to prune symlinked sources")
	}
	if _, err := os.Stat(filepath.Join(target, "index.md")); err != nil {
		t.Fatalf("external content modified: %v", err)
	}
}

func TestPruneLeavesRetainedRepositoryContentUntouched(t *testing.T) {
	root := t.TempDir()
	for _, repo := range []string{"old-poc", "retained"} {
		dir := filepath.Join(root, "sources", "github", "acme", repo)
		if err := os.MkdirAll(filepath.Join(dir, "docs"), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "index.md"), []byte("---\ntype: repository\nrepository: "+repo+"\n---\n# Repository\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	note := filepath.Join(root, "sources", "github", "acme", "retained", "docs", "index.md")
	original := []byte("---\ntype: repository\n---\n- [Custom runbook](runbook.md)\n")
	if err := os.WriteFile(note, original, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := PruneSources(PruneOptions{VaultRoot: root, Config: config.HarvesterConfig{ExcludeProjects: []string{"*poc"}}}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(note)
	if err != nil || string(data) != string(original) {
		t.Fatalf("prune changed retained content: %s %v", data, err)
	}
}
