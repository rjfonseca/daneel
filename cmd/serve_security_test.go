package cmd

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"

	graphv1 "github.com/rjfonseca/daneel/api/v1"
	"github.com/rjfonseca/daneel/internal/config"
)

func TestGetNoteVaultBoundary(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "vault")
	if err := os.MkdirAll(filepath.Join(root, "notes"), 0700); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(parent, "private.md")
	if err := os.WriteFile(outside, []byte("private"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "notes", "hello.md"), []byte("hello"), 0600); err != nil {
		t.Fatal(err)
	}
	server := &GraphServer{vaultDir: root}
	for _, path := range []string{"../private.md", "notes/../../private.md", outside, "", "."} {
		t.Run(path, func(t *testing.T) {
			_, err := server.GetNote(context.Background(), connect.NewRequest(&graphv1.GetNoteRequest{RelPath: path}))
			if connect.CodeOf(err) != connect.CodePermissionDenied {
				t.Fatalf("expected permission denied for %q, got %v", path, err)
			}
		})
	}
	res, err := server.GetNote(context.Background(), connect.NewRequest(&graphv1.GetNoteRequest{RelPath: "notes/hello.md"}))
	if err != nil || res.Msg.ContentMarkdown != "hello" {
		t.Fatalf("valid note: %v, %v", res, err)
	}
	_, err = server.GetNote(context.Background(), connect.NewRequest(&graphv1.GetNoteRequest{RelPath: "notes/missing.md"}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("missing note: %v", err)
	}
	t.Run("escaping symlink", func(t *testing.T) {
		if err := os.Symlink(outside, filepath.Join(root, "notes", "link.md")); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		_, err := server.GetNote(context.Background(), connect.NewRequest(&graphv1.GetNoteRequest{RelPath: "notes/link.md"}))
		if connect.CodeOf(err) != connect.CodePermissionDenied {
			t.Fatalf("expected permission denied, got %v", err)
		}
	})
}

func TestGalaxyHTTPBoundary(t *testing.T) {
	server, err := NewGalaxyHTTPServer(&config.Config{VaultPath: t.TempDir()}, nil, 4321, nil)
	if err != nil {
		t.Fatal(err)
	}
	if server.Addr != "127.0.0.1:4321" {
		t.Errorf("unsafe listener: %s", server.Addr)
	}
	for _, tc := range []struct {
		name, host, origin string
		allowed            bool
	}{
		{"local navigation", "localhost:4321", "", true},
		{"local RPC", "127.0.0.1:4321", "http://127.0.0.1:4321", true},
		{"development", "localhost:4321", "http://localhost:5173", true},
		{"foreign origin", "localhost:4321", "https://example.com", false},
		{"null origin", "localhost:4321", "null", false},
		{"rebound host", "example.com:4321", "", false},
		{"foreign host local origin", "example.com:4321", "http://localhost:4321", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://"+tc.host+"/", nil)
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			rr := httptest.NewRecorder()
			server.Handler.ServeHTTP(rr, req)
			if !tc.allowed && rr.Code != http.StatusForbidden {
				t.Fatalf("expected forbidden, got %d", rr.Code)
			}
			if tc.allowed && rr.Code != http.StatusOK {
				t.Fatalf("expected success, got %d", rr.Code)
			}
			if tc.allowed && tc.origin != "" && rr.Header().Get("Access-Control-Allow-Origin") != tc.origin {
				t.Fatal("missing explicit CORS origin")
			}
		})
	}
}
