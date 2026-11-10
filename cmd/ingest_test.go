//nolint:revive // package comment not required
package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rjfonseca/daneel/internal/config"
	"github.com/rjfonseca/daneel/internal/ingest"
	"github.com/rjfonseca/daneel/internal/search"
)

func setupTestVault(t *testing.T) (string, string) {
	t.Helper()
	vault := t.TempDir()
	dbDir := filepath.Join(vault, ".daneel", "cache")
	if err := os.MkdirAll(dbDir, 0750); err != nil {
		t.Fatalf("failed to create db dir: %v", err)
	}
	dbPath := filepath.Join(dbDir, "vault_index.db")

	cfg = &config.Config{
		VaultPath: vault,
		DBPath:    dbPath,
		Vault: config.VaultConfig{
			IndexedDirs: []string{"inbox", "notes", "sources"},
		},
	}
	vaultPath = vault

	t.Cleanup(func() {
		cfg = nil
		vaultPath = ""
		RootCmd.SetArgs(nil)
		jsonOut = false
		ingestDest = "inbox"
		ingestRecursive = true
		ingestMove = false
		ingestReindex = true
		ingestZipFile = ""
		ingestCleanInbox = true
	})

	return vault, dbPath
}

func TestIngestCmd_SingleFile(t *testing.T) {
	vault, dbPath := setupTestVault(t)

	// Create a host file outside the vault
	hostDir := t.TempDir()
	hostFile := filepath.Join(hostDir, "rocket-science.md")
	content := "# Rocket Science\n\nIon thruster dynamics for deep space exploration.\n"
	if err := os.WriteFile(hostFile, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write host file: %v", err)
	}

	RootCmd.SetArgs([]string{"--vault-path", vault, "ingest", hostFile})
	out, err := captureStdout(t, RootCmd.Execute)
	if err != nil {
		t.Fatalf("ingest command failed: %v (output: %s)", err, out)
	}

	// Verify file was ingested into inbox/
	destFile := filepath.Join(vault, "inbox", "rocket-science.md")
	data, err := os.ReadFile(destFile)
	if err != nil {
		t.Fatalf("expected ingested file in inbox: %v", err)
	}
	if !strings.Contains(string(data), "Ion thruster dynamics") {
		t.Errorf("expected content to be preserved, got: %s", string(data))
	}

	// Verify SQLite search index was updated
	engine, err := search.Open(dbPath, vault)
	if err != nil {
		t.Fatalf("failed to open search engine: %v", err)
	}
	defer engine.Close() //nolint:errcheck,gosec

	results, err := engine.Search(t.Context(), "thruster", "all", 10)
	if err != nil {
		t.Fatalf("search query failed: %v", err)
	}
	if len(results) == 0 {
		t.Errorf("expected search engine to index the ingested note, got 0 results")
	}
}

func TestIngestCmd_Directory(t *testing.T) {
	vault, _ := setupTestVault(t)

	// Create host directory structure
	hostDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(hostDir, "tech1.md"), []byte("# Architecture 1\nKernel design"), 0600); err != nil {
		t.Fatal(err)
	}
	subDir := filepath.Join(hostDir, "embedded")
	if err := os.Mkdir(subDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "tech2.md"), []byte("# Firmware\nMicrocontroller HAL"), 0600); err != nil {
		t.Fatal(err)
	}

	RootCmd.SetArgs([]string{"--vault-path", vault, "ingest", hostDir, "--dest", "notes/tech", "--recursive=true"})
	out, err := captureStdout(t, RootCmd.Execute)
	if err != nil {
		t.Fatalf("ingest directory failed: %v (output: %s)", err, out)
	}

	// Verify both files ingested into notes/tech/
	f1 := filepath.Join(vault, "notes", "tech", "tech1.md")
	if _, err := os.Stat(f1); err != nil {
		t.Errorf("expected notes/tech/tech1.md to exist: %v", err)
	}

	f2 := filepath.Join(vault, "notes", "tech", "embedded", "tech2.md")
	if _, err := os.Stat(f2); err != nil {
		t.Errorf("expected notes/tech/embedded/tech2.md to exist: %v", err)
	}
}

