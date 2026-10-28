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
