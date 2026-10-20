package harvester

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReconcileSourcesCatalog_SingleIndexPerRepo(t *testing.T) {
	tempVault := t.TempDir()

	// 1. Create a nested GitLab repository with subdirectories
	repoDir := filepath.Join(tempVault, "sources", "gitlab", "acme", "saas", "my-service")
	docsDir := filepath.Join(repoDir, "docs")
	plansDir := filepath.Join(repoDir, "plans")
	specsDir := filepath.Join(repoDir, "docs", "superpowers", "specs")

	_ = os.MkdirAll(docsDir, 0750)
	_ = os.MkdirAll(plansDir, 0750)
	_ = os.MkdirAll(specsDir, 0750)

	_ = os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("# My Service\nCore service."), 0600)
	_ = os.WriteFile(filepath.Join(docsDir, "architecture.md"), []byte("# Architecture"), 0600)
	_ = os.WriteFile(filepath.Join(plansDir, "roadmap.md"), []byte("# Roadmap"), 0600)
	_ = os.WriteFile(filepath.Join(specsDir, "api-spec.md"), []byte("# API Spec"), 0600)

	// Simulate pre-existing spurious index.md files in subdirectories:
	// spurious1 & spurious2: purely auto-generated Daneel boilerplate (zero human content)
	spurious1 := filepath.Join(plansDir, "index.md")
	spurious2 := filepath.Join(specsDir, "_index.md")
	_ = os.WriteFile(spurious1, []byte("---\ntype: repository\nrepository: plans\n---\n# 📦 plans\n## 📚 Cataloged Documentation\n- [roadmap](roadmap.md)\n"), 0600)
	_ = os.WriteFile(spurious2, []byte("---\ntype: repository\nrepository: specs\n---\n# 📦 specs\n- [api-spec](api-spec.md)\n"), 0600)

	// Legitimate human documentation committed in docs/index.md (e.g. MkDocs landing page)
	legitDocIndex := filepath.Join(docsDir, "index.md")
	_ = os.WriteFile(legitDocIndex, []byte("# Welcome to My Service Docs\n\nThis guide explains architecture, APIs, and deployments."), 0600)

	// 2. Run ReconcileSourcesCatalog
	stats, err := ReconcileSourcesCatalog(tempVault)
	if err != nil {
		t.Fatalf("ReconcileSourcesCatalog failed: %v", err)
	}

	if stats.TotalReposIndexed < 1 {
		t.Errorf("expected at least 1 repository indexed, got %d", stats.TotalReposIndexed)
	}

	// 3. Verify root index.md exists and contains correct metadata & doc links
	rootIndex := filepath.Join(repoDir, "index.md")
	contentBytes, err := os.ReadFile(rootIndex)
	if err != nil {
		t.Fatalf("expected repository root index.md to exist: %v", err)
	}
	content := string(contentBytes)

	if !strings.Contains(content, "type: repository") {
		t.Errorf("expected root index.md to have type: repository")
	}
	if !strings.Contains(content, "architecture.md") {
		t.Errorf("expected root index.md to catalog docs/architecture.md")
	}
	if !strings.Contains(content, "roadmap.md") {
		t.Errorf("expected root index.md to catalog plans/roadmap.md")
	}
	if !strings.Contains(content, "api-spec.md") {
		t.Errorf("expected root index.md to catalog specs/api-spec.md")
	}

	// Root index.md MUST also catalog the legitimate subfolder docs/index.md!
	if !strings.Contains(content, "docs/index.md") {
		t.Errorf("expected root index.md to catalog legitimate subfolder docs/index.md")
	}

	// 4. Verify purely spurious boilerplate files were purged
	if _, err := os.Stat(spurious1); !os.IsNotExist(err) {
		t.Errorf("expected spurious plans/index.md to be deleted")
	}
	if _, err := os.Stat(spurious2); !os.IsNotExist(err) {
		t.Errorf("expected spurious specs/_index.md to be deleted")
	}

	// 5. CRITICAL: Verify legitimate user documentation in docs/index.md was NOT deleted!
	legitBytes, err := os.ReadFile(legitDocIndex)
	if err != nil {
		t.Fatalf("CRITICAL REGRESSION: legitimate docs/index.md was deleted! %v", err)
	}
	if !strings.Contains(string(legitBytes), "Welcome to My Service Docs") {
		t.Errorf("expected legitimate user documentation to be fully preserved")
	}

	// 6. Verify ancestor groups were created with type: group
	groupIndex := filepath.Join(tempVault, "sources", "gitlab", "acme", "saas", "index.md")
	gBytes, err := os.ReadFile(groupIndex)
	if err != nil {
		t.Fatalf("expected group index.md to exist: %v", err)
	}
	if !strings.Contains(string(gBytes), "type: group") {
		t.Errorf("expected group index.md to have type: group")
	}
	if !strings.Contains(string(gBytes), "my-service") {
		t.Errorf("expected group index.md to list repository my-service")
	}
}

