package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rjfonseca/daneel/internal/config"
)

func TestAuditSessionsCmd(t *testing.T) {
	tempBrain := t.TempDir()
	tempVault := t.TempDir()

	// Setup mock conversation
	convDir := filepath.Join(tempBrain, "test-conv", ".system_generated", "logs")
	if err := os.MkdirAll(convDir, 0750); err != nil {
		t.Fatalf("failed to create mock brain dir: %v", err)
	}

	convJSONL := fmt.Sprintf(`{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":"%s","content":"<USER_REQUEST>\nLembre-se de não comitar direto na main!\n</USER_REQUEST>"}
{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"%s","content":"Certo, branch separada criada.","tool_calls":[]}
`, time.Now().Format(time.RFC3339), time.Now().Format(time.RFC3339))

	if err := os.WriteFile(filepath.Join(convDir, "transcript.jsonl"), []byte(convJSONL), 0600); err != nil {
		t.Fatalf("failed to write transcript: %v", err)
	}

	cfg = &config.Config{
		VaultPath: tempVault,
	}

	b := new(bytes.Buffer)
	RootCmd.SetOut(b)
	RootCmd.SetErr(b)
	RootCmd.SetArgs([]string{"audit", "sessions", "--since=24h", "--dry-run", "--brain-dir=" + tempBrain})

	err := RootCmd.Execute()
	if err != nil {
		t.Fatalf("audit sessions command failed: %v", err)
	}
}

func TestAuditAgentsCmd(t *testing.T) {
	tempBrain := t.TempDir()
	tempClaude := t.TempDir()
	tempCursor := t.TempDir()
	tempVault := t.TempDir()

	// Mock Antigravity transcript
	convDir := filepath.Join(tempBrain, "test-conv", ".system_generated", "logs")
	_ = os.MkdirAll(convDir, 0750)
	convJSONL := fmt.Sprintf(`{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":"%s","content":"Como configuro o novo gateway de pagamentos?"}
{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"%s","content":"Qual versão do gateway você está usando?","tool_calls":[]}
`, time.Now().Format(time.RFC3339), time.Now().Format(time.RFC3339))
	_ = os.WriteFile(filepath.Join(convDir, "transcript.jsonl"), []byte(convJSONL), 0600)

	cfg = &config.Config{
		VaultPath: tempVault,
		DBPath:    filepath.Join(tempVault, "cache", "index.db"),
	}

	b := new(bytes.Buffer)
	RootCmd.SetOut(b)
	RootCmd.SetErr(b)
	RootCmd.SetArgs([]string{
		"audit", "agents",
		"--since=24h",
		"--agent=all",
		"--brain-dir=" + tempBrain,
		"--claude-dir=" + tempClaude,
		"--cursor-dir=" + tempCursor,
		"--json",
	})

	err := RootCmd.Execute()
	if err != nil {
		t.Fatalf("audit agents command failed: %v", err)
	}
}
