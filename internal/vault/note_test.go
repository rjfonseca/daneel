//nolint:revive // package comment not required
package vault

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseNote(t *testing.T) {
	tempDir := t.TempDir()

	notePath := filepath.Join(tempDir, "test-note.md")
	content := `---
type: tecnologia
description: Nota de teste para o parser OKF.
tags:
  - golang
  - ia
---

# Test Note

Este é um documento de teste com link [Outra Nota](../dados/outra-nota.md) e [[Wiki Link|Texto]].

## Seção Secundária
Conteúdo adicional.
`
	if err := os.WriteFile(notePath, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write test note: %v", err)
	}

	note, err := ParseNote(notePath, tempDir)
	if err != nil {
		t.Fatalf("ParseNote failed: %v", err)
	}

	if note.Title != "Test Note" {
		t.Errorf("expected Title 'Test Note', got '%s'", note.Title)
	}
	if note.NoteType != "tecnologia" {
		t.Errorf("expected NoteType 'tecnologia', got '%s'", note.NoteType)
	}
	if note.Description != "Nota de teste para o parser OKF." {
		t.Errorf("expected Description, got '%s'", note.Description)
	}
	if len(note.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(note.Tags))
	}
	if len(note.Links) != 2 {
		t.Errorf("expected 2 links, got %d", len(note.Links))
	}
}

func TestExtractLinks_IgnoreCodeBlocks(t *testing.T) {
	content := `
# My Document

Here is a real link: [Real Note](real-note.md) and [[WikiNote]].

Here is a markdown code block showing link syntax:
` + "```markdown" + `
[Example](fake/path.md)
[[FakeWiki]]
` + "```" + `

Here is a Go generic function inside code block:
` + "```go" + `
func Handler[T any](input T) {}
` + "```" + `

And inline code: ` + "`[Inline Fake](inline/fake.md)`" + `

End of document.
`
	links := ExtractLinks(content, "notes/test.md", "/vault")
	if len(links) != 2 {
		t.Fatalf("expected exactly 2 real links, got %d: %+v", len(links), links)
	}

	if links[0].LinkText != "Real Note" {
		t.Errorf("expected first link 'Real Note', got '%s'", links[0].LinkText)
	}
	if links[1].LinkText != "WikiNote" {
		t.Errorf("expected second link 'WikiNote', got '%s'", links[1].LinkText)
	}
}

func TestDetermineScope(t *testing.T) {
	tests := []struct {
		name     string
		relPath  string
		expected string
	}{
		{"Tech Note", "notes/ia/rag.md", "tech"},
		{"Daily Log", "daily/2026-08-18.md", "daily"},
		{"Meta Info", "meta/carreira/pdi.md", "meta"},
		{"Inbox Draft", "inbox/ideia.md", "inbox"},
		{"Sources Archive", "sources/reunioes/meet.md", "sources"},
		{"Public Wiki", "public/wiki/artigo.md", "public"},
		{"Public Content", "public/content/post.md", "public"},
		{"Root Note", "overview.md", "other"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetermineScope(tt.relPath)
			if got != tt.expected {
				t.Errorf("DetermineScope(%q) = %q, expected %q", tt.relPath, got, tt.expected)
			}
		})
	}
}

