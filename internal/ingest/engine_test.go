package ingest_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	graphv1 "github.com/rjfonseca/daneel/api/v1"
	"github.com/rjfonseca/daneel/internal/ingest"
	"github.com/rjfonseca/daneel/internal/search"
)

type eventRecorder struct {
	mu     sync.Mutex
	events []*graphv1.UIEvent
}

func (r *eventRecorder) PublishUIEvent(evt *graphv1.UIEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, evt)
}

func (r *eventRecorder) Events() []*graphv1.UIEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	copied := make([]*graphv1.UIEvent, len(r.events))
	copy(copied, r.events)
	return copied
}

func TestIngestSingleFile_WithFrontmatter(t *testing.T) {
	vault := t.TempDir()
	srcDir := t.TempDir()

	srcFile := filepath.Join(srcDir, "doc-with-fm.md")
	srcContent := `---
title: "Existing Title"
type: concept
custom_key: 42
tags: [architecture, design]
---
# Existing Title

Body content explaining the architecture.
`
	if err := os.WriteFile(srcFile, []byte(srcContent), 0600); err != nil {
		t.Fatal(err)
	}

	engine := ingest.NewEngine(vault, nil, nil)
	opts := ingest.IngestOptions{
		Destination: "inbox",
		Tags:        []string{"reviewed", "architecture"}, // duplicate tag should be de-duped
		SourceURL:   "https://example.com/spec",
	}

	report, err := engine.IngestPath(context.Background(), srcFile, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.TotalProcessed != 1 {
		t.Fatalf("expected 1 processed, got %d", report.TotalProcessed)
	}
	if len(report.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(report.Items))
	}

	item := report.Items[0]
	if item.Title != "Existing Title" {
		t.Errorf("expected title 'Existing Title', got %q", item.Title)
	}
	if item.RelPath != filepath.ToSlash(filepath.Join("inbox", "doc-with-fm.md")) {
		t.Errorf("expected relPath 'inbox/doc-with-fm.md', got %q", item.RelPath)
	}

	// Verify file content on disk
	destPath := filepath.Join(vault, item.RelPath)
	data, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("failed to read dest file: %v", err)
	}

	// Verify frontmatter was preserved and augmented
	parts := strings.SplitN(string(data), "---", 3)
	if len(parts) < 3 {
		t.Fatalf("expected frontmatter fences in %q", string(data))
	}

	var fm map[string]any
	if err := yaml.Unmarshal([]byte(parts[1]), &fm); err != nil {
		t.Fatalf("failed to parse yaml frontmatter: %v", err)
	}

	if fm["title"] != "Existing Title" {
		t.Errorf("expected title 'Existing Title', got %v", fm["title"])
	}
	if fm["type"] != "concept" {
		t.Errorf("expected type 'concept', got %v", fm["type"])
	}
	if fm["custom_key"] != 42 {
		t.Errorf("expected custom_key 42, got %v", fm["custom_key"])
	}
	if fm["source_url"] != "https://example.com/spec" {
		t.Errorf("expected source_url 'https://example.com/spec', got %v", fm["source_url"])
	}
	if fm["ingested_at"] == nil || fm["ingested_at"] == "" {
		t.Errorf("expected ingested_at timestamp to be set")
	}

	// Check body preserved
	if !strings.Contains(parts[2], "Body content explaining the architecture.") {
		t.Errorf("expected body to be preserved, got %q", parts[2])
	}
}

