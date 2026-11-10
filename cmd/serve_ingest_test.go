package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"connectrpc.com/connect"

	graphv1 "github.com/rjfonseca/daneel/api/v1"
	"github.com/rjfonseca/daneel/internal/search"
)

func TestIngestPath_ValidSingleFile(t *testing.T) {
	vault := t.TempDir()
	dbPath := filepath.Join(vault, ".daneel", "cache", "search.db")
	engine, err := search.Open(dbPath, vault)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = engine.Close()
	})

	server := &GraphServer{vaultDir: vault, engine: engine}

	// Create a host file outside vault
	hostDir := t.TempDir()
	filePath := filepath.Join(hostDir, "standalone-note.md")
	content := "# My Important Note\n\nSome insightful content here."
	if err := os.WriteFile(filePath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	req := connect.NewRequest(&graphv1.IngestPathRequest{
		Path:        filePath,
		Destination: "inbox",
	})

	resp, err := server.IngestPath(context.Background(), req)
	if err != nil {
		t.Fatalf("IngestPath failed: %v", err)
	}

	if resp.Msg.TotalProcessed != 1 {
		t.Fatalf("expected 1 processed item, got %d", resp.Msg.TotalProcessed)
	}
	if len(resp.Msg.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(resp.Msg.Items))
	}

	item := resp.Msg.Items[0]
	if !strings.HasPrefix(item.RelPath, "inbox/") {
		t.Errorf("expected item.RelPath to start with inbox/, got %s", item.RelPath)
	}
	if item.Title != "My Important Note" {
		t.Errorf("expected title 'My Important Note', got %q", item.Title)
	}

	// Verify file exists in vault
	vaultNotePath := filepath.Join(vault, item.RelPath)
	if _, err := os.Stat(vaultNotePath); err != nil {
		t.Fatalf("vault note file missing: %v", err)
	}

	// Verify indexed in FTS5 engine
	var indexedTitle string
	if err := engine.DB().QueryRow("SELECT title FROM notes_meta WHERE rel_path = ?", item.RelPath).Scan(&indexedTitle); err != nil {
		t.Fatalf("note not indexed in search engine: %v", err)
	}
	if indexedTitle != "My Important Note" {
		t.Errorf("indexed title = %q, expected 'My Important Note'", indexedTitle)
	}
}

func TestIngestPath_Directory(t *testing.T) {
	vault := t.TempDir()
	server := &GraphServer{vaultDir: vault}

	// Create source directory structure
	hostDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(hostDir, "doc1.md"), []byte("# Doc One\nContent 1"), 0600); err != nil {
		t.Fatal(err)
	}
	subDir := filepath.Join(hostDir, "sub")
	if err := os.Mkdir(subDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "doc2.md"), []byte("# Doc Two\nContent 2"), 0600); err != nil {
		t.Fatal(err)
	}

	t.Run("recursive", func(t *testing.T) {
		req := connect.NewRequest(&graphv1.IngestPathRequest{
			Path:        hostDir,
			Destination: "sources/research",
			Recursive:   true,
		})
		resp, err := server.IngestPath(context.Background(), req)
		if err != nil {
			t.Fatalf("IngestPath directory failed: %v", err)
		}
		if resp.Msg.TotalProcessed != 2 {
			t.Fatalf("expected 2 items processed, got %d", resp.Msg.TotalProcessed)
		}
		for _, it := range resp.Msg.Items {
			if !strings.HasPrefix(it.RelPath, "sources/research") {
				t.Errorf("unexpected relPath: %s", it.RelPath)
			}
		}
	})

	t.Run("non-recursive", func(t *testing.T) {
		destDir := t.TempDir()
		vault2 := t.TempDir()
		server2 := &GraphServer{vaultDir: vault2}
		req := connect.NewRequest(&graphv1.IngestPathRequest{
			Path:        hostDir,
			Destination: "sources/flat",
			Recursive:   false,
		})
		resp, err := server2.IngestPath(context.Background(), req)
		if err != nil {
			t.Fatalf("IngestPath directory non-recursive failed: %v", err)
		}
		if resp.Msg.TotalProcessed != 1 {
			t.Fatalf("expected 1 item processed non-recursively, got %d", resp.Msg.TotalProcessed)
		}
		_ = destDir
	})
}

func TestIngestPath_MoveSource(t *testing.T) {
	vault := t.TempDir()
	server := &GraphServer{vaultDir: vault}

	hostDir := t.TempDir()
	sourceFile := filepath.Join(hostDir, "move-me.md")
	if err := os.WriteFile(sourceFile, []byte("# Moving Note\nBody"), 0600); err != nil {
		t.Fatal(err)
	}

	req := connect.NewRequest(&graphv1.IngestPathRequest{
		Path:        sourceFile,
		Destination: "inbox",
		Move:        true,
	})

	resp, err := server.IngestPath(context.Background(), req)
	if err != nil {
		t.Fatalf("IngestPath move failed: %v", err)
	}
	if resp.Msg.TotalProcessed != 1 {
		t.Fatalf("expected 1 processed, got %d", resp.Msg.TotalProcessed)
	}

	// Source file should no longer exist
	if _, err := os.Stat(sourceFile); !os.IsNotExist(err) {
		t.Fatalf("expected source file to be removed after move, got err: %v", err)
	}

	// Destination file must exist
	destFile := filepath.Join(vault, resp.Msg.Items[0].RelPath)
	if _, err := os.Stat(destFile); err != nil {
		t.Fatalf("destination note missing: %v", err)
	}
}