func TestDetermineCategory(t *testing.T) {
	tests := []struct {
		name     string
		relPath  string
		expected string
	}{
		{"Root notes category", "notes/arquitetura/cache.md", "notes/arquitetura"},
		{"Deep notes category collapses to section", "notes/arquitetura/sub/cache.md", "notes/arquitetura"},
		{"Daily note", "daily/2026-08-18.md", "daily"},
		{"GitHub repo docs collapse to repo root", "sources/github/acme/data-pipeline/docs/index.md", "sources/github/acme/data-pipeline"},
		{"GitLab repo specs collapse to repo root (gateway)", "sources/gitlab/acme/edge/gateway/docs/superpowers/specs/2026-03-09.md", "sources/gitlab/acme/edge/gateway"},
		{"GitLab repo conductor collapse to repo root", "sources/gitlab/acme/edge/gateway/conductor/tracks/recup/spec.md", "sources/gitlab/acme/edge/gateway"},
		{"Sources meeting archive", "sources/reunioes/2026-09-08-frontend-alignment.md", "sources/reunioes"},
		{"Sources jira post-mortems", "sources/jira/post-mortems/INCIDENT-123.md", "sources/jira/post-mortems"},
		{"GitLab saas repo frontend docs", "sources/gitlab/saas/frontend/docs/readme.md", "sources/gitlab/saas/frontend"},
		{"GitLab saas repo frontend root file", "sources/gitlab/saas/frontend/README.md", "sources/gitlab/saas/frontend"},
		{"GitLab saas repo backend nested file", "sources/gitlab/saas/backend/sub/api.md", "sources/gitlab/saas/backend"},
		{"GitLab apis repo customer-api docs", "sources/gitlab/apis/customer-api/docs/spec.md", "sources/gitlab/apis/customer-api"},
		{"GitLab org saas repo portal", "sources/gitlab/myorg/saas/portal/docs/arch.md", "sources/gitlab/myorg/saas/portal"},
		{"GitLab org apis repo gateway", "sources/gitlab/myorg/apis/gateway/specs/v1.md", "sources/gitlab/myorg/apis/gateway"},
		{"GitLab saas repo named tools", "sources/gitlab/saas/tools/docs/cli.md", "sources/gitlab/saas/tools"},
		{"GitLab stores repo checkout docs", "sources/gitlab/stores/checkout/docs/readme.md", "sources/gitlab/stores/checkout"},
		{"GitLab org stores repo catalog specs", "sources/gitlab/myorg/stores/catalog/specs/api.md", "sources/gitlab/myorg/stores/catalog"},
		{"Root note", "README.md", "root"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetermineCategory(tt.relPath)
			if got != tt.expected {
				t.Errorf("DetermineCategory(%q) = %q, expected %q", tt.relPath, got, tt.expected)
			}
		})
	}
}

func TestNormalizeText(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Ação e Reação", "acao e reacao"},
		{"Crédito Tributário", "credito tributario"},
		{"GOLANG & Microservices", "golang & microservices"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := NormalizeText(tt.input)
			if got != tt.expected {
				t.Errorf("NormalizeText(%q) = %q, expected %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestResolveTaxonomy_OKFAncestors(t *testing.T) {
	tempVault := t.TempDir()

	// Setup mock structure
	pocsDir := filepath.Join(tempVault, "sources", "gitlab", "acme", "saas", "pocs")
	repoDir := filepath.Join(pocsDir, "my-repo")
	docsDir := filepath.Join(repoDir, "docs")
	_ = os.MkdirAll(docsDir, 0750)

	// Group MOC
	pocsMOC := "---\ntype: group\ngroup: acme/saas/pocs\n---\n# Poc Group"
	_ = os.WriteFile(filepath.Join(pocsDir, "index.md"), []byte(pocsMOC), 0600)

	// Repository MOC
	repoMOC := "---\ntype: repository\nrepository: my-repo\ngroup: acme/saas/pocs\n---\n# My Repo"
	_ = os.WriteFile(filepath.Join(repoDir, "index.md"), []byte(repoMOC), 0600)

	// Repo document
	guideDoc := "# Guide\nContent"
	_ = os.WriteFile(filepath.Join(docsDir, "guide.md"), []byte(guideDoc), 0600)

	// 1. Group MOC resolution
	groupTax := ResolveTaxonomy("sources/gitlab/acme/saas/pocs/index.md", tempVault)
	if groupTax.Domain != "sources/gitlab" || groupTax.Group != "acme/saas/pocs" || groupTax.Repository != "" {
		t.Errorf("Group MOC taxonomy mismatch: %+v", groupTax)
	}

	// 2. Repo MOC resolution
	repoTax := ResolveTaxonomy("sources/gitlab/acme/saas/pocs/my-repo/index.md", tempVault)
	if repoTax.Domain != "sources/gitlab" || repoTax.Group != "acme/saas/pocs" || repoTax.Repository != "my-repo" {
		t.Errorf("Repo MOC taxonomy mismatch: %+v", repoTax)
	}

	// 3. Nested document in repo resolution
	docTax := ResolveTaxonomy("sources/gitlab/acme/saas/pocs/my-repo/docs/guide.md", tempVault)
	if docTax.Domain != "sources/gitlab" || docTax.Group != "acme/saas/pocs" || docTax.Repository != "my-repo" {
		t.Errorf("Nested doc taxonomy mismatch: %+v", docTax)
	}
	if docTax.Category != "sources/gitlab/acme/saas/pocs/my-repo" {
		t.Errorf("Nested doc category = %q, expected %q", docTax.Category, "sources/gitlab/acme/saas/pocs/my-repo")
	}
}