func TestIngestCmd_Move(t *testing.T) {
	vault, _ := setupTestVault(t)

	hostDir := t.TempDir()
	hostFile := filepath.Join(hostDir, "transient-draft.md")
	if err := os.WriteFile(hostFile, []byte("# Transient Draft\nTo be moved"), 0600); err != nil {
		t.Fatal(err)
	}

	RootCmd.SetArgs([]string{"--vault-path", vault, "ingest", hostFile, "--move"})
	out, err := captureStdout(t, RootCmd.Execute)
	if err != nil {
		t.Fatalf("ingest with --move failed: %v (output: %s)", err, out)
	}

	// Verify host file was deleted
	if _, err := os.Stat(hostFile); !os.IsNotExist(err) {
		t.Errorf("expected source file to be removed, got err: %v", err)
	}

	// Verify file exists in vault inbox
	destFile := filepath.Join(vault, "inbox", "transient-draft.md")
	if _, err := os.Stat(destFile); err != nil {
		t.Errorf("expected file in vault inbox: %v", err)
	}
}

func TestIngestCmd_NoArgs_MeetingFallback(t *testing.T) {
	vault, _ := setupTestVault(t)

	inboxDir := filepath.Join(vault, "inbox")
	if err := os.MkdirAll(inboxDir, 0750); err != nil {
		t.Fatal(err)
	}

	// Create test meeting note in inbox
	meetFile := filepath.Join(inboxDir, "Alinhamento Orbital - 2026-08-15 - Anotações do Gemini.md")
	meetContent := `# Alinhamento Orbital

convidados Ada Lovelace, Alan Turing

### **Resumo**
Discutimos a migração orbital.

### **Próximas etapas**
- Testar propulsores
`
	if err := os.WriteFile(meetFile, []byte(meetContent), 0600); err != nil {
		t.Fatal(err)
	}

	RootCmd.SetArgs([]string{"--vault-path", vault, "ingest"})
	out, err := captureStdout(t, RootCmd.Execute)
	if err != nil {
		t.Fatalf("ingest fallback failed: %v (output: %s)", err, out)
	}

	if !strings.Contains(out, "Meeting Ingestion") {
		t.Errorf("expected meeting ingestion output, got: %s", out)
	}

	// Verify meeting file was processed and moved to sources/reunioes/
	meetingsDir := filepath.Join(vault, "sources", "reunioes")
	entries, err := os.ReadDir(meetingsDir)
	if err != nil || len(entries) == 0 {
		t.Fatalf("expected meeting note in sources/reunioes, got err: %v, entries: %d", err, len(entries))
	}
}

func TestIngestCmd_JsonOutput(t *testing.T) {
	vault, _ := setupTestVault(t)

	hostDir := t.TempDir()
	hostFile := filepath.Join(hostDir, "structured-note.md")
	if err := os.WriteFile(hostFile, []byte("# Structured Note\nJson parsing test"), 0600); err != nil {
		t.Fatal(err)
	}

	RootCmd.SetArgs([]string{"--vault-path", vault, "ingest", hostFile, "--json"})
	out, err := captureStdout(t, RootCmd.Execute)
	if err != nil {
		t.Fatalf("ingest --json failed: %v (output: %s)", err, out)
	}

	var report ingest.IngestReport
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("failed to parse JSON report: %v (raw output: %s)", err, out)
	}

	if report.TotalProcessed != 1 {
		t.Errorf("expected TotalProcessed == 1, got %d", report.TotalProcessed)
	}
	if len(report.Items) != 1 {
		t.Fatalf("expected 1 item in report, got %d", len(report.Items))
	}
	if report.Items[0].Title != "Structured Note" {
		t.Errorf("expected title 'Structured Note', got %q", report.Items[0].Title)
	}
	if !strings.HasPrefix(report.Items[0].RelPath, "inbox/") {
		t.Errorf("expected RelPath in inbox/, got %q", report.Items[0].RelPath)
	}

	destFile := filepath.Join(vault, report.Items[0].RelPath)
	if _, err := os.Stat(destFile); err != nil {
		t.Errorf("expected destination file to exist at %s: %v", destFile, err)
	}
}