func TestReconcileSourcesCatalog_SanitizeSpuriousTypeFromUserDoc(t *testing.T) {
	tempVault := t.TempDir()

	repoDir := filepath.Join(tempVault, "sources", "gitlab", "acme", "saas", "user-doc-repo")
	docsDir := filepath.Join(repoDir, "docs")
	_ = os.MkdirAll(docsDir, 0750)

	_ = os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("# User Doc Repo"), 0600)

	// User doc that had type: repository erroneously injected by a past bug
	userDoc := filepath.Join(docsDir, "index.md")
	initialContent := "---\ntype: repository\nrepository: docs\ntitle: Architecture Documentation\n---\n# Architecture Documentation\n\nThis is human-written technical documentation about microservices."
	_ = os.WriteFile(userDoc, []byte(initialContent), 0600)

	_, err := ReconcileSourcesCatalog(tempVault)
	if err != nil {
		t.Fatalf("ReconcileSourcesCatalog failed: %v", err)
	}

	// User doc MUST be preserved!
	resBytes, err := os.ReadFile(userDoc)
	if err != nil {
		t.Fatalf("expected user doc to be preserved: %v", err)
	}
	resContent := string(resBytes)

	// Content must be intact
	if !strings.Contains(resContent, "This is human-written technical documentation about microservices.") {
		t.Errorf("expected human-written text to be preserved")
	}
	// Erroneous type: repository MUST be stripped so it doesn't masquerade as a repo
	if strings.Contains(resContent, "type: repository") {
		t.Errorf("expected type: repository to be removed from subfolder doc frontmatter")
	}
}

func TestReconcileSourcesCatalog_GitHubSingleIndex(t *testing.T) {
	tempVault := t.TempDir()

	repoDir := filepath.Join(tempVault, "sources", "github", "octocat", "hello-world")
	docsDir := filepath.Join(repoDir, "docs", "deep")
	_ = os.MkdirAll(docsDir, 0750)

	_ = os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("# Hello World"), 0600)
	_ = os.WriteFile(filepath.Join(docsDir, "guide.md"), []byte("# Guide"), 0600)

	// Purely spurious boilerplate MOC
	spurious := filepath.Join(docsDir, "index.md")
	_ = os.WriteFile(spurious, []byte("---\ntype: repository\nrepository: deep\n---\n# 📦 deep\n- [guide](guide.md)\n"), 0600)

	stats, err := ReconcileSourcesCatalog(tempVault)
	if err != nil {
		t.Fatalf("ReconcileSourcesCatalog failed: %v", err)
	}

	if stats.TotalReposIndexed != 1 {
		t.Errorf("expected 1 repo indexed, got %d", stats.TotalReposIndexed)
	}

	rootIndex := filepath.Join(repoDir, "index.md")
	if _, err := os.Stat(rootIndex); os.IsNotExist(err) {
		t.Fatalf("expected root index.md to exist")
	}

	// Pure boilerplate spurious file should be purged
	if _, err := os.Stat(spurious); !os.IsNotExist(err) {
		t.Errorf("expected spurious index.md to be deleted")
	}
}
