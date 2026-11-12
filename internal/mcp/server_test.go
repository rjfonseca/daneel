//nolint:revive // package comment not required
package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rjfonseca/daneel/internal/config"
	"github.com/rjfonseca/daneel/internal/routine"
	"github.com/rjfonseca/daneel/internal/search"
)

func TestMCPToonFormat(t *testing.T) {
	tempDir := t.TempDir()

	notesDir := filepath.Join(tempDir, "notes")
	_ = os.MkdirAll(notesDir, 0750)
	testNote := filepath.Join(notesDir, "arch.md")
	_ = os.WriteFile(testNote, []byte("# Architecture Decision\n\nLink to [[other]]"), 0600)
	otherNote := filepath.Join(notesDir, "other.md")
	_ = os.WriteFile(otherNote, []byte("# Other Note\n\nBacklink target"), 0600)

	dbPath := filepath.Join(tempDir, "cache.db")
	engine, err := search.Open(dbPath, tempDir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer engine.Close() //nolint:errcheck,gosec

	_, _, _ = engine.IndexVault(context.Background(), []string{"notes"}, true)

	// 1. tools/list
	// 2. search_vault (default -> TOON)
	// 3. search_vault (explicit markdown -> Markdown)
	// 4. get_backlinks (default -> TOON)
	listReq := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}` + "\n"
	searchToonReq := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"search_vault","arguments":{"query":"Architecture"}}}` + "\n"
	searchMdReq := `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"search_vault","arguments":{"query":"Architecture","format":"markdown"}}}` + "\n"
	backlinksToonReq := `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"get_backlinks","arguments":{"rel_path":"notes/other.md"}}}` + "\n"

	inBuf := bytes.NewBufferString(listReq + searchToonReq + searchMdReq + backlinksToonReq)
	outBuf := &bytes.Buffer{}

	server := &Server{
		vaultRoot: tempDir,
		engine:    engine,
		in:        inBuf,
		out:       outBuf,
	}

	err = server.Serve()
	if err != nil {
		t.Fatalf("Serve failed: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(outBuf.String()), "\n")
	if len(lines) < 4 {
		t.Fatalf("expected 4 responses, got %d: %s", len(lines), outBuf.String())
	}

	// 1. Verify tools/list schema contains "format"
	var listResp JSONRPCResponse
	_ = json.Unmarshal([]byte(lines[0]), &listResp)
	listBytes, _ := json.Marshal(listResp.Result)
	if !strings.Contains(string(listBytes), `"format"`) {
		t.Errorf("tools/list does not declare format parameter: %s", string(listBytes))
	}

	// 2. Verify search_vault default is TOON
	var searchToonResp JSONRPCResponse
	_ = json.Unmarshal([]byte(lines[1]), &searchToonResp)
	searchToonBytes, _ := json.Marshal(searchToonResp.Result)
	searchToonStr := string(searchToonBytes)
	if !strings.Contains(searchToonStr, "results[") || !strings.Contains(searchToonStr, "Architecture Decision") {
		t.Errorf("expected TOON format with results[...], got: %s", searchToonStr)
	}

	// 3. Verify search_vault with format: markdown returns Markdown
	var searchMdResp JSONRPCResponse
	_ = json.Unmarshal([]byte(lines[2]), &searchMdResp)
	searchMdBytes, _ := json.Marshal(searchMdResp.Result)
	searchMdStr := string(searchMdBytes)
	if !strings.Contains(searchMdStr, "# 🔍 Search Results") {
		t.Errorf("expected Markdown header '# 🔍 Search Results', got: %s", searchMdStr)
	}

	// 4. Verify get_backlinks default is TOON
	var backlinksToonResp JSONRPCResponse
	_ = json.Unmarshal([]byte(lines[3]), &backlinksToonResp)
	backlinksBytes, _ := json.Marshal(backlinksToonResp.Result)
	backlinksStr := string(backlinksBytes)
	if !strings.Contains(backlinksStr, "backlinks[") {
		t.Errorf("expected TOON format with backlinks[...], got: %s", backlinksStr)
	}
}

func TestMCPServer(t *testing.T) {
	tempDir := t.TempDir()

	notesDir := filepath.Join(tempDir, "notes")
	_ = os.MkdirAll(notesDir, 0750)

	testNote := filepath.Join(notesDir, "test.md")
	_ = os.WriteFile(testNote, []byte("# Microservices Architecture\n\nDiscussion on microservices."), 0600)

	dbPath := filepath.Join(tempDir, "cache.db")
	engine, err := search.Open(dbPath, tempDir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer engine.Close() //nolint:errcheck,gosec // fully read or safe to ignore

	_, _, _ = engine.IndexVault(context.Background(), []string{"notes"}, true)

	// Create mock remote repo
	mockRemote := filepath.Join(tempDir, "mock-repo")
	_ = os.MkdirAll(mockRemote, 0750)
	_ = exec.Command("git", "-C", mockRemote, "init").Run()                                  //nolint:gosec // intentional
	_ = exec.Command("git", "-C", mockRemote, "config", "user.email", "test@test.com").Run() //nolint:gosec // intentional
	_ = exec.Command("git", "-C", mockRemote, "config", "user.name", "Test").Run()           //nolint:gosec // intentional
	_ = exec.Command("git", "-C", mockRemote, "config", "commit.gpgsign", "false").Run()     //nolint:gosec // intentional
	_ = os.WriteFile(filepath.Join(mockRemote, "README.md"), []byte("# Mock Repo\n\nA mock service for MCP testing.\n"), 0600)
	_ = exec.Command("git", "-C", mockRemote, "add", "-A").Run()            //nolint:gosec // intentional
	_ = exec.Command("git", "-C", mockRemote, "commit", "-m", "init").Run() //nolint:gosec // intentional

	// Pre-create target structure in temporary work root
	workDir := filepath.Join(tempDir, "Work")
	targetDir := filepath.Join(workDir, "github.com", "test", "mock-service")
	_ = os.MkdirAll(filepath.Dir(targetDir), 0750)
	_ = exec.Command("git", "clone", mockRemote, targetDir).Run() //nolint:gosec // intentional

	// 1. Test initialize, list, search, and clone
	initReq := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n"
	listToolsReq := `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}` + "\n"
	searchReq := `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"search_vault","arguments":{"query":"Microservices"}}}` + "\n"
	cloneReq := `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"clone_repository","arguments":{"repo":"github.com/test/mock-service"}}}` + "\n"
	uiOpenReq := `{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"ui_open_note","arguments":{"rel_path":"notes/test.md"}}}` + "\n"
	listAnchorsReq := `{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"ui_list_pinned_anchors","arguments":{}}}` + "\n"
	pinReq := `{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"ui_pin_note","arguments":{"rel_path":"notes/test.md","label":"Test","icon":"📌"}}}` + "\n"
	unpinReq := `{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"ui_unpin_note","arguments":{"rel_path":"notes/test.md"}}}` + "\n"
	uiFocusReq := `{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"ui_focus_domain","arguments":{"domain":"notes/arquitetura"}}}` + "\n"

	inBuf := bytes.NewBufferString(initReq + listToolsReq + searchReq + cloneReq + uiOpenReq + listAnchorsReq + pinReq + unpinReq + uiFocusReq)
	outBuf := &bytes.Buffer{}

	server := &Server{
		vaultRoot: tempDir,
		engine:    engine,
		cfg: &config.Config{
			VaultPath: tempDir,
			Workspace: config.WorkspaceConfig{
				WorkDir: workDir,
			},
		},
		in:  inBuf,
		out: outBuf,
	}

	err = server.Serve()
	if err != nil {
		t.Fatalf("Serve failed: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(outBuf.String()), "\n")
	if len(lines) < 9 {
		t.Fatalf("expected 9 responses, got %d: %s", len(lines), outBuf.String())
	}

	var initResp JSONRPCResponse
	_ = json.Unmarshal([]byte(lines[0]), &initResp)
	if initResp.Error != nil {
		t.Errorf("init returned error: %v", initResp.Error)
	}

	var listResp JSONRPCResponse
	_ = json.Unmarshal([]byte(lines[1]), &listResp)
	if listResp.Error != nil {
		t.Errorf("tools/list returned error: %v", listResp.Error)
	}

	var callResp JSONRPCResponse
	_ = json.Unmarshal([]byte(lines[2]), &callResp)
	if callResp.Error != nil {
		t.Errorf("tools/call (search) returned error: %v", callResp.Error)
	}

	var cloneResp JSONRPCResponse
	_ = json.Unmarshal([]byte(lines[3]), &cloneResp)
	if cloneResp.Error != nil {
		t.Errorf("tools/call (clone) returned error: %v", cloneResp.Error)
	}

	var uiResp JSONRPCResponse
	_ = json.Unmarshal([]byte(lines[4]), &uiResp)
	if uiResp.Error != nil {
		t.Errorf("tools/call (ui_open_note) returned error: %v", uiResp.Error)
	}

	var listAnchorsResp JSONRPCResponse
	_ = json.Unmarshal([]byte(lines[5]), &listAnchorsResp)
	if listAnchorsResp.Error != nil {
		t.Errorf("tools/call (ui_list_pinned_anchors) returned error: %v", listAnchorsResp.Error)
	}

	var pinResp JSONRPCResponse
	_ = json.Unmarshal([]byte(lines[6]), &pinResp)
	if pinResp.Error != nil {
		t.Errorf("tools/call (ui_pin_note) returned error: %v", pinResp.Error)
	}

	var unpinResp JSONRPCResponse
	_ = json.Unmarshal([]byte(lines[7]), &unpinResp)
	if unpinResp.Error != nil {
		t.Errorf("tools/call (ui_unpin_note) returned error: %v", unpinResp.Error)
	}

	var focusResp JSONRPCResponse
	_ = json.Unmarshal([]byte(lines[8]), &focusResp)
	if focusResp.Error != nil {
		t.Errorf("tools/call (ui_focus_domain) returned error: %v", focusResp.Error)
	}
}

func TestMCPDiscoveryTools(t *testing.T) {
	tempDir := t.TempDir()

	notesDir := filepath.Join(tempDir, "notes", "pessoas")
	_ = os.MkdirAll(notesDir, 0750)

	personNote := filepath.Join(notesDir, "alice-carol-smith.md")
	content := `---
type: person
role: Engineering Manager
squad: Tribo Platform
email: alice.smith@acme.corp
aliases:
  - "Alice Carol Smith (asmith)"
  - "asmith"
tags:
  - leadership
---
# 👤 Alice Carol Smith
`
	_ = os.WriteFile(personNote, []byte(content), 0600)

	dbPath := filepath.Join(tempDir, "cache.db")
	engine, err := search.Open(dbPath, tempDir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer engine.Close() //nolint:errcheck,gosec

	_, _, _ = engine.IndexVault(context.Background(), []string{"notes"}, true)

	// Requests to test:
	// 1. tools/list
	// 2. resolve_note
	// 3. search_people
	// 4. search_vault with note_type
	listReq := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}` + "\n"
	resolveReq := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"resolve_note","arguments":{"query":"asmith"}}}` + "\n"
	peopleReq := `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"search_people","arguments":{"query":"Alice Smith"}}}` + "\n"
	searchVaultReq := `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"search_vault","arguments":{"note_type":"person"}}}` + "\n"

	inBuf := bytes.NewBufferString(listReq + resolveReq + peopleReq + searchVaultReq)
	outBuf := &bytes.Buffer{}

	server := &Server{
		vaultRoot: tempDir,
		engine:    engine,
		in:        inBuf,
		out:       outBuf,
	}

	err = server.Serve()
	if err != nil {
		t.Fatalf("Serve failed: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(outBuf.String()), "\n")
	if len(lines) < 4 {
		t.Fatalf("expected 4 responses, got %d: %s", len(lines), outBuf.String())
	}

	// Verify tools/list contains discovery tools
	var listResp JSONRPCResponse
	_ = json.Unmarshal([]byte(lines[0]), &listResp)
	listBytes, _ := json.Marshal(listResp.Result)
	listStr := string(listBytes)
	for _, expectedTool := range []string{"resolve_note", "search_people", "search_projects"} {
		if !strings.Contains(listStr, expectedTool) {
			t.Errorf("tools/list missing tool %s", expectedTool)
		}
	}

	// Verify resolve_note
	var resolveResp JSONRPCResponse
	_ = json.Unmarshal([]byte(lines[1]), &resolveResp)
	if resolveResp.Error != nil {
		t.Errorf("resolve_note returned error: %v", resolveResp.Error)
	}
	resolveBytes, _ := json.Marshal(resolveResp.Result)
	if !strings.Contains(string(resolveBytes), "alice-carol-smith.md") {
		t.Errorf("resolve_note did not match expected path: %s", string(resolveBytes))
	}

	// Verify search_people
	var peopleResp JSONRPCResponse
	_ = json.Unmarshal([]byte(lines[2]), &peopleResp)
	if peopleResp.Error != nil {
		t.Errorf("search_people returned error: %v", peopleResp.Error)
	}
	peopleBytes, _ := json.Marshal(peopleResp.Result)
	if !strings.Contains(string(peopleBytes), "Alice Carol Smith") {
		t.Errorf("search_people did not find Alice: %s", string(peopleBytes))
	}

	// Verify search_vault with note_type
	var searchResp JSONRPCResponse
	_ = json.Unmarshal([]byte(lines[3]), &searchResp)
	if searchResp.Error != nil {
		t.Errorf("search_vault returned error: %v", searchResp.Error)
	}
	searchBytes, _ := json.Marshal(searchResp.Result)
	if !strings.Contains(string(searchBytes), "alice-carol-smith.md") {
		t.Errorf("search_vault did not find note by note_type: %s", string(searchBytes))
	}
}

func TestMCPAgentEvalsTool(t *testing.T) {
	tempDir := t.TempDir()

	dbPath := filepath.Join(tempDir, "cache.db")
	engine, err := search.Open(dbPath, tempDir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer engine.Close() //nolint:errcheck,gosec

	listToolsReq := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}` + "\n"
	evalReq := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"audit_agent_missed_opportunities","arguments":{"since":"24h","agent":"all"}}}` + "\n"

	inBuf := bytes.NewBufferString(listToolsReq + evalReq)
	outBuf := &bytes.Buffer{}

	server := &Server{
		vaultRoot: tempDir,
		engine:    engine,
		in:        inBuf,
		out:       outBuf,
	}

	if err := server.Serve(); err != nil {
		t.Fatalf("Serve failed: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(outBuf.String()), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected 2 responses, got %d: %s", len(lines), outBuf.String())
	}

	// Verify tools/list contains audit_agent_missed_opportunities
	var listResp JSONRPCResponse
	_ = json.Unmarshal([]byte(lines[0]), &listResp)
	listBytes, _ := json.Marshal(listResp.Result)
	if !strings.Contains(string(listBytes), "audit_agent_missed_opportunities") {
		t.Errorf("tools/list missing tool audit_agent_missed_opportunities: %s", string(listBytes))
	}

	// Verify tool call
	var callResp JSONRPCResponse
	_ = json.Unmarshal([]byte(lines[1]), &callResp)
	if callResp.Error != nil {
		t.Errorf("call returned error: %v", callResp.Error)
	}
	callBytes, _ := json.Marshal(callResp.Result)
	if !strings.Contains(string(callBytes), "Agent Evals") {
		t.Errorf("expected response to contain Agent Evals report, got: %s", string(callBytes))
	}
}

func TestNoteToolsStayInsideVault(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	// A sibling temp directory keeps the external fixture isolated from other tests.
	external := t.TempDir()
	outside := filepath.Join(external, "outside.md")
	if err := os.WriteFile(outside, []byte("private external content"), 0600); err != nil {
		t.Fatal(err)
	}
	server := NewServer(root, nil)
	for _, tool := range []string{"get_note_content", "ui_open_note"} {
		for _, path := range []string{outside, "../outside-note.md"} {
			_, isErr := server.executeTool(tool, map[string]interface{}{"rel_path": path})
			if !isErr {
				t.Fatalf("%s accepted external path %q", tool, path)
			}
		}
	}
	if err := os.Symlink(outside, filepath.Join(root, "link.md")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	for _, tool := range []string{"get_note_content", "ui_open_note"} {
		text, isErr := server.executeTool(tool, map[string]interface{}{"rel_path": "link.md"})
		if !isErr || strings.Contains(text, "private external content") {
			t.Fatalf("%s escaped vault: %s", tool, text)
		}
	}
}

func TestMCPTelemetryRecorded(t *testing.T) {
	vaultRoot := t.TempDir()
	dbPath := filepath.Join(vaultRoot, "cache.db")
	engine, err := search.Open(dbPath, vaultRoot)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = engine.Close() }()

	initReq := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n"
	toolReq := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"vault_health","arguments":{}}}` + "\n"

	inBuf := bytes.NewBufferString(initReq + toolReq)
	outBuf := &bytes.Buffer{}

	server := &Server{
		vaultRoot: vaultRoot,
		engine:    engine,
		in:        inBuf,
		out:       outBuf,
	}

	if err := server.Serve(); err != nil {
		t.Fatalf("Serve failed: %v", err)
	}

	store, err := routine.OpenDefaultStore(vaultRoot)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	aggs, err := store.GetCommandAggregates(context.Background(), 0, 50)
	if err != nil {
		t.Fatalf("GetCommandAggregates failed: %v", err)
	}

	found := false
	for _, a := range aggs {
		if a.Surface == "mcp" && a.Command == "vault_health" {
			found = true
			if a.TotalCalls != 1 {
				t.Errorf("expected 1 call, got %d", a.TotalCalls)
			}
			break
		}
	}

	if !found {
		t.Fatalf("expected MCP telemetry record for vault_health, got aggs: %+v", aggs)
	}
}

func TestIngestTools(t *testing.T) {
	vaultRoot := t.TempDir()
	dbPath := filepath.Join(vaultRoot, "cache.db")
	engine, err := search.Open(dbPath, vaultRoot)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = engine.Close() }()

	outsideDir := t.TempDir()

	// Prepare an external file to ingest
	outsideDoc := filepath.Join(outsideDir, "external-doc.md")
	if err := os.WriteFile(outsideDoc, []byte("# External Document\n\nExternal doc content."), 0600); err != nil {
		t.Fatalf("failed to write outside file: %v", err)
	}

	// Prepare an external directory with nested files
	outsideBatchDir := filepath.Join(outsideDir, "batch")
	if err := os.MkdirAll(filepath.Join(outsideBatchDir, "sub"), 0750); err != nil {
		t.Fatalf("failed to create batch dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outsideBatchDir, "note1.md"), []byte("# Note 1\n\nContent 1"), 0600); err != nil {
		t.Fatalf("failed to write batch note1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outsideBatchDir, "sub", "note2.md"), []byte("# Note 2\n\nContent 2"), 0600); err != nil {
		t.Fatalf("failed to write batch note2: %v", err)
	}

	// Prepare a file to test --move
	outsideMove := filepath.Join(outsideDir, "to-move.md")
	if err := os.WriteFile(outsideMove, []byte("# To Move\n\nWill be moved."), 0600); err != nil {
		t.Fatalf("failed to write outside move file: %v", err)
	}

	makeToolCall := func(id int, name string, args map[string]interface{}) string {
		req := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      id,
			"method":  "tools/call",
			"params": map[string]interface{}{
				"name":      name,
				"arguments": args,
			},
		}
		b, _ := json.Marshal(req)
		return string(b)
	}

	reqs := []string{
		// 1. tools/list
		`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`,
		// 2. ingest_note success
		makeToolCall(2, "ingest_note", map[string]interface{}{
			"title":       "Agent Research Note",
			"content":     "# Agent Research Note\n\nSome important finding.",
			"destination": "inbox",
			"tags":        []string{"agent", "research"},
			"source_url":  "https://example.com/research",
		}),
		// 3. ingest_note missing title
		makeToolCall(3, "ingest_note", map[string]interface{}{
			"content": "Missing title content",
		}),
		// 4. ingest_note missing content
		makeToolCall(4, "ingest_note", map[string]interface{}{
			"title": "Missing content",
		}),
		// 5. ingest_path single file
		makeToolCall(5, "ingest_path", map[string]interface{}{
			"path":        outsideDoc,
			"destination": "notes/imports",
		}),
		// 6. ingest_path directory recursive
		makeToolCall(6, "ingest_path", map[string]interface{}{
			"path":        outsideBatchDir,
			"destination": "notes/batch",
			"recursive":   true,
		}),
		// 7. ingest_path with move
		makeToolCall(7, "ingest_path", map[string]interface{}{
			"path":        outsideMove,
			"destination": "inbox",
			"move":        true,
		}),
		// 8. ingest_path missing path
		makeToolCall(8, "ingest_path", map[string]interface{}{
			"path": "",
		}),
		// 9. ingest_path non-existent path
		makeToolCall(9, "ingest_path", map[string]interface{}{
			"path": "/nonexistent/path/note.md",
		}),
	}

	inBuf := bytes.NewBufferString(strings.Join(reqs, "\n") + "\n")
	outBuf := &bytes.Buffer{}

	server := &Server{
		vaultRoot: vaultRoot,
		engine:    engine,
		in:        inBuf,
		out:       outBuf,
	}

	if err := server.Serve(); err != nil {
		t.Fatalf("Serve failed: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(outBuf.String()), "\n")
	if len(lines) != len(reqs) {
		t.Fatalf("expected %d responses, got %d: %s", len(reqs), len(lines), outBuf.String())
	}

	// 1. Verify tools/list contains both tools and their schemas
	var listResp JSONRPCResponse
	if err := json.Unmarshal([]byte(lines[0]), &listResp); err != nil {
		t.Fatalf("unmarshal list response: %v", err)
	}
	var listResult struct {
		Tools []Tool `json:"tools"`
	}
	resBytes, _ := json.Marshal(listResp.Result)
	_ = json.Unmarshal(resBytes, &listResult)

	var ingestNoteTool, ingestPathTool *Tool
	for i := range listResult.Tools {
		if listResult.Tools[i].Name == "ingest_note" {
			ingestNoteTool = &listResult.Tools[i]
		}
		if listResult.Tools[i].Name == "ingest_path" {
			ingestPathTool = &listResult.Tools[i]
		}
	}

	if ingestNoteTool == nil {
		t.Fatalf("tools/list missing tool 'ingest_note'")
	}
	if ingestNoteTool.Description != "Ingests raw markdown note content directly into the Vault, synthesizing YAML frontmatter and indexing in SQLite FTS5." {
		t.Errorf("unexpected ingest_note description: %q", ingestNoteTool.Description)
	}
	schemaBytes, _ := json.Marshal(ingestNoteTool.InputSchema)
	schemaStr := string(schemaBytes)
	for _, prop := range []string{"title", "content", "destination", "tags", "source_url"} {
		if !strings.Contains(schemaStr, `"`+prop+`"`) {
			t.Errorf("ingest_note schema missing property %q: %s", prop, schemaStr)
		}
	}
	if !strings.Contains(schemaStr, `"required":["title","content"]`) && !strings.Contains(schemaStr, `"required":["content","title"]`) {
		t.Errorf("ingest_note schema missing required fields: %s", schemaStr)
	}

	if ingestPathTool == nil {
		t.Fatalf("tools/list missing tool 'ingest_path'")
	}
	if ingestPathTool.Description != "Ingests an arbitrary file or directory from the host filesystem into the Vault, supporting recursive scans, frontmatter synthesis, and FTS5 indexing." {
		t.Errorf("unexpected ingest_path description: %q", ingestPathTool.Description)
	}
	pathSchemaBytes, _ := json.Marshal(ingestPathTool.InputSchema)
	pathSchemaStr := string(pathSchemaBytes)
	for _, prop := range []string{"path", "destination", "recursive", "move"} {
		if !strings.Contains(pathSchemaStr, `"`+prop+`"`) {
			t.Errorf("ingest_path schema missing property %q: %s", prop, pathSchemaStr)
		}
	}
	if !strings.Contains(pathSchemaStr, `"required":["path"]`) {
		t.Errorf("ingest_path schema missing required [path]: %s", pathSchemaStr)
	}

	// 2. ingest_note success
	var noteCallResp JSONRPCResponse
	_ = json.Unmarshal([]byte(lines[1]), &noteCallResp)
	if noteCallResp.Error != nil {
		t.Fatalf("ingest_note call returned RPC error: %v", noteCallResp.Error)
	}
	var noteResult ToolCallResult
	cBytes, _ := json.Marshal(noteCallResp.Result)
	_ = json.Unmarshal(cBytes, &noteResult)
	if noteResult.IsError {
		t.Fatalf("ingest_note returned isError=true: %+v", noteResult)
	}
	if len(noteResult.Content) == 0 {
		t.Fatalf("ingest_note returned empty content")
	}
	var noteData struct {
		Status  string `json:"status"`
		RelPath string `json:"rel_path"`
		Title   string `json:"title"`
	}
	if err := json.Unmarshal([]byte(noteResult.Content[0].Text), &noteData); err != nil {
		t.Fatalf("failed to parse ingest_note response JSON: %v, raw text: %s", err, noteResult.Content[0].Text)
	}
	if noteData.Status != "success" || noteData.Title != "Agent Research Note" {
		t.Errorf("unexpected noteData: %+v", noteData)
	}
	if !strings.HasPrefix(noteData.RelPath, "inbox/agent-research-note") {
		t.Errorf("unexpected rel_path: %q", noteData.RelPath)
	}
	createdNoteBytes, err := os.ReadFile(filepath.Join(vaultRoot, filepath.FromSlash(noteData.RelPath)))
	if err != nil {
		t.Fatalf("failed to read created note from disk: %v", err)
	}
	createdNoteStr := string(createdNoteBytes)
	if !strings.Contains(createdNoteStr, "source_url: https://example.com/research") {
		t.Errorf("created note missing source_url in frontmatter: %s", createdNoteStr)
	}
	if !strings.Contains(createdNoteStr, "tags:") || !strings.Contains(createdNoteStr, "agent") {
		t.Errorf("created note missing tags: %s", createdNoteStr)
	}

	// 3. ingest_note missing title
	var missingTitleResp JSONRPCResponse
	_ = json.Unmarshal([]byte(lines[2]), &missingTitleResp)
	var missingTitleResult ToolCallResult
	cBytes, _ = json.Marshal(missingTitleResp.Result)
	_ = json.Unmarshal(cBytes, &missingTitleResult)
	if !missingTitleResult.IsError {
		t.Errorf("expected isError=true for missing title, got false")
	}

	// 4. ingest_note missing content
	var missingContentResp JSONRPCResponse
	_ = json.Unmarshal([]byte(lines[3]), &missingContentResp)
	var missingContentResult ToolCallResult
	cBytes, _ = json.Marshal(missingContentResp.Result)
	_ = json.Unmarshal(cBytes, &missingContentResult)
	if !missingContentResult.IsError {
		t.Errorf("expected isError=true for missing content, got false")
	}

	// 5. ingest_path single file
	var pathSingleResp JSONRPCResponse
	_ = json.Unmarshal([]byte(lines[4]), &pathSingleResp)
	if pathSingleResp.Error != nil {
		t.Fatalf("ingest_path single returned RPC error: %v", pathSingleResp.Error)
	}
	var pathSingleResult ToolCallResult
	cBytes, _ = json.Marshal(pathSingleResp.Result)
	_ = json.Unmarshal(cBytes, &pathSingleResult)
	if pathSingleResult.IsError {
		t.Fatalf("ingest_path single returned isError=true: %+v", pathSingleResult)
	}
	if len(pathSingleResult.Content) == 0 {
		t.Fatalf("ingest_path single returned empty content")
	}
	var pathReport struct {
		TotalProcessed int `json:"total_processed"`
		Items          []struct {
			RelPath string `json:"rel_path"`
			Title   string `json:"title"`
		} `json:"items"`
		Errors []string `json:"errors"`
	}
	if err := json.Unmarshal([]byte(pathSingleResult.Content[0].Text), &pathReport); err != nil {
		t.Fatalf("failed to parse ingest_path report: %v, raw text: %s", err, pathSingleResult.Content[0].Text)
	}
	if pathReport.TotalProcessed != 1 || len(pathReport.Items) != 1 {
		t.Fatalf("expected 1 processed item, got %+v", pathReport)
	}
	if !strings.HasPrefix(pathReport.Items[0].RelPath, "notes/imports/") {
		t.Errorf("expected item rel_path in notes/imports/, got %q", pathReport.Items[0].RelPath)
	}

	// 6. ingest_path directory recursive
	var pathBatchResp JSONRPCResponse
	_ = json.Unmarshal([]byte(lines[5]), &pathBatchResp)
	if pathBatchResp.Error != nil {
		t.Fatalf("ingest_path batch returned RPC error: %v", pathBatchResp.Error)
	}
	var pathBatchResult ToolCallResult
	cBytes, _ = json.Marshal(pathBatchResp.Result)
	_ = json.Unmarshal(cBytes, &pathBatchResult)
	if pathBatchResult.IsError {
		t.Fatalf("ingest_path batch returned isError=true: %+v", pathBatchResult)
	}
	if len(pathBatchResult.Content) == 0 {
		t.Fatalf("ingest_path batch returned empty content")
	}
	var batchReport struct {
		TotalProcessed int `json:"total_processed"`
		Items          []struct {
			RelPath string `json:"rel_path"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(pathBatchResult.Content[0].Text), &batchReport); err != nil {
		t.Fatalf("failed to parse batch report: %v", err)
	}
	if batchReport.TotalProcessed != 2 || len(batchReport.Items) != 2 {
		t.Fatalf("expected 2 processed items, got %+v", batchReport)
	}

	// 7. ingest_path with move
	var pathMoveResp JSONRPCResponse
	_ = json.Unmarshal([]byte(lines[6]), &pathMoveResp)
	if pathMoveResp.Error != nil {
		t.Fatalf("ingest_path move returned RPC error: %v", pathMoveResp.Error)
	}
	var pathMoveResult ToolCallResult
	cBytes, _ = json.Marshal(pathMoveResp.Result)
	_ = json.Unmarshal(cBytes, &pathMoveResult)
	if pathMoveResult.IsError {
		t.Fatalf("ingest_path move returned isError=true: %+v", pathMoveResult)
	}
	if len(pathMoveResult.Content) == 0 {
		t.Fatalf("ingest_path move returned empty content")
	}
	if _, err := os.Stat(outsideMove); !os.IsNotExist(err) {
		t.Errorf("expected source file %s to be deleted after move, err: %v", outsideMove, err)
	}

	// 8. ingest_path missing path
	var missingPathResp JSONRPCResponse
	_ = json.Unmarshal([]byte(lines[7]), &missingPathResp)
	var missingPathResult ToolCallResult
	cBytes, _ = json.Marshal(missingPathResp.Result)
	_ = json.Unmarshal(cBytes, &missingPathResult)
	if !missingPathResult.IsError {
		t.Errorf("expected isError=true for empty path, got false")
	}

	// 9. ingest_path non-existent path
	var nonExistentResp JSONRPCResponse
	_ = json.Unmarshal([]byte(lines[8]), &nonExistentResp)
	var nonExistentResult ToolCallResult
	cBytes, _ = json.Marshal(nonExistentResp.Result)
	_ = json.Unmarshal(cBytes, &nonExistentResult)
	if !nonExistentResult.IsError {
		t.Errorf("expected isError=true for non-existent path, got false")
	}
}