func TestIngestSingleFile_WithoutFrontmatter(t *testing.T) {
	vault := t.TempDir()
	srcDir := t.TempDir()

	srcFile := filepath.Join(srcDir, "plain-doc.md")
	srcContent := `# Synthesized Document Title

This is markdown text without any existing frontmatter block.
`
	if err := os.WriteFile(srcFile, []byte(srcContent), 0600); err != nil {
		t.Fatal(err)
	}

	engine := ingest.NewEngine(vault, nil, nil)
	opts := ingest.IngestOptions{
		Destination: "inbox",
		SourceURL:   "https://example.com/rfc",
		Tags:        []string{"rfc"},
	}

	report, err := engine.IngestPath(context.Background(), srcFile, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.TotalProcessed != 1 {
		t.Fatalf("expected 1 processed, got %d", report.TotalProcessed)
	}
	item := report.Items[0]
	if item.Title != "Synthesized Document Title" {
		t.Errorf("expected title 'Synthesized Document Title', got %q", item.Title)
	}

	destPath := filepath.Join(vault, item.RelPath)
	data, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("failed to read dest file: %v", err)
	}

	parts := strings.SplitN(string(data), "---", 3)
	if len(parts) < 3 {
		t.Fatalf("expected synthesized frontmatter fences in %q", string(data))
	}

	var fm map[string]any
	if err := yaml.Unmarshal([]byte(parts[1]), &fm); err != nil {
		t.Fatalf("failed to parse yaml frontmatter: %v", err)
	}

	if fm["title"] != "Synthesized Document Title" {
		t.Errorf("expected synthesized title, got %v", fm["title"])
	}
	if fm["source_url"] != "https://example.com/rfc" {
		t.Errorf("expected source_url, got %v", fm["source_url"])
	}
	if fm["ingested_at"] == nil || fm["ingested_at"] == "" {
		t.Errorf("expected ingested_at to be set")
	}
	if fm["source_path"] != srcFile {
		t.Errorf("expected source_path %q, got %v", srcFile, fm["source_path"])
	}

	if !strings.Contains(parts[2], "This is markdown text without any existing frontmatter block.") {
		t.Errorf("expected body to be preserved, got %q", parts[2])
	}
}

func TestIngestSingleFile_CollisionHandling(t *testing.T) {
	vault := t.TempDir()
	srcDir := t.TempDir()

	srcFile1 := filepath.Join(srcDir, "collision.md")
	if err := os.WriteFile(srcFile1, []byte("# Original Content\nVersion 1"), 0600); err != nil {
		t.Fatal(err)
	}

	engine := ingest.NewEngine(vault, nil, nil)
	opts := ingest.IngestOptions{Destination: "inbox"}

	// First ingestion
	report1, err := engine.IngestPath(context.Background(), srcFile1, opts)
	if err != nil {
		t.Fatalf("first ingestion failed: %v", err)
	}
	if len(report1.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(report1.Items))
	}
	path1 := report1.Items[0].RelPath
	if path1 != filepath.ToSlash(filepath.Join("inbox", "collision.md")) {
		t.Errorf("expected path inbox/collision.md, got %q", path1)
	}

	// Second ingestion with same filename but different content
	srcDir2 := t.TempDir()
	srcFile2 := filepath.Join(srcDir2, "collision.md")
	if err := os.WriteFile(srcFile2, []byte("# Changed Content\nVersion 2"), 0600); err != nil {
		t.Fatal(err)
	}

	report2, err := engine.IngestPath(context.Background(), srcFile2, opts)
	if err != nil {
		t.Fatalf("second ingestion failed: %v", err)
	}
	if len(report2.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(report2.Items))
	}
	path2 := report2.Items[0].RelPath

	// The second path must NOT overwrite path1
	if path2 == path1 {
		t.Fatalf("collision was not resolved! Both paths are %q", path1)
	}
	if report2.Items[0].Warning == "" {
		t.Errorf("expected collision warning on second ingestion")
	}

	// Verify original file at path1 is intact
	data1, err := os.ReadFile(filepath.Join(vault, path1))
	if err != nil {
		t.Fatalf("failed to read original file: %v", err)
	}
	if !strings.Contains(string(data1), "Version 1") {
		t.Fatalf("original file was overwritten! Content: %q", string(data1))
	}

	// Verify second file at path2 has Version 2
	data2, err := os.ReadFile(filepath.Join(vault, path2))
	if err != nil {
		t.Fatalf("failed to read second file: %v", err)
	}
	if !strings.Contains(string(data2), "Version 2") {
		t.Fatalf("second file missing content: %q", string(data2))
	}
}

