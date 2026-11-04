package vault_test

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/rjfonseca/daneel/internal/vault"
)

func TestScanVaultIncremental_SkipsUnchangedAndDetectsModified(t *testing.T) {
	tmpDir := t.TempDir()

	archDir := filepath.Join(tmpDir, "notes", "arch")
	if err := os.MkdirAll(archDir, 0750); err != nil {
		t.Fatalf("failed to create notes/arch: %v", err)
	}

	designPath := filepath.Join(archDir, "design.md")
	targetPath := filepath.Join(archDir, "target.md")

	designContent := `---
type: architecture
description: Core design
---
# Design

See [Target](target.md)
`
	targetContent := `# Target

Some target content.
`

	if err := os.WriteFile(designPath, []byte(designContent), 0600); err != nil {
		t.Fatalf("failed to write design.md: %v", err)
	}
	if err := os.WriteFile(targetPath, []byte(targetContent), 0600); err != nil {
		t.Fatalf("failed to write target.md: %v", err)
	}

	// 1. Initial scan: cold cache
	notes, err := vault.ScanVaultIncremental(tmpDir, []string{"notes"}, false)
	if err != nil {
		t.Fatalf("initial ScanVaultIncremental failed: %v", err)
	}
	if len(notes) != 2 {
		t.Fatalf("expected 2 notes on initial scan, got %d", len(notes))
	}

	// Verify SQLite database was created and has correct watermark rows
	dbPath := filepath.Join(tmpDir, ".daneel", "cache", "vault_index.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open cached db: %v", err)
	}
	defer db.Close() //nolint:errcheck

	var count int
	var sumLinks int
	row := db.QueryRow("SELECT count(*), coalesce(sum(link_count), 0) FROM file_watermarks")
	if err := row.Scan(&count, &sumLinks); err != nil {
		t.Fatalf("failed to query file_watermarks: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 rows in file_watermarks, got %d", count)
	}
	if sumLinks != 1 {
		t.Errorf("expected sum(link_count) == 1, got %d", sumLinks)
	}

	// 2. Warm cache hit
	notes2, err := vault.ScanVaultIncremental(tmpDir, []string{"notes"}, false)
	if err != nil {
		t.Fatalf("warm ScanVaultIncremental failed: %v", err)
	}
	if len(notes2) != 2 {
		t.Fatalf("expected 2 notes on warm scan, got %d", len(notes2))
	}

	var designNote *vault.Note
	for _, n := range notes2 {
		if n.RelPath == "notes/arch/design.md" {
			designNote = n
			break
		}
	}
	if designNote == nil {
		t.Fatalf("notes/arch/design.md not found in warm scan results")
	}

	if designNote.Title != "Design" {
		t.Errorf("expected Title == 'Design', got %q", designNote.Title)
	}
	if designNote.NoteType != "architecture" {
		t.Errorf("expected NoteType == 'architecture', got %q", designNote.NoteType)
	}
	if designNote.Description != "Core design" {
		t.Errorf("expected Description == 'Core design', got %q", designNote.Description)
	}
	if len(designNote.Links) != 1 {
		t.Errorf("expected 1 link, got %d", len(designNote.Links))
	}
	if designNote.Content != "" {
		t.Errorf("expected Content == '' on warm cache hit, got %q", designNote.Content)
	}

	// 3. Modifying notes/arch/design.md
	modifiedContent := `---
type: architecture
description: Core design
---
# Design

See [Target](target.md) and [Other](other.md)
`
	if err := os.WriteFile(designPath, []byte(modifiedContent), 0600); err != nil {
		t.Fatalf("failed to update design.md: %v", err)
	}
	newMTime := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(designPath, newMTime, newMTime); err != nil {
		t.Fatalf("failed to update mtime of design.md: %v", err)
	}

	notes3, err := vault.ScanVaultIncremental(tmpDir, []string{"notes"}, false)
	if err != nil {
		t.Fatalf("modified ScanVaultIncremental failed: %v", err)
	}
	var modifiedDesign *vault.Note
	for _, n := range notes3 {
		if n.RelPath == "notes/arch/design.md" {
			modifiedDesign = n
			break
		}
	}
	if modifiedDesign == nil {
		t.Fatalf("modified notes/arch/design.md not found")
	}
	if len(modifiedDesign.Links) != 2 {
		t.Errorf("expected 2 links after modification, got %d", len(modifiedDesign.Links))
	}
	if modifiedDesign.Content == "" {
		t.Errorf("expected Content != '' for re-parsed modified file, got empty")
	}

	// 4. Force scan
	notes4, err := vault.ScanVaultIncremental(tmpDir, []string{"notes"}, true)
	if err != nil {
		t.Fatalf("force ScanVaultIncremental failed: %v", err)
	}
	if len(notes4) != 2 {
		t.Fatalf("expected 2 notes on force scan, got %d", len(notes4))
	}
	for _, n := range notes4 {
		if n.Content == "" {
			t.Errorf("expected note %s to have non-empty Content on force scan", n.RelPath)
		}
	}

	// 5. Delete notes/arch/target.md and verify stale row deletion
	if err := os.Remove(targetPath); err != nil {
		t.Fatalf("failed to remove target.md: %v", err)
	}

	notes5, err := vault.ScanVaultIncremental(tmpDir, []string{"notes"}, false)
	if err != nil {
		t.Fatalf("post-delete ScanVaultIncremental failed: %v", err)
	}
	if len(notes5) != 1 {
		t.Fatalf("expected 1 note after deletion, got %d", len(notes5))
	}

	var postDeleteCount int
	row = db.QueryRow("SELECT count(*) FROM file_watermarks")
	if err := row.Scan(&postDeleteCount); err != nil {
		t.Fatalf("failed to query file_watermarks post-delete: %v", err)
	}
	if postDeleteCount != 1 {
		t.Errorf("expected 1 row in file_watermarks after target.md deleted, got %d", postDeleteCount)
	}
}

func BenchmarkScanVaultIncremental(b *testing.B) {
	tmpDir := b.TempDir()
	notesDir := filepath.Join(tmpDir, "notes")
	if err := os.MkdirAll(notesDir, 0750); err != nil {
		b.Fatalf("failed to create notes dir: %v", err)
	}

	for i := 0; i < 100; i++ {
		filePath := filepath.Join(notesDir, fmt.Sprintf("note_%03d.md", i))
		content := fmt.Sprintf(`---
type: concept
description: Note number %d
---
# Note %d

Reference to [Next](note_%03d.md)
`, i, i, (i+1)%100)
		if err := os.WriteFile(filePath, []byte(content), 0600); err != nil {
			b.Fatalf("failed to write synthetic note %d: %v", i, err)
		}
	}

	// Prime the cache
	_, err := vault.ScanVaultIncremental(tmpDir, []string{"notes"}, false)
	if err != nil {
		b.Fatalf("failed to prime incremental cache: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		notes, err := vault.ScanVaultIncremental(tmpDir, []string{"notes"}, false)
		if err != nil {
			b.Fatalf("benchmark iteration failed: %v", err)
		}
		if len(notes) != 100 {
			b.Fatalf("expected 100 notes, got %d", len(notes))
		}
	}
}
