package toon

import (
	"encoding/json"
	"fmt"
	"testing"
)

type searchResultItem struct {
	Title    string  `json:"title"`
	RelPath  string  `json:"rel_path"`
	Scope    string  `json:"scope"`
	NoteType string  `json:"note_type"`
	Snippet  string  `json:"snippet"`
	Score    float64 `json:"score"`
}

func TestBenchmarkTokenSavings_10Notes(t *testing.T) {
	var items []searchResultItem
	var toonRows [][]string

	for i := 1; i <= 10; i++ {
		title := fmt.Sprintf("Microservices Architecture Chapter %d", i)
		relPath := fmt.Sprintf("notes/architecture/service-%02d.md", i)
		scope := "notes/architecture"
		noteType := "tecnologia"
		snippet := fmt.Sprintf("Comprehensive architectural analysis for distributed service %d with event-driven state transitions.", i)
		score := 12.50 - float64(i)*0.4

		items = append(items, searchResultItem{
			Title:    title,
			RelPath:  relPath,
			Scope:    scope,
			NoteType: noteType,
			Snippet:  snippet,
			Score:    score,
		})

		toonRows = append(toonRows, []string{
			title,
			relPath,
			scope,
			noteType,
			snippet,
			fmt.Sprintf("%.2f", score),
		})
	}

	jsonBytes, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		t.Fatalf("json marshal failed: %v", err)
	}
	jsonStr := string(jsonBytes)

	headers := []string{"title", "path", "scope", "type", "snippet", "score"}
	toonStr := EncodeTabular("results", headers, toonRows)

	jsonTokens, toonTokens, savings := CalculateSavings(jsonStr, toonStr)

	t.Logf("10 Notes Search Results:")
	t.Logf("  JSON: %d bytes, ~%d estimated tokens", len(jsonStr), jsonTokens)
	t.Logf("  TOON: %d bytes, ~%d estimated tokens", len(toonStr), toonTokens)
	t.Logf("  Real Token Savings: %.2f%%", savings)

	if savings < 40.0 {
		t.Errorf("expected at least 40%% token savings, got %.2f%%", savings)
	}
}

func TestBenchmarkTokenSavings_20Projects(t *testing.T) {
	type projectItem struct {
		Name   string `json:"name"`
		Path   string `json:"path"`
		Branch string `json:"branch"`
		Status string `json:"status"`
		Stack  string `json:"stack"`
	}

	var items []projectItem
	var toonRows [][]string

	for i := 1; i <= 20; i++ {
		name := fmt.Sprintf("service-%02d", i)
		path := fmt.Sprintf("/home/user/Work/acme.corp/%s", name)
		branch := "main"
		status := "clean"
		stack := "go"
		if i%3 == 0 {
			branch = "feature/token-optimization"
			status = "dirty"
			stack = "typescript"
		}

		items = append(items, projectItem{
			Name:   name,
			Path:   path,
			Branch: branch,
			Status: status,
			Stack:  stack,
		})

		toonRows = append(toonRows, []string{name, path, branch, status, stack})
	}

	jsonBytes, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		t.Fatalf("json marshal failed: %v", err)
	}
	jsonStr := string(jsonBytes)

	headers := []string{"name", "path", "branch", "status", "stack"}
	toonStr := EncodeTabular("projects", headers, toonRows)

	jsonTokens, toonTokens, savings := CalculateSavings(jsonStr, toonStr)

	t.Logf("20 Projects Catalog Results:")
	t.Logf("  JSON: %d bytes, ~%d estimated tokens", len(jsonStr), jsonTokens)
	t.Logf("  TOON: %d bytes, ~%d estimated tokens", len(toonStr), toonTokens)
	t.Logf("  Real Token Savings: %.2f%%", savings)

	if savings < 50.0 {
		t.Errorf("expected at least 50%% token savings for uniform projects, got %.2f%%", savings)
	}
}