func TestIngestDirectory_Recursive(t *testing.T) {
	vault := t.TempDir()
	srcDir := t.TempDir()

	// Create directory hierarchy
	// srcDir/
	//   note1.md
	//   ignored.png
	//   .hidden/
	//     secret.md
	//   sub/
	//     note2.md
	//     deep/
	//       note3.md
	if err := os.WriteFile(filepath.Join(srcDir, "note1.md"), []byte("# Note 1"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "ignored.png"), []byte("image bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(srcDir, ".hidden"), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, ".hidden", "secret.md"), []byte("# Secret"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(srcDir, "sub", "deep"), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "sub", "note2.md"), []byte("# Note 2"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "sub", "deep", "note3.md"), []byte("# Note 3"), 0600); err != nil {
		t.Fatal(err)
	}

	engine := ingest.NewEngine(vault, nil, nil)
	opts := ingest.IngestOptions{
		Destination: "notes/imported",
		Recursive:   true,
	}

	report, err := engine.IngestPath(context.Background(), srcDir, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.TotalProcessed != 3 {
		t.Fatalf("expected 3 processed notes, got %d", report.TotalProcessed)
	}

	expectedPaths := map[string]bool{
		filepath.ToSlash(filepath.Join("notes", "imported", "note1.md")):                false,
		filepath.ToSlash(filepath.Join("notes", "imported", "sub", "note2.md")):         false,
		filepath.ToSlash(filepath.Join("notes", "imported", "sub", "deep", "note3.md")): false,
	}

	for _, item := range report.Items {
		clean := filepath.ToSlash(item.RelPath)
		for expected := range expectedPaths {
			if clean == expected {
				expectedPaths[expected] = true
			}
		}
	}

	for p, found := range expectedPaths {
		if !found {
			t.Errorf("expected path %q not found in ingested items", p)
		}
		if _, err := os.Stat(filepath.Join(vault, p)); err != nil {
			t.Errorf("expected file %q to exist on disk: %v", p, err)
		}
	}

	// Verify ignored.png and secret.md were not ingested
	if _, err := os.Stat(filepath.Join(vault, "notes/imported", "ignored.png")); !os.IsNotExist(err) {
		t.Errorf("ignored.png should not have been ingested")
	}
	if _, err := os.Stat(filepath.Join(vault, "notes/imported", ".hidden", "secret.md")); !os.IsNotExist(err) {
		t.Errorf(".hidden/secret.md should not have been ingested")
	}
}

func TestIngestDirectory_NonRecursive(t *testing.T) {
	vault := t.TempDir()
	srcDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(srcDir, "root.md"), []byte("# Root"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(srcDir, "sub"), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "sub", "nested.md"), []byte("# Nested"), 0600); err != nil {
		t.Fatal(err)
	}

	engine := ingest.NewEngine(vault, nil, nil)
	opts := ingest.IngestOptions{
		Destination: "inbox",
		Recursive:   false,
	}

	report, err := engine.IngestPath(context.Background(), srcDir, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.TotalProcessed != 1 {
		t.Fatalf("expected only 1 processed note (non-recursive), got %d", report.TotalProcessed)
	}
	if report.Items[0].RelPath != filepath.ToSlash(filepath.Join("inbox", "root.md")) {
		t.Errorf("expected inbox/root.md, got %q", report.Items[0].RelPath)
	}
}

func TestIngest_PathSafetyAndSymlinks(t *testing.T) {
	vault := t.TempDir()
	srcDir := t.TempDir()
	outsideDir := t.TempDir()

	srcFile := filepath.Join(srcDir, "safe.md")
	if err := os.WriteFile(srcFile, []byte("# Safe Note"), 0600); err != nil {
		t.Fatal(err)
	}

	outsideFile := filepath.Join(outsideDir, "secret.md")
	if err := os.WriteFile(outsideFile, []byte("# Secret Note"), 0600); err != nil {
		t.Fatal(err)
	}

	engine := ingest.NewEngine(vault, nil, nil)

	// 1. Destination traversal attempt with "../"
	optsTraversal := ingest.IngestOptions{Destination: "../../etc"}
	_, err := engine.IngestPath(context.Background(), srcFile, optsTraversal)
	if err == nil {
		t.Fatalf("expected error on path traversal destination, got nil")
	}

	// 2. Absolute destination outside vault
	optsAbs := ingest.IngestOptions{Destination: outsideDir}
	_, err = engine.IngestPath(context.Background(), srcFile, optsAbs)
	if err == nil {
		t.Fatalf("expected error on absolute destination outside vault, got nil")
	}

	// 3. Symlink inside source directory pointing outside source directory
	symlinkPath := filepath.Join(srcDir, "escaping-symlink.md")
	if err := os.Symlink(outsideFile, symlinkPath); err != nil {
		t.Fatal(err)
	}

	// Ingesting standalone escaping symlink must be rejected
	_, err = engine.IngestPath(context.Background(), symlinkPath, ingest.IngestOptions{Destination: "inbox"})
	if err == nil {
		t.Fatalf("expected error on escaping symlink ingestion, got nil")
	}

	// Ingesting directory containing escaping symlink must reject or record error
	report, err := engine.IngestDirectory(context.Background(), srcDir, ingest.IngestOptions{Destination: "inbox", Recursive: true})
	if err != nil {
		// Returning an error is acceptable
	} else {
		// Or reporting it in report.Errors is acceptable, without ingesting the escaped content
		if len(report.Errors) == 0 {
			t.Fatalf("expected error recorded for escaping symlink in directory")
		}
		for _, item := range report.Items {
			if strings.Contains(item.RelPath, "escaping-symlink") {
				t.Fatalf("escaping symlink should not have been ingested: %q", item.RelPath)
			}
		}
	}
}

func TestIngest_MoveFlag(t *testing.T) {
	vault := t.TempDir()
	srcDir := t.TempDir()

	srcFile := filepath.Join(srcDir, "to-move.md")
	if err := os.WriteFile(srcFile, []byte("# Move Me\nWill be removed."), 0600); err != nil {
		t.Fatal(err)
	}

	engine := ingest.NewEngine(vault, nil, nil)
	opts := ingest.IngestOptions{
		Destination: "inbox",
		Move:        true,
	}

	report, err := engine.IngestPath(context.Background(), srcFile, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.TotalProcessed != 1 {
		t.Fatalf("expected 1 processed, got %d", report.TotalProcessed)
	}

	// Check file exists in vault
	destPath := filepath.Join(vault, report.Items[0].RelPath)
	if _, err := os.Stat(destPath); err != nil {
		t.Fatalf("destination file missing: %v", err)
	}

	// Check source file was removed
	if _, err := os.Stat(srcFile); !os.IsNotExist(err) {
		t.Fatalf("expected source file to be removed when Move: true, but it exists: %v", err)
	}
}

func TestIngestContent(t *testing.T) {
	vault := t.TempDir()
	engine := ingest.NewEngine(vault, nil, nil)

	opts := ingest.IngestOptions{
		Destination: "inbox",
		SourceURL:   "https://example.com/captured",
		Tags:        []string{"quick", "idea"},
	}

	// First note
	note1, err := engine.IngestContent(context.Background(), "Project Architecture", []byte("Initial design draft."), opts)
	if err != nil {
		t.Fatalf("failed to ingest content: %v", err)
	}

	if note1.Title != "Project Architecture" {
		t.Errorf("expected title 'Project Architecture', got %q", note1.Title)
	}
	if note1.RelPath != filepath.ToSlash(filepath.Join("inbox", "project-architecture.md")) {
		t.Errorf("expected relPath 'inbox/project-architecture.md', got %q", note1.RelPath)
	}

	// Second note with same title (collision test)
	note2, err := engine.IngestContent(context.Background(), "Project Architecture", []byte("Second draft."), opts)
	if err != nil {
		t.Fatalf("failed to ingest content collision: %v", err)
	}

	if note2.RelPath == note1.RelPath {
		t.Fatalf("expected different path for colliding title, both are %q", note1.RelPath)
	}
	if note2.Warning == "" {
		t.Errorf("expected collision warning on second note")
	}

	// Verify disk contents
	data1, err := os.ReadFile(filepath.Join(vault, note1.RelPath))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data1), "Initial design draft.") {
		t.Errorf("note 1 content corrupted: %q", string(data1))
	}

	data2, err := os.ReadFile(filepath.Join(vault, note2.RelPath))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data2), "Second draft.") {
		t.Errorf("note 2 content corrupted: %q", string(data2))
	}
}

