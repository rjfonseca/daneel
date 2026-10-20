//nolint:revive // package comment not required
package search

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/rjfonseca/daneel/internal/ops/workspace"
)

func TestDiscoveryAndEntityResolution(t *testing.T) {
	tempDir := t.TempDir()
	ctx := context.Background()

	// 1. Create notes directory structure
	notesArchDir := filepath.Join(tempDir, "notes", "arquitetura")
	notesPessoasDir := filepath.Join(tempDir, "notes", "pessoas")
	_ = os.MkdirAll(notesArchDir, 0750)
	_ = os.MkdirAll(notesPessoasDir, 0750)

	// Note 1: Architecture note with aliases
	noteScaffold := filepath.Join(notesArchDir, "scaffold-build-system.md")
	scaffoldContent := `---
type: arquitetura
description: Scaffold Build System & Governança Cross-Squad
tags:
  - build
  - ci-cd
aliases:
  - "Scaffold System"
  - "Build System"
---
# Scaffold Build System & Governança Cross-Squad

Documentação técnica da governança de builds.
`
	_ = os.WriteFile(noteScaffold, []byte(scaffoldContent), 0600)

	// Note 2: Person note with compound name, aliases, and role/squad
	noteLovelace := filepath.Join(notesPessoasDir, "augusta-ada-byron-king-lovelace.md")
	lovelaceContent := `---
type: person
description: Professional profile of Augusta Ada Byron King Lovelace.
role: Engineering Manager
squad: Platform Core / Architecture
email: ada.lovelace@acme.corp
aliases:
  - "Augusta Ada Byron King Lovelace"
  - "Augusta Ada Byron King Lovelace (lovelace)"
  - "lovelace"
tags:
  - person
---
# 👤 Augusta Ada Byron King Lovelace

Perfil e histórico de interações.
`
	_ = os.WriteFile(noteLovelace, []byte(lovelaceContent), 0600)

	// Note 3: Another person note with typo tolerance testing
	noteTuring := filepath.Join(notesPessoasDir, "alan-mathison-turing.md")
	turingContent := `---
type: person
description: Professional profile of Alan Mathison Turing.
role: Advanced Software Engineer
squad: Squad Ingestion - Capture
email: alan.turing@acme.corp
aliases:
  - "Alan Turing"
tags:
  - person
  - capture
---
# 👤 Alan Mathison Turing

Engenheiro de software sênior.
`
	_ = os.WriteFile(noteTuring, []byte(turingContent), 0600)

	dbPath := filepath.Join(tempDir, "vault.db")
	engine, err := Open(dbPath, tempDir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer engine.Close() //nolint:errcheck,gosec

	// Index vault
	_, _, err = engine.IndexVault(ctx, []string{"notes"}, true)
	if err != nil {
		t.Fatalf("IndexVault failed: %v", err)
	}

	// ----------------------------------------------------
	// Test Primitive 1: ResolveNote (Fuzzy Resolution)
	// ----------------------------------------------------
	t.Run("ResolveNote_ExactAndFuzzy", func(t *testing.T) {
		// Test A: slug resolution
		res, err := engine.ResolveNote(ctx, "scaffold build system")
		if err != nil {
			t.Fatalf("ResolveNote failed: %v", err)
		}
		if !res.Found || res.Match == nil {
			t.Fatalf("expected to find note, got not found")
		}
		if res.Match.RelPath != "notes/arquitetura/scaffold-build-system.md" {
			t.Errorf("expected rel_path 'notes/arquitetura/scaffold-build-system.md', got '%s'", res.Match.RelPath)
		}
		if res.Match.Score < 0.90 {
			t.Errorf("expected score >= 0.90, got %f", res.Match.Score)
		}

		// Test B: alias resolution
		resAlias, err := engine.ResolveNote(ctx, "Scaffold System")
		if err != nil {
			t.Fatalf("ResolveNote failed: %v", err)
		}
		if !resAlias.Found || resAlias.Match == nil {
			t.Fatalf("expected to resolve alias, got not found")
		}
		if resAlias.Match.RelPath != "notes/arquitetura/scaffold-build-system.md" {
			t.Errorf("expected alias to match scaffold note, got '%s'", resAlias.Match.RelPath)
		}

		// Test C: non-existent note
		resNone, err := engine.ResolveNote(ctx, "nonexistent-quantum-core-xyz")
		if err != nil {
			t.Fatalf("ResolveNote unexpected error: %v", err)
		}
		if resNone.Found {
			t.Errorf("expected found=false for nonexistent note, got true")
		}
	})

	// ----------------------------------------------------
	// Test Primitive 2: SearchPeople (Compound Name & Filters)
	// ----------------------------------------------------
	t.Run("SearchPeople_CompoundMatchingAndFilters", func(t *testing.T) {
		// Test A: Compound name "Ada Lovelace" matching "Augusta Ada Byron King Lovelace"
		people, err := engine.SearchPeople(ctx, "Ada Lovelace", "", "", "")
		if err != nil {
			t.Fatalf("SearchPeople failed: %v", err)
		}
		if len(people) == 0 {
			t.Fatalf("expected to find Ada Lovelace, got 0 results")
		}
		if people[0].Name != "Augusta Ada Byron King Lovelace" {
			t.Errorf("expected 'Augusta Ada Byron King Lovelace', got '%s'", people[0].Name)
		}
		if people[0].Role != "Engineering Manager" {
			t.Errorf("expected role 'Engineering Manager', got '%s'", people[0].Role)
		}

		// Test B: Nickname / Alias "lovelace"
		peopleLovelace, err := engine.SearchPeople(ctx, "lovelace", "", "", "")
		if err != nil {
			t.Fatalf("SearchPeople failed: %v", err)
		}
		if len(peopleLovelace) == 0 {
			t.Fatalf("expected to find lovelace, got 0")
		}

		// Test C: Typo tolerance "Turin" matching "Turing"
		peopleTurin, err := engine.SearchPeople(ctx, "Turin", "", "", "")
		if err != nil {
			t.Fatalf("SearchPeople failed: %v", err)
		}
		if len(peopleTurin) == 0 {
			t.Fatalf("expected typo tolerance to find Turing, got 0")
		}
		if peopleTurin[0].Name != "Alan Mathison Turing" {
			t.Errorf("expected 'Alan Mathison Turing', got '%s'", peopleTurin[0].Name)
		}

		// Test D: Squad filter
		squadPeople, err := engine.SearchPeople(ctx, "", "Capture", "", "")
		if err != nil {
			t.Fatalf("SearchPeople squad filter failed: %v", err)
		}
		if len(squadPeople) != 1 || squadPeople[0].Name != "Alan Mathison Turing" {
			t.Errorf("expected only Turing for squad Capture, got %v", squadPeople)
		}

		// Test E: Role filter
		rolePeople, err := engine.SearchPeople(ctx, "", "", "Engineering Manager", "")
		if err != nil {
			t.Fatalf("SearchPeople role filter failed: %v", err)
		}
		if len(rolePeople) != 1 || rolePeople[0].Name != "Augusta Ada Byron King Lovelace" {
			t.Errorf("expected only Lovelace for role Engineering Manager, got %v", rolePeople)
		}
	})

	// ----------------------------------------------------
	// Test Primitive 3: SearchProjects (Workspace SQLite Cache)
	// ----------------------------------------------------
	t.Run("SearchProjects_CachingAndQueries", func(t *testing.T) {
		repos := []workspace.RepoInfo{
			{
				Name:        "notification-service",
				RelPath:     "gitlab.com/acme/platform/notification-service",
				Path:        filepath.Join(tempDir, "work", "notification-service"),
				Branch:      "main",
				Dirty:       false,
				Stack:       "Go (1.24)",
				Description: "Example notification delivery service",
			},
			{
				Name:        "gateway",
				RelPath:     "github.com/acme/gateway",
				Path:        filepath.Join(tempDir, "work", "gateway"),
				Branch:      "feat/parser",
				Dirty:       true,
				Stack:       "TypeScript",
				Description: "Example API gateway",
			},
		}

		err := engine.SaveWorkspaceRepos(ctx, repos)
		if err != nil {
			t.Fatalf("SaveWorkspaceRepos failed: %v", err)
		}

		// Query by name
		resNotifications, err := engine.SearchProjects(ctx, "notification-service", false, false, "", 10)
		if err != nil {
			t.Fatalf("SearchProjects failed: %v", err)
		}
		if len(resNotifications) != 1 || resNotifications[0].Name != "notification-service" {
			t.Errorf("expected to find notification-service repo, got %v", resNotifications)
		}

		// Query dirty only
		resDirty, err := engine.SearchProjects(ctx, "", true, false, "", 10)
		if err != nil {
			t.Fatalf("SearchProjects dirtyOnly failed: %v", err)
		}
		if len(resDirty) != 1 || resDirty[0].Name != "gateway" {
			t.Errorf("expected only gateway as dirty, got %v", resDirty)
		}

		// Query clean only
		resClean, err := engine.SearchProjects(ctx, "", false, true, "", 10)
		if err != nil {
			t.Fatalf("SearchProjects cleanOnly failed: %v", err)
		}
		if len(resClean) != 1 || resClean[0].Name != "notification-service" {
			t.Errorf("expected only notification-service as clean, got %v", resClean)
		}

		// Query by stack
		resStack, err := engine.SearchProjects(ctx, "", false, false, "go", 10)
		if err != nil {
			t.Fatalf("SearchProjects stack filter failed: %v", err)
		}
		if len(resStack) != 1 || resStack[0].Name != "notification-service" {
			t.Errorf("expected notification-service with stack Go, got %v", resStack)
		}
	})

	// ----------------------------------------------------
	// Test Primitive 4: Faceted Search (SearchFaceted)
	// ----------------------------------------------------
	t.Run("SearchFaceted_TypeAndTag", func(t *testing.T) {
		// Search with query and type
		resType, err := engine.SearchFaceted(ctx, SearchOptions{
			Query:    "governança",
			NoteType: "arquitetura",
		})
		if err != nil {
			t.Fatalf("SearchFaceted failed: %v", err)
		}
		if len(resType) != 1 || resType[0].RelPath != "notes/arquitetura/scaffold-build-system.md" {
			t.Errorf("expected scaffold note, got %v", resType)
		}

		// Search with type only (no query)
		resTypeOnly, err := engine.SearchFaceted(ctx, SearchOptions{
			NoteType: "person",
		})
		if err != nil {
			t.Fatalf("SearchFaceted type only failed: %v", err)
		}
		if len(resTypeOnly) != 2 {
			t.Errorf("expected 2 person notes, got %d", len(resTypeOnly))
		}

		// Search with tag only (no query)
		resTagOnly, err := engine.SearchFaceted(ctx, SearchOptions{
			Tag: "ci-cd",
		})
		if err != nil {
			t.Fatalf("SearchFaceted tag only failed: %v", err)
		}
		if len(resTagOnly) != 1 || resTagOnly[0].RelPath != "notes/arquitetura/scaffold-build-system.md" {
			t.Errorf("expected scaffold note for tag ci-cd, got %v", resTagOnly)
		}
	})
}
