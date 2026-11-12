//nolint:revive // package comment not required
package mcp

import (
	"strings"
	"testing"
)

func TestToolCatalog_Search(t *testing.T) {
	cat := NewToolCatalog()

	// 1. Search by exact keyword
	res := cat.Search("workspace git", "", 5)
	if len(res) == 0 {
		t.Fatalf("expected tools for 'workspace git', got 0")
	}
	foundScan := false
	for _, tool := range res {
		if tool.Name == "scan_workspace_repos" || tool.Name == "clone_repository" {
			foundScan = true
			break
		}
	}
	if !foundScan {
		t.Errorf("expected scan_workspace_repos or clone_repository in results, got: %+v", res)
	}

	// 2. Search with category filter
	uiTools := cat.Search("note", "ui", 10)
	for _, tool := range uiTools {
		if tool.Category != "ui" {
			t.Errorf("expected category 'ui', got: %s (tool: %s)", tool.Category, tool.Name)
		}
	}

	// 3. Search for people tools
	peopleTools := cat.Search("colleague 1:1 briefing", "people", 5)
	if len(peopleTools) == 0 {
		t.Fatalf("expected people tools, got 0")
	}
	if peopleTools[0].Name != "person_briefing" && peopleTools[0].Name != "search_people" {
		t.Errorf("unexpected top tool for people briefing: %s", peopleTools[0].Name)
	}
}

func TestToolCatalog_FormatTOON(t *testing.T) {
	cat := NewToolCatalog()
	tools := cat.Search("search_vault", "", 1)
	if len(tools) == 0 {
		t.Fatalf("expected search_vault tool, got 0")
	}

	toonStr := cat.FormatTOON(tools)
	if !strings.Contains(toonStr, "tools[1") || !strings.Contains(toonStr, "{name") {
		t.Errorf("expected TOON header for tools, got: %s", toonStr)
	}
	if !strings.Contains(toonStr, "search_vault") {
		t.Errorf("expected search_vault in TOON string, got: %s", toonStr)
	}
}