func TestIngestContent_Validation(t *testing.T) {
	vault := t.TempDir()
	engine := ingest.NewEngine(vault, nil, nil)

	// Blank title and content
	_, err := engine.IngestContent(context.Background(), "   ", []byte("   "), ingest.IngestOptions{})
	if err == nil {
		t.Fatalf("expected error on empty title and content")
	}

	// Destination traversal
	_, err = engine.IngestContent(context.Background(), "Test", []byte("Content"), ingest.IngestOptions{Destination: "../../../escape"})
	if err == nil {
		t.Fatalf("expected error on destination traversal")
	}
}

func TestIngestParallel_NoDataRaceOrOverwrite(t *testing.T) {
	vault := t.TempDir()
	engine := ingest.NewEngine(vault, nil, nil)

	const concurrency = 10
	errs := make(chan error, concurrency)
	paths := make(chan string, concurrency)

	for i := 0; i < concurrency; i++ {
		go func(idx int) {
			note, err := engine.IngestContent(
				context.Background(),
				"Concurrent Note",
				[]byte(strings.Repeat("data ", idx+1)),
				ingest.IngestOptions{Destination: "inbox"},
			)
			if err != nil {
				errs <- err
				return
			}
			paths <- note.RelPath
		}(i)
	}

	seenPaths := make(map[string]bool)
	for i := 0; i < concurrency; i++ {
		select {
		case err := <-errs:
			t.Fatalf("concurrent ingestion failed: %v", err)
		case p := <-paths:
			if seenPaths[p] {
				t.Fatalf("path collision detected in concurrent ingestion: %s", p)
			}
			seenPaths[p] = true
			if _, err := os.Stat(filepath.Join(vault, p)); err != nil {
				t.Fatalf("ingested file missing on disk: %s", p)
			}
		}
	}
}

