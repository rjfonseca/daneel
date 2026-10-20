package cmd

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"

	"github.com/rjfonseca/daneel/internal/safefile"

	"connectrpc.com/connect"
)

// readVaultNote uses an OS-enforced root so symlink resolution cannot escape the vault.
func readVaultNote(vaultDir, relPath string) ([]byte, error) {
	if !filepath.IsLocal(relPath) || filepath.Clean(relPath) == "." {
		return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("note path must be local to the vault"))
	}
	content, err := safefile.ReadFile(vaultDir, relPath)
	if os.IsNotExist(err) {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("note not found"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("cannot read note within vault"))
	}
	return content, nil
}

// localRequests prevents foreign browser origins and DNS rebinding from reaching local RPCs.
func localRequests(port int, next http.Handler) http.Handler {
	hosts := []string{fmt.Sprintf("localhost:%d", port), fmt.Sprintf("127.0.0.1:%d", port)}
	origins := []string{"http://" + hosts[0], "http://" + hosts[1], "http://localhost:5173", "http://127.0.0.1:5173"}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if !slices.Contains(hosts, r.Host) || (origin != "" && !slices.Contains(origins, origin)) {
			http.Error(w, "Only local requests are permitted", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
