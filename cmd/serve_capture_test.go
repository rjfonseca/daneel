package cmd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"connectrpc.com/connect"
	"gopkg.in/yaml.v3"

	graphv1 "github.com/rjfonseca/daneel/api/v1"
	"github.com/rjfonseca/daneel/internal/config"
	"github.com/rjfonseca/daneel/internal/search"
)

func captureRequest(title, content, source string) *connect.Request[graphv1.CaptureNoteRequest] {
	return connect.NewRequest(&graphv1.CaptureNoteRequest{Title: title, ContentMarkdown: content, SourceUrl: source})
}

func TestCaptureNoteSavesAndIndexesLiteralContent(t *testing.T) {
	vault := t.TempDir()
	engine, err := search.Open(filepath.Join(vault, ".daneel", "cache", "search.db"), vault)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := engine.Close(); err != nil {
			t.Errorf("close search engine: %v", err)
		}
	})
	server := &GraphServer{vaultDir: vault, engine: engine}
	content := "A literal body\n---\n# Heading\n"
	res, err := server.CaptureNote(context.Background(), captureRequest("A: \"quoted\" idea", content, "https://example.com/a?b=1"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(res.Msg.RelPath, "inbox/") || !strings.HasSuffix(res.Msg.RelPath, ".md") {
		t.Fatalf("unexpected path: %q", res.Msg.RelPath)
	}
	if res.Msg.Warning != "" {
		t.Fatalf("unexpected warning: %q", res.Msg.Warning)
	}
	data, err := os.ReadFile(filepath.Join(vault, res.Msg.RelPath))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "\n"+content) {
		t.Fatalf("body changed: %q", data)
	}
	parts := strings.SplitN(string(data), "---\n", 3)
	if len(parts) != 3 {
		t.Fatalf("missing frontmatter: %q", data)
	}
	var meta map[string]any
	if err := yaml.Unmarshal([]byte(parts[1]), &meta); err != nil {
		t.Fatal(err)
	}
	if meta["title"] != "A: \"quoted\" idea" || meta["source_url"] != "https://example.com/a?b=1" {
		t.Fatalf("metadata: %#v", meta)
	}
	var indexedTitle string
	if err := engine.DB().QueryRow("SELECT title FROM notes_meta WHERE rel_path = ?", res.Msg.RelPath).Scan(&indexedTitle); err != nil {
		t.Fatal(err)
	}
	if indexedTitle != "A: \"quoted\" idea" {
		t.Fatalf("indexed title: %q", indexedTitle)
	}
}

func TestCaptureNoteRejectsInvalidInputWithoutWriting(t *testing.T) {
	cases := []struct{ name, title, content, source string }{
		{"blank title", " \n ", "body", ""},
		{"long title", strings.Repeat("a", 201), "body", ""},
		{"title control", "bad\x00title", "body", ""},
		{"blank content", "Valid", " \n ", ""},
		{"large content", "Valid", strings.Repeat("a", 1<<20+1), ""},
		{"file URL", "Valid", "body", "file:///etc/passwd"},
		{"hostless URL", "Valid", "body", "https:///missing"},
		{"source control", "Valid", "body", "https://example.com/\n"},
		{"long source", "Valid", "body", "https://example.com/" + strings.Repeat("a", 2040)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			vault := t.TempDir()
			server := &GraphServer{vaultDir: vault}
			_, err := server.CaptureNote(context.Background(), captureRequest(tc.title, tc.content, tc.source))
			if connect.CodeOf(err) != connect.CodeInvalidArgument {
				t.Fatalf("expected invalid argument, got %v", err)
			}
			if _, err := os.Lstat(filepath.Join(vault, "inbox")); !os.IsNotExist(err) {
				t.Fatalf("inbox created for rejected input: %v", err)
			}
		})
	}
}

