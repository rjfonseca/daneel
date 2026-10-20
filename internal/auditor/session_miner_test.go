package auditor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSessionMiner_Audit(t *testing.T) {
	tempBrain := t.TempDir()
	tempVault := t.TempDir()

	// Setup mock conversation 1: User Correction
	conv1Dir := filepath.Join(tempBrain, "conv-1", ".system_generated", "logs")
	if err := os.MkdirAll(conv1Dir, 0750); err != nil {
		t.Fatalf("failed to create mock conv1: %v", err)
	}

	conv1JSONL := fmt.Sprintf(`{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":"%s","content":"<USER_REQUEST>\nLembre-se de nunca commitar na branch main!\n</USER_REQUEST>"}
{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"%s","content":"Entendido, vou usar uma branch separada.","tool_calls":[{"name":"run_command"}]}
`, time.Now().Format(time.RFC3339), time.Now().Format(time.RFC3339))

	if err := os.WriteFile(filepath.Join(conv1Dir, "transcript.jsonl"), []byte(conv1JSONL), 0600); err != nil {
		t.Fatalf("failed to write conv1 transcript: %v", err)
	}

	// Setup mock conversation 2: Multi-step repetitive workflow (Skill)
	conv2Dir := filepath.Join(tempBrain, "conv-2")
	if err := os.MkdirAll(conv2Dir, 0750); err != nil {
		t.Fatalf("failed to create mock conv2: %v", err)
	}

	conv2JSONL := fmt.Sprintf(`{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":"%s","content":"Pesquise os erros no log, depois execute o teste e me dê um relatório."}
{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"%s","content":"Aqui está o relatório.","tool_calls":[{"name":"view_file"}]}
`, time.Now().Format(time.RFC3339), time.Now().Format(time.RFC3339))

	if err := os.WriteFile(filepath.Join(conv2Dir, "transcript.jsonl"), []byte(conv2JSONL), 0600); err != nil {
		t.Fatalf("failed to write conv2 transcript: %v", err)
	}

	// Setup mock conversation 3: Knowledge Gap (Evals)
	conv3Dir := filepath.Join(tempBrain, "conv-3")
	if err := os.MkdirAll(conv3Dir, 0750); err != nil {
		t.Fatalf("failed to create mock conv3: %v", err)
	}

	conv3JSONL := fmt.Sprintf(`{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":"%s","content":"Como configuro o novo gateway de pagamentos?"}
{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"%s","content":"Qual versão do gateway você está usando?","tool_calls":[]}
`, time.Now().Format(time.RFC3339), time.Now().Format(time.RFC3339))

	if err := os.WriteFile(filepath.Join(conv3Dir, "transcript.jsonl"), []byte(conv3JSONL), 0600); err != nil {
		t.Fatalf("failed to write conv3 transcript: %v", err)
	}

	// Run SessionMiner Audit
	miner := NewSessionMiner(AuditOptions{
		Since:      24 * time.Hour,
		BrainDir:   tempBrain,
		VaultRoot:  tempVault,
		MineSkills: true,
		Evals:      true,
		Apply:      true,
		DryRun:     false,
	})

	ctx := context.Background()
	report, err := miner.Audit(ctx)
	if err != nil {
		t.Fatalf("Audit failed: %v", err)
	}

	if report.ScannedConversations != 3 {
		t.Errorf("expected 3 scanned conversations, got %d", report.ScannedConversations)
	}

	if report.ScannedTurns != 3 {
		t.Errorf("expected 3 scanned turns, got %d", report.ScannedTurns)
	}

	// Verify Correction discovered
	foundCorrection := false
	for _, l := range report.Learnings {
		if l.SignalType == SignalCorrection {
			foundCorrection = true
			if l.ConversationID != "conv-1" {
				t.Errorf("expected conv-1 for correction, got %s", l.ConversationID)
			}
		}
	}
	if !foundCorrection {
		t.Errorf("expected to find at least one user correction learning")
	}

	// Verify Skill Proposal discovered
	if len(report.SkillProposals) == 0 {
		t.Errorf("expected at least one skill proposal, got 0")
	}

	// Verify Knowledge Gap discovered
	foundGap := false
	for _, l := range report.Learnings {
		if l.SignalType == SignalKnowledgeGap {
			foundGap = true
		}
	}
	if !foundGap {
		t.Errorf("expected to find at least one knowledge gap learning")
	}

	// Verify Apply wrote files into tempVault
	proposalDir := filepath.Join(tempVault, ".agents", "skills", "propostas")
	entries, err := os.ReadDir(proposalDir)
	if err != nil || len(entries) == 0 {
		t.Errorf("expected skill proposals directory to contain entries, got err=%v", err)
	}
}
