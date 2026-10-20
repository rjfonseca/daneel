//nolint:revive // package comment not required
package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rjfonseca/daneel/internal/config"
	"github.com/rjfonseca/daneel/internal/ops/workspace"
	"github.com/rjfonseca/daneel/internal/search"
)

func TestDiscoveryCommands(t *testing.T) {
	tempVault := t.TempDir()

	_ = os.MkdirAll(filepath.Join(tempVault, "notes", "arquitetura"), 0750)
	_ = os.MkdirAll(filepath.Join(tempVault, "notes", "pessoas"), 0750)

	// Note 1: Architecture note
	archFile := filepath.Join(tempVault, "notes", "arquitetura", "scaffold-build-system.md")
	_ = os.WriteFile(archFile, []byte(`---
type: arquitetura
description: Scaffold Build System & Governança Cross-Squad
tags:
  - build
  - ci-cd
aliases:
  - "Scaffold System"
---
# Scaffold Build System & Governança Cross-Squad
Build system documentation.
`), 0600)

	// Note 2: Person note
	personFile := filepath.Join(tempVault, "notes", "pessoas", "alice-carol-smith.md")
	_ = os.WriteFile(personFile, []byte(`---
type: person
role: Engineering Manager
squad: Tribo Platform
email: alice.smith@acme.corp
aliases:
  - "Alice Carol Smith"
  - "Alice Carol Smith (asmith)"
  - "asmith"
tags:
  - person
---
# 👤 Alice Carol Smith
Profile notes.
`), 0600)

	cfg = &config.Config{
		VaultPath: tempVault,
		DBPath:    filepath.Join(tempVault, ".daneel", "cache", "vault_index.db"),
		Vault:     config.VaultConfig{IndexedDirs: []string{"notes"}},
	}

	// Index vault
	engine, err := search.Open(cfg.DBPath, cfg.VaultPath)
	if err != nil {
		t.Fatalf("Open search engine failed: %v", err)
	}
	_, _, err = engine.IndexVault(t.Context(), []string{"notes"}, true)
	if err != nil {
		t.Fatalf("IndexVault failed: %v", err)
	}

	// Seed workspace_repos for project testing
	repos := []workspace.RepoInfo{
		{
			Name:        "gateway",
			RelPath:     "gitlab.com/acme/platform/gateway",
			Path:        filepath.Join(tempVault, "work", "gateway"),
			Branch:      "main",
			Dirty:       false,
			Stack:       "Go (1.24)",
			Description: "API gateway service",
		},
		{
			Name:        "tree-engine",
			RelPath:     "github.com/acme/tree-engine",
			Path:        filepath.Join(tempVault, "work", "tree-engine"),
			Branch:      "master",
			Dirty:       true,
			Stack:       "TypeScript",
			Description: "Behavior-Tree Execution Engine",
		},
	}
	_ = engine.SaveWorkspaceRepos(t.Context(), repos)
	_ = engine.Close()

	// 1. Test daneel resolve
	t.Run("ResolveCommand", func(t *testing.T) {
		err := resolveCmd.RunE(resolveCmd, []string{"scaffold", "build", "system"})
		if err != nil {
			t.Errorf("resolveCmd failed: %v", err)
		}

		// With JSON
		jsonOut = true
		defer func() { jsonOut = false }()
		err = resolveCmd.RunE(resolveCmd, []string{"asmith"})
		if err != nil {
			t.Errorf("resolveCmd with JSON failed: %v", err)
		}
	})

	// 2. Test daneel people
	t.Run("PeopleCommand", func(t *testing.T) {
		peopleSquad = ""
		peopleRole = ""
		peopleEmail = ""

		err := peopleCmd.RunE(peopleCmd, []string{"Alice Smith"})
		if err != nil {
			t.Errorf("peopleCmd with query failed: %v", err)
		}

		// Filter by squad
		peopleSquad = "Platform"
		err = peopleCmd.RunE(peopleCmd, []string{})
		if err != nil {
			t.Errorf("peopleCmd with squad failed: %v", err)
		}
		peopleSquad = ""

		// Filter by role
		peopleRole = "Engineering Manager"
		err = peopleCmd.RunE(peopleCmd, []string{})
		if err != nil {
			t.Errorf("peopleCmd with role failed: %v", err)
		}
		peopleRole = ""
	})

	// 3. Test daneel projects
	t.Run("ProjectsCommand", func(t *testing.T) {
		projectsDirty = false
		projectsClean = false
		projectsStack = ""
		projectsLimit = 10

		err := projectsCmd.RunE(projectsCmd, []string{"gateway"})
		if err != nil {
			t.Errorf("projectsCmd failed: %v", err)
		}

		// Dirty only
		projectsDirty = true
		err = projectsCmd.RunE(projectsCmd, []string{})
		if err != nil {
			t.Errorf("projectsCmd dirty failed: %v", err)
		}
		projectsDirty = false

		// Stack filter
		projectsStack = "go"
		err = projectsCmd.RunE(projectsCmd, []string{})
		if err != nil {
			t.Errorf("projectsCmd stack failed: %v", err)
		}
		projectsStack = ""
	})

	// 4. Test daneel search faceted
	t.Run("SearchFacetedCommand", func(t *testing.T) {
		searchScope = "all"
		searchType = "arquitetura"
		searchTag = ""
		searchLimit = 10

		err := searchCmd.RunE(searchCmd, []string{"governança"})
		if err != nil {
			t.Errorf("searchCmd with type failed: %v", err)
		}

		// Search with tag only (no query args)
		searchType = ""
		searchTag = "ci-cd"
		err = searchCmd.RunE(searchCmd, []string{})
		if err != nil {
			t.Errorf("searchCmd with tag failed: %v", err)
		}
		searchTag = ""
	})
}