func TestIngestPath_InvalidInput(t *testing.T) {
	vault := t.TempDir()
	server := &GraphServer{vaultDir: vault}

	t.Run("nil request", func(t *testing.T) {
		_, err := server.IngestPath(context.Background(), nil)
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("expected InvalidArgument for nil request, got %v", err)
		}
	})

	t.Run("empty path", func(t *testing.T) {
		req := connect.NewRequest(&graphv1.IngestPathRequest{Path: "  "})
		_, err := server.IngestPath(context.Background(), req)
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("expected InvalidArgument for empty path, got %v", err)
		}
	})

	t.Run("nonexistent path", func(t *testing.T) {
		req := connect.NewRequest(&graphv1.IngestPathRequest{Path: filepath.Join(vault, "nonexistent.md")})
		_, err := server.IngestPath(context.Background(), req)
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Errorf("expected NotFound for nonexistent path, got %v", err)
		}
	})

	t.Run("destination escape", func(t *testing.T) {
		hostFile := filepath.Join(t.TempDir(), "note.md")
		if err := os.WriteFile(hostFile, []byte("# Note\nBody"), 0600); err != nil {
			t.Fatal(err)
		}
		req := connect.NewRequest(&graphv1.IngestPathRequest{
			Path:        hostFile,
			Destination: "../outside",
		})
		_, err := server.IngestPath(context.Background(), req)
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("expected InvalidArgument for destination escaping vault root, got %v", err)
		}
	})

	t.Run("canceled context", func(t *testing.T) {
		hostFile := filepath.Join(t.TempDir(), "note.md")
		if err := os.WriteFile(hostFile, []byte("# Note\nBody"), 0600); err != nil {
			t.Fatal(err)
		}
		req := connect.NewRequest(&graphv1.IngestPathRequest{
			Path:        hostFile,
			Destination: "inbox",
		})
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := server.IngestPath(ctx, req)
		if connect.CodeOf(err) != connect.CodeCanceled {
			t.Errorf("expected CodeCanceled, got %v", err)
		}
	})
}

func TestIngestPath_SymlinkEscape(t *testing.T) {
	vault := t.TempDir()
	server := &GraphServer{vaultDir: vault}

	outsideDir := t.TempDir()
	outsideSecret := filepath.Join(outsideDir, "secret.md")
	if err := os.WriteFile(outsideSecret, []byte("# Secret\nData"), 0600); err != nil {
		t.Fatal(err)
	}

	sourceDir := t.TempDir()
	symlinkPath := filepath.Join(sourceDir, "symlink-escape.md")
	if err := os.Symlink(outsideSecret, symlinkPath); err != nil {
		t.Fatal(err)
	}

	req := connect.NewRequest(&graphv1.IngestPathRequest{
		Path:        symlinkPath,
		Destination: "inbox",
	})
	_, err := server.IngestPath(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for escaping symlink, got nil")
	}
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("expected InvalidArgument for escaping symlink, got %v", err)
	}
}

func TestCaptureNote_CustomDestination(t *testing.T) {
	vault := t.TempDir()
	dbPath := filepath.Join(vault, ".daneel", "cache", "search.db")
	engine, err := search.Open(dbPath, vault)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = engine.Close()
	})

	server := &GraphServer{vaultDir: vault, engine: engine}

	req := connect.NewRequest(&graphv1.CaptureNoteRequest{
		Title:           "Custom Dest Note",
		ContentMarkdown: "Content in custom folder",
		Destination:     "notes/architecture",
	})

	resp, err := server.CaptureNote(context.Background(), req)
	if err != nil {
		t.Fatalf("CaptureNote failed: %v", err)
	}

	if !strings.HasPrefix(resp.Msg.RelPath, "notes/architecture/") {
		t.Fatalf("expected RelPath to start with notes/architecture/, got %q", resp.Msg.RelPath)
	}

	notePath := filepath.Join(vault, resp.Msg.RelPath)
	if _, err := os.Stat(notePath); err != nil {
		t.Fatalf("expected note file to exist at %s: %v", notePath, err)
	}

	// Verify indexing
	var indexedTitle string
	if err := engine.DB().QueryRow("SELECT title FROM notes_meta WHERE rel_path = ?", resp.Msg.RelPath).Scan(&indexedTitle); err != nil {
		t.Fatalf("note not indexed in search engine: %v", err)
	}
	if indexedTitle != "Custom Dest Note" {
		t.Errorf("indexed title = %q, expected 'Custom Dest Note'", indexedTitle)
	}
}

func TestCaptureNote_InvalidDestination(t *testing.T) {
	vault := t.TempDir()
	server := &GraphServer{vaultDir: vault}

	invalidDests := []string{
		"../escaping",
		"/absolute/path",
		"notes/../../escape",
		"bad\x00dir",
	}

	for _, dest := range invalidDests {
		t.Run(dest, func(t *testing.T) {
			req := connect.NewRequest(&graphv1.CaptureNoteRequest{
				Title:           "Valid Title",
				ContentMarkdown: "Valid content",
				Destination:     dest,
			})
			_, err := server.CaptureNote(context.Background(), req)
			if connect.CodeOf(err) != connect.CodeInvalidArgument {
				t.Fatalf("expected InvalidArgument for destination %q, got %v", dest, err)
			}
		})
	}
}
