package search

import (
	"fmt"
	"strings"
	"testing"
)

func TestChunkMarkdown_Empty(t *testing.T) {
	if chunks := ChunkMarkdown("notes/empty.md", "Empty", "   \n\n\t ", 350, 40); len(chunks) != 0 {
		t.Fatalf("expected 0 chunks for empty content, got %d", len(chunks))
	}
}

func TestChunkMarkdown_HeadingHierarchyAndBreadcrumbs(t *testing.T) {
	md := `Intro paragraph before any heading.

# Overview
System overview text.

## Storage
Storage layer details.

### SQLite FTS5
Full-text search configuration.

## Networking
Networking layer details.
`
	chunks := ChunkMarkdown("notes/arch.md", "Architecture", md, 350, 40)
	if len(chunks) != 5 {
		t.Fatalf("expected 5 chunks, got %d", len(chunks))
	}

	expected := []struct {
		heading string
		prefix  string
		body    string
	}{
		{"", "Architecture", "Intro paragraph before any heading."},
		{"Overview", "Architecture > Overview", "System overview text."},
		{"Overview > Storage", "Architecture > Overview > Storage", "Storage layer details."},
		{"Overview > Storage > SQLite FTS5", "Architecture > Overview > Storage > SQLite FTS5", "Full-text search configuration."},
		{"Overview > Networking", "Architecture > Overview > Networking", "Networking layer details."},
	}

	for i, exp := range expected {
		if chunks[i].ChunkIdx != i {
			t.Errorf("chunk %d: expected ChunkIdx %d, got %d", i, i, chunks[i].ChunkIdx)
		}
		if chunks[i].RelPath != "notes/arch.md" {
			t.Errorf("chunk %d: expected RelPath notes/arch.md, got %s", i, chunks[i].RelPath)
		}
		if chunks[i].Heading != exp.heading {
			t.Errorf("chunk %d: expected heading %q, got %q", i, exp.heading, chunks[i].Heading)
		}
		if !strings.HasPrefix(chunks[i].Content, exp.prefix+"\n\n") {
			t.Errorf("chunk %d: expected content prefix %q, got %q", i, exp.prefix, chunks[i].Content)
		}
		if !strings.Contains(chunks[i].Content, exp.body) {
			t.Errorf("chunk %d: expected body %q in content %q", i, exp.body, chunks[i].Content)
		}
		if len(chunks[i].ContentHash) != 64 {
			t.Errorf("chunk %d: expected 64-char sha256 hex, got %q", i, chunks[i].ContentHash)
		}
	}
}

func TestChunkMarkdown_RespectsFencedCodeBlocks(t *testing.T) {
	md := `## Deployment

Run the following script:

` + "```bash" + `
# This is a bash comment, NOT a heading
## Another comment
echo "hello"
` + "```" + `

And with tilde fences:

~~~markdown
# Fake H1 inside tilde fence
## Fake H2 inside tilde fence
~~~

### Verification
Verify the service is healthy.
`
	chunks := ChunkMarkdown("notes/deploy.md", "Service Guide", md, 350, 40)
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks (headings inside code fences must not split), got %d", len(chunks))
	}
	if chunks[0].Heading != "Deployment" {
		t.Errorf("expected first chunk heading 'Deployment', got %q", chunks[0].Heading)
	}
	if !strings.Contains(chunks[0].Content, "# This is a bash comment") {
		t.Errorf("expected bash comment preserved inside first chunk")
	}
	if !strings.Contains(chunks[0].Content, "# Fake H1 inside tilde fence") {
		t.Errorf("expected tilde fence content preserved inside first chunk")
	}
	if chunks[1].Heading != "Deployment > Verification" {
		t.Errorf("expected second chunk heading 'Deployment > Verification', got %q", chunks[1].Heading)
	}
}

func TestChunkMarkdown_SplitsLargeSectionWithOverlap(t *testing.T) {
	makePara := func(prefix string, n int) string {
		words := make([]string, n)
		for i := range words {
			words[i] = fmt.Sprintf("%s%d", prefix, i)
		}
		return strings.Join(words, " ")
	}

	p1 := makePara("alpha", 12)
	p2 := makePara("beta", 12)
	p3 := makePara("gamma", 12)
	md := "## Big Section\n\n" + p1 + "\n\n" + p2 + "\n\n" + p3

	chunks := ChunkMarkdown("notes/big.md", "Big Note", md, 20, 4)
	if len(chunks) < 2 {
		t.Fatalf("expected section to split into multiple chunks, got %d", len(chunks))
	}

	// Verify overlap: trailing words of chunk 0 should appear at start of chunk 1 body.
	if !strings.Contains(chunks[1].Content, "alpha11") && !strings.Contains(chunks[1].Content, "beta") {
		t.Errorf("expected overlap words in second chunk, got %q", chunks[1].Content)
	}
	if !strings.Contains(chunks[0].Content, "alpha8 alpha9 alpha10 alpha11") {
		t.Errorf("expected p1 in first chunk, got %q", chunks[0].Content)
	}
	if !strings.Contains(chunks[1].Content, "alpha8 alpha9 alpha10 alpha11") {
		t.Errorf("expected 4-word overlap from p1 in second chunk, got %q", chunks[1].Content)
	}
}

func TestChunkMarkdown_DeterministicHash(t *testing.T) {
	md := "## Section\n\nDeterministic content here."
	c1 := ChunkMarkdown("a.md", "Title", md, 350, 40)
	c2 := ChunkMarkdown("a.md", "Title", md, 350, 40)
	if len(c1) != 1 || len(c2) != 1 {
		t.Fatalf("expected 1 chunk each")
	}
	if c1[0].ContentHash != c2[0].ContentHash {
		t.Errorf("expected deterministic hash, got %s vs %s", c1[0].ContentHash, c2[0].ContentHash)
	}
}