func TestCaptureNoteRejectsSymlinkInbox(t *testing.T) {
	for _, inside := range []bool{false, true} {
		t.Run(map[bool]string{false: "outside", true: "inside"}[inside], func(t *testing.T) {
			vault := t.TempDir()
			target := t.TempDir()
			if inside {
				target = filepath.Join(vault, "target")
				if err := os.Mkdir(target, 0700); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Symlink(target, filepath.Join(vault, "inbox")); err != nil {
				t.Fatal(err)
			}
			server := &GraphServer{vaultDir: vault}
			if _, err := server.CaptureNote(context.Background(), captureRequest("Idea", "body", "")); err == nil {
				t.Fatal("accepted symlink inbox")
			}
			files, err := os.ReadDir(target)
			if err != nil {
				t.Fatal(err)
			}
			if len(files) != 0 {
				t.Fatalf("wrote through symlink: %v", files)
			}
		})
	}
}

func TestCaptureNoteParallelWritesNeverOverwrite(t *testing.T) {
	vault := t.TempDir()
	if err := os.Mkdir(filepath.Join(vault, "inbox"), 0700); err != nil {
		t.Fatal(err)
	}
	prior := filepath.Join(vault, "inbox", "existing.md")
	if err := os.WriteFile(prior, []byte("existing"), 0600); err != nil {
		t.Fatal(err)
	}
	server := &GraphServer{vaultDir: vault}
	const count = 20
	paths := make(chan string, count)
	errs := make(chan error, count)
	var wg sync.WaitGroup
	for range count {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := server.CaptureNote(context.Background(), captureRequest("Same title", "body", ""))
			if err != nil {
				errs <- err
				return
			}
			paths <- res.Msg.RelPath
		}()
	}
	wg.Wait()
	close(paths)
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	seen := map[string]bool{}
	for path := range paths {
		if seen[path] {
			t.Errorf("duplicate path: %s", path)
		}
		seen[path] = true
	}
	if len(seen) != count {
		t.Fatalf("saved %d of %d", len(seen), count)
	}
	data, err := os.ReadFile(prior)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "existing" {
		t.Fatalf("overwrote existing note: %q", data)
	}
}

func TestCaptureNoteReturnsSavedPathWhenIndexingFails(t *testing.T) {
	vault := t.TempDir()
	engine, err := search.Open(filepath.Join(vault, "search.db"), vault)
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.Close(); err != nil {
		t.Fatal(err)
	}
	server := &GraphServer{vaultDir: vault, engine: engine}
	res, err := server.CaptureNote(context.Background(), captureRequest("Idea", "body", ""))
	if err != nil {
		t.Fatal(err)
	}
	if res.Msg.RelPath == "" || res.Msg.Warning == "" {
		t.Fatalf("expected saved path and warning: %+v", res.Msg)
	}
	if _, err := os.Stat(filepath.Join(vault, res.Msg.RelPath)); err != nil {
		t.Fatal(err)
	}
}

func TestCaptureNoteHTTPBoundary(t *testing.T) {
	vault := t.TempDir()
	server, err := NewGalaxyHTTPServer(&config.Config{VaultPath: vault}, nil, 4321, nil)
	if err != nil {
		t.Fatal(err)
	}
	request := func(host, origin string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "http://"+host+"/graph.v1.GraphService/CaptureNote", strings.NewReader(`{"title":"Idea","contentMarkdown":"Body"}`))
		req.Header.Set("Content-Type", "application/json")
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		response := httptest.NewRecorder()
		server.Handler.ServeHTTP(response, req)
		return response
	}
	for _, response := range []*httptest.ResponseRecorder{
		request("localhost:4321", "https://example.com"),
		request("example.com:4321", "http://localhost:4321"),
	} {
		if response.Code != http.StatusForbidden {
			t.Fatalf("unsafe capture request returned %d: %s", response.Code, response.Body.String())
		}
	}
	if _, err := os.Lstat(filepath.Join(vault, "inbox")); !os.IsNotExist(err) {
		t.Fatalf("blocked request wrote to inbox: %v", err)
	}
	response := request("localhost:4321", "http://localhost:5173")
	if response.Code != http.StatusOK {
		t.Fatalf("local capture returned %d: %s", response.Code, response.Body.String())
	}
	var saved struct {
		RelPath string `json:"relPath"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(saved.RelPath, "inbox/") {
		t.Fatalf("unexpected path: %q in %s", saved.RelPath, response.Body.String())
	}
}