func TestIndexingAndEvents(t *testing.T) {
	vault := t.TempDir()
	dbDir := t.TempDir()
	dbPath := filepath.Join(dbDir, "cache.db")

	searchEngine, err := search.Open(dbPath, vault)
	if err != nil {
		t.Fatalf("failed to open search engine: %v", err)
	}
	defer searchEngine.Close() //nolint:errcheck

	hub := &eventRecorder{}
	engine := ingest.NewEngine(vault, searchEngine, hub)

	ctx := context.Background()
	beforeTime := time.Now().UnixMilli() - 100

	// 1. IngestPath (single file)
	srcDir := t.TempDir()
	srcFile := filepath.Join(srcDir, "indexer-test.md")
	srcContent := `# Quantum Telemetry Notes
Sensors captured atomic photon telemetry and gravitational fluctuations.
`
	if err := os.WriteFile(srcFile, []byte(srcContent), 0600); err != nil {
		t.Fatal(err)
	}

	report, err := engine.IngestPath(ctx, srcFile, ingest.IngestOptions{
		Destination: "notes/physics",
		Tags:        []string{"quantum", "telemetry"},
	})
	if err != nil {
		t.Fatalf("IngestPath failed: %v", err)
	}
	if report.TotalProcessed != 1 {
		t.Fatalf("expected 1 processed note, got %d", report.TotalProcessed)
	}

	// 2. IngestContent
	contentTitle := "Cosmic Ray Observation"
	contentBody := "Sensors detected high-energy muon cascades across galactic core."
	note, err := engine.IngestContent(ctx, contentTitle, []byte(contentBody), ingest.IngestOptions{
		Destination: "notes/astrophysics",
		Tags:        []string{"cosmic", "muons"},
	})
	if err != nil {
		t.Fatalf("IngestContent failed: %v", err)
	}

	afterTime := time.Now().UnixMilli() + 100

	// Verify Search returns both notes
	results1, err := searchEngine.Search(ctx, "quantum", "all", 10)
	if err != nil {
		t.Fatalf("search for 'quantum' failed: %v", err)
	}
	if len(results1) == 0 {
		t.Fatalf("expected search result for 'quantum', got 0")
	}
	found1 := false
	for _, res := range results1 {
		if strings.Contains(res.RelPath, "indexer-test.md") {
			found1 = true
			if res.Title != "Quantum Telemetry Notes" {
				t.Errorf("expected title 'Quantum Telemetry Notes', got %q", res.Title)
			}
			break
		}
	}
	if !found1 {
		t.Errorf("note 'indexer-test.md' not found in search results: %+v", results1)
	}

	results2, err := searchEngine.Search(ctx, "muon", "all", 10)
	if err != nil {
		t.Fatalf("search for 'muon' failed: %v", err)
	}
	if len(results2) == 0 {
		t.Fatalf("expected search result for 'muon', got 0")
	}
	found2 := false
	for _, res := range results2 {
		if res.RelPath == note.RelPath {
			found2 = true
			if res.Title != contentTitle {
				t.Errorf("expected title %q, got %q", contentTitle, res.Title)
			}
			break
		}
	}
	if !found2 {
		t.Errorf("note %q not found in search results: %+v", note.RelPath, results2)
	}

	// Verify EventHub received ACTION_GALAXY_RELOAD events with valid timestamps
	events := hub.Events()
	if len(events) != 2 {
		t.Fatalf("expected 2 UI events, got %d", len(events))
	}

	for i, evt := range events {
		if evt.Action != graphv1.UIActionType_ACTION_GALAXY_RELOAD {
			t.Errorf("event %d: expected ACTION_GALAXY_RELOAD (%v), got %v",
				i, graphv1.UIActionType_ACTION_GALAXY_RELOAD, evt.Action)
		}
		if evt.Timestamp <= 0 {
			t.Errorf("event %d: expected non-zero timestamp, got %d", i, evt.Timestamp)
		}
		if evt.Timestamp < beforeTime || evt.Timestamp > afterTime {
			t.Errorf("event %d: timestamp %d outside expected range [%d, %d]",
				i, evt.Timestamp, beforeTime, afterTime)
		}
	}
}

func TestIngestPath_SymlinkToDirectory(t *testing.T) {
	vault := t.TempDir()
	srcDir := t.TempDir()

	realDir := filepath.Join(srcDir, "real_docs")
	if err := os.MkdirAll(realDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(realDir, "note1.md"), []byte("# Note One\nContent 1"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(realDir, "note2.md"), []byte("# Note Two\nContent 2"), 0600); err != nil {
		t.Fatal(err)
	}

	symDir := filepath.Join(srcDir, "sym_docs")
	if err := os.Symlink(realDir, symDir); err != nil {
		t.Fatal(err)
	}

	engine := ingest.NewEngine(vault, nil, nil)
	report, err := engine.IngestPath(context.Background(), symDir, ingest.IngestOptions{
		Destination: "inbox/symlinked",
		Recursive:   true,
	})
	if err != nil {
		t.Fatalf("IngestPath failed on symlink to directory: %v", err)
	}

	if report.TotalProcessed != 2 {
		t.Fatalf("expected 2 processed notes, got %d", report.TotalProcessed)
	}

	for _, name := range []string{"note1.md", "note2.md"} {
		dest := filepath.Join(vault, "inbox", "symlinked", name)
		if _, err := os.Stat(dest); err != nil {
			t.Errorf("expected file %q to exist: %v", dest, err)
		}
	}
}

func TestEngine_SymlinkedVaultRoot_Regression(t *testing.T) {
	// Replicates macOS /var -> /private/var and symlinked vault environments.
	realVault := t.TempDir()
	linkParent := t.TempDir()
	symlinkVault := filepath.Join(linkParent, "symlinked_vault")
	if err := os.Symlink(realVault, symlinkVault); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}

	engine := ingest.NewEngine(symlinkVault, nil, nil)

	// Test 1: Single file ingestion into existing inbox
	srcDir := t.TempDir()
	src1 := filepath.Join(srcDir, "doc1.md")
	if err := os.WriteFile(src1, []byte("# Note 1\nContent"), 0600); err != nil {
		t.Fatal(err)
	}
	rep1, err := engine.IngestPath(context.Background(), src1, ingest.IngestOptions{Destination: "inbox"})
	if err != nil {
		t.Fatalf("first ingestion with symlinked vault failed: %v", err)
	}
	if len(rep1.Items) != 1 || rep1.Items[0].RelPath != "inbox/doc1.md" {
		t.Fatalf("unexpected result: %+v", rep1)
	}

	// Test 2: Second ingestion with collision into now-existing inbox
	src2 := filepath.Join(srcDir, "doc1_diff.md")
	if err := os.WriteFile(src2, []byte("# Note 1\nDifferent Content"), 0600); err != nil {
		t.Fatal(err)
	}
	// Name it doc1.md to trigger collision
	src2Col := filepath.Join(t.TempDir(), "doc1.md")
	if err := os.WriteFile(src2Col, []byte("# Note 1\nDifferent Content"), 0600); err != nil {
		t.Fatal(err)
	}
	rep2, err := engine.IngestPath(context.Background(), src2Col, ingest.IngestOptions{Destination: "inbox"})
	if err != nil {
		t.Fatalf("collision ingestion with symlinked vault failed: %v", err)
	}
	if len(rep2.Items) != 1 || rep2.Items[0].Warning == "" {
		t.Fatalf("expected collision resolution with warning, got: %+v", rep2)
	}

	// Test 3: Directory ingestion into nested folders
	dirSrc := t.TempDir()
	if err := os.WriteFile(filepath.Join(dirSrc, "n1.md"), []byte("# Nested 1"), 0600); err != nil {
		t.Fatal(err)
	}
	subDir := filepath.Join(dirSrc, "sub")
	if err := os.Mkdir(subDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "n2.md"), []byte("# Nested 2"), 0600); err != nil {
		t.Fatal(err)
	}

	repDir, err := engine.IngestPath(context.Background(), dirSrc, ingest.IngestOptions{
		Destination: "notes/nested",
		Recursive:   true,
	})
	if err != nil {
		t.Fatalf("directory ingestion with symlinked vault failed: %v", err)
	}
	if repDir.TotalProcessed != 2 {
		t.Fatalf("expected 2 processed notes, got %d (errors: %v)", repDir.TotalProcessed, repDir.Errors)
	}
}

func TestEngine_WindowsBackslashDestinations_CrossPlatform(t *testing.T) {
	vault := t.TempDir()
	engine := ingest.NewEngine(vault, nil, nil)

	srcDir := t.TempDir()
	src := filepath.Join(srcDir, "component.md")
	if err := os.WriteFile(src, []byte("# Component Spec\nArchitecture detail"), 0600); err != nil {
		t.Fatal(err)
	}

	// Destination with backslashes
	opts := ingest.IngestOptions{
		Destination: `notes\backend\services`,
	}
	rep, err := engine.IngestPath(context.Background(), src, opts)
	if err != nil {
		t.Fatalf("ingest with backslash destination failed: %v", err)
	}
	if len(rep.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(rep.Items))
	}
	expectedRel := "notes/backend/services/component.md"
	if rep.Items[0].RelPath != expectedRel {
		t.Errorf("expected forward slash relative path %q, got %q", expectedRel, rep.Items[0].RelPath)
	}

	// Verify file was physically written to disk
	expectedDisk := filepath.Join(vault, "notes", "backend", "services", "component.md")
	if _, err := os.Stat(expectedDisk); err != nil {
		t.Fatalf("expected disk file at %s: %v", expectedDisk, err)
	}
}
