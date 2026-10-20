package auditor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rjfonseca/daneel/internal/dream"
	"github.com/rjfonseca/daneel/internal/search"
)

func TestHasVaultSearchTool(t *testing.T) {
	tests := []struct {
		tools    []string
		expected bool
	}{
		{tools: []string{"run_command", "view_file"}, expected: false},
		{tools: []string{"search_vault"}, expected: true},
		{tools: []string{"resolve_note"}, expected: true},
		{tools: []string{"get_note_content"}, expected: true},
		{tools: []string{"search_projects"}, expected: true},
		{tools: []string{"search_web"}, expected: false},
		{tools: []string{"grep_search"}, expected: false},
		{tools: []string{"daneel_search"}, expected: true},
	}

	for _, tt := range tests {
		got := HasVaultSearchTool(tt.tools)
		if got != tt.expected {
			t.Errorf("HasVaultSearchTool(%v) = %v, expected %v", tt.tools, got, tt.expected)
		}
	}
}

func TestExtractCentralQuery(t *testing.T) {
	tests := []struct {
		prompt   string
		reply    string
		expected string
	}{
		{
			prompt:   "Como funciona a autenticação do serviço de identidade?",
			reply:    "Qual versão do serviço de identidade você quer saber?",
			expected: "autenticação do serviço de identidade",
		},
		{
			prompt:   "Onde fica o gateway de pagamentos?",
			reply:    "Em qual ambiente?",
			expected: "gateway de pagamentos",
		},
		{
			prompt:   "qual é o fluxo de deploy no kubernetes?",
			reply:    "Você está usando helm ou kustomize?",
			expected: "fluxo de deploy no kubernetes",
		},
	}

	for _, tt := range tests {
		got := ExtractCentralQuery(tt.prompt, tt.reply)
		if got != tt.expected {
			t.Errorf("ExtractCentralQuery(%q, %q) = %q, expected %q", tt.prompt, tt.reply, got, tt.expected)
		}
	}
}

func TestFindSuspiciousDeltas(t *testing.T) {
	now := time.Now()
	interactions := []Interaction{
		{
			AgentName:  "antigravity",
			Timestamp:  now,
			UserPrompt: "Como configuro o novo gateway de pagamentos?",
			AgentReply: "Qual versão do gateway você está usando?",
			ToolsUsed:  []string{},
		},
		{
			AgentName:  "antigravity",
			Timestamp:  now,
			UserPrompt: "Como configuro o novo gateway de pagamentos?",
			AgentReply: "Deixe-me verificar no cofre.",
			ToolsUsed:  []string{"search_vault"}, // Negated: has vault tool
		},
		{
			AgentName:  "claude-code",
			Timestamp:  now,
			UserPrompt: "ok", // Too short
			AgentReply: "Qual é o próximo passo?",
			ToolsUsed:  []string{},
		},
		{
			AgentName:  "cursor",
			Timestamp:  now,
			UserPrompt: "Onde encontro os schemas do Kafka?",
			AgentReply: "Você pode me informar onde residem os schemas?",
			ToolsUsed:  []string{},
		},
	}

	deltas := FindSuspiciousDeltas(interactions)
	if len(deltas) != 2 {
		t.Fatalf("expected 2 suspicious deltas, got %d", len(deltas))
	}

	if deltas[0].Interaction.AgentName != "antigravity" {
		t.Errorf("expected first delta from antigravity, got %s", deltas[0].Interaction.AgentName)
	}
	if deltas[1].Interaction.AgentName != "cursor" {
		t.Errorf("expected second delta from cursor, got %s", deltas[1].Interaction.AgentName)
	}
}

func TestAgentEvaluator_AgnosticScannersAndClassification(t *testing.T) {
	tempBrain := t.TempDir()
	tempClaude := t.TempDir()
	tempCursor := t.TempDir()
	tempVault := t.TempDir()

	// 1. Setup mock Antigravity transcript
	conv1Dir := filepath.Join(tempBrain, "conv-1", ".system_generated", "logs")
	if err := os.MkdirAll(conv1Dir, 0750); err != nil {
		t.Fatalf("failed to create mock conv1: %v", err)
	}
	antigravityJSONL := fmt.Sprintf(`{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":"%s","content":"Como configuro o novo gateway de pagamentos?"}
{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"%s","content":"Qual versão do gateway você está usando?","tool_calls":[]}
`, time.Now().Format(time.RFC3339), time.Now().Format(time.RFC3339))
	_ = os.WriteFile(filepath.Join(conv1Dir, "transcript.jsonl"), []byte(antigravityJSONL), 0600)

	// 2. Setup mock Claude Code transcript
	claudeJSONL := fmt.Sprintf(`{"type":"user","message":{"content":"Onde estão as credenciais do banco staging?"},"timestamp":"%s"}
{"type":"assistant","message":{"content":"Você pode me passar o host ou a senha do banco?"},"tools":[]}
`, time.Now().Format(time.RFC3339))
	_ = os.WriteFile(filepath.Join(tempClaude, "session-1.jsonl"), []byte(claudeJSONL), 0600)

	// 3. Setup mock Cursor transcript (JSON format)
	cursorJSON := fmt.Sprintf(`[
		{"timestamp":"%s","user":"Qual a porta de conexão do redis?","assistant":"Pode confirmar se o redis roda na porta 6379?","tools":[]}
	]`, time.Now().Format(time.RFC3339))
	_ = os.WriteFile(filepath.Join(tempCursor, "chat.json"), []byte(cursorJSON), 0600)

	// 4. Setup mock Vault note: "gateway de pagamentos" exists in vault!
	notesDir := filepath.Join(tempVault, "notes", "sistemas")
	_ = os.MkdirAll(notesDir, 0750)
	noteContent := `---
title: Gateway de Pagamentos
type: arquitetura
tags:
  - pagamentos
  - gateway
---

# Gateway de Pagamentos
Configuração do gateway na versão v2.
`
	_ = os.WriteFile(filepath.Join(notesDir, "gateway-pagamentos.md"), []byte(noteContent), 0600)

	// Setup SQLite search engine with indexed note
	dbPath := filepath.Join(tempVault, ".daneel", "cache", "index.db")
	engine, err := search.Open(dbPath, tempVault)
	if err != nil {
		t.Fatalf("failed to open search engine: %v", err)
	}
	defer engine.Close() //nolint:errcheck

	_, _, err = engine.IndexVault(context.Background(), []string{"notes"}, true)
	if err != nil {
		t.Fatalf("failed to index vault: %v", err)
	}

	// 5. Run AgentEvaluator
	opts := AgentEvalOptions{
		Since:       24 * time.Hour,
		AgentFilter: "all",
		Apply:       true,
		DryRun:      false,
		BrainDir:    tempBrain,
		ClaudeDir:   tempClaude,
		CursorDir:   tempCursor,
		VaultRoot:   tempVault,
	}

	evaluator := NewAgentEvaluator(engine, opts)
	report, err := evaluator.Evaluate(context.Background())
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if report.ScannedInteractions != 3 {
		t.Errorf("expected 3 scanned interactions, got %d", report.ScannedInteractions)
	}
	if report.SuspiciousDeltas != 3 {
		t.Errorf("expected 3 suspicious deltas, got %d", report.SuspiciousDeltas)
	}

	// Verify that Antigravity prompt was classified as MissedOpportunity (note existed!)
	foundMissed := false
	foundGap := false
	for _, res := range report.Results {
		if res.Classification == ClassificationMissedOpportunity {
			foundMissed = true
			if res.AgentName != "antigravity" {
				t.Errorf("expected MissedOpportunity from antigravity, got %s", res.AgentName)
			}
			if len(res.MatchedNotes) == 0 {
				t.Errorf("expected matched notes for MissedOpportunity, got none")
			}
		}
		if res.Classification == ClassificationKnowledgeGap {
			foundGap = true
		}
	}

	if !foundMissed {
		t.Errorf("expected at least one MissedOpportunity")
	}
	if !foundGap {
		t.Errorf("expected at least one KnowledgeGap")
	}

	if report.AppliedCount == 0 {
		t.Errorf("expected AppliedCount > 0 when Apply=true and DryRun=false")
	}
}

func TestUpdateDashboardMetricasEvals(t *testing.T) {
	tempVault := t.TempDir()
	metaDir := filepath.Join(tempVault, "meta")
	_ = os.MkdirAll(metaDir, 0750)

	initialDash := `# 🦅 Painel Central do Cérebro
- Total de Entradas: 42 notas
`
	dashPath := filepath.Join(metaDir, "dashboard-metricas.md")
	_ = os.WriteFile(dashPath, []byte(initialDash), 0600)

	report := &AgentEvalReport{
		Since:               "48h",
		ScannedInteractions: 12,
		SuspiciousDeltas:    4,
		MissedOpportunities: 2,
		KnowledgeGaps:       2,
	}

	UpdateDashboardMetricasEvals(tempVault, report)

	data, err := os.ReadFile(dashPath)
	if err != nil {
		t.Fatalf("failed to read dashboard: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "## 🤖 3. Telemetria de Agentes & Evals") {
		t.Errorf("expected dashboard to contain evals section, got:\n%s", content)
	}
	if !strings.Contains(content, "- **Missed Opportunities (Falha de Autonomia do Agente):** 2") {
		t.Errorf("expected dashboard to contain missed opportunities count, got:\n%s", content)
	}
}

func TestAgentEvaluator_TelemetryCachePersistenceDryRun(t *testing.T) {
	tempBrain := t.TempDir()
	tempVault := t.TempDir()
	_ = os.MkdirAll(filepath.Join(tempVault, ".daneel", "cache"), 0750)

	// Mock Antigravity session
	conv1Dir := filepath.Join(tempBrain, "conv-1", ".system_generated", "logs")
	_ = os.MkdirAll(conv1Dir, 0750)
	antigravityJSONL := fmt.Sprintf(`{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":"%s","content":"Como configuro o novo gateway de pagamentos?"}
{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"%s","content":"Qual versão do gateway você está usando?","tool_calls":[]}
`, time.Now().Format(time.RFC3339), time.Now().Format(time.RFC3339))
	_ = os.WriteFile(filepath.Join(conv1Dir, "transcript.jsonl"), []byte(antigravityJSONL), 0600)

	dbPath := filepath.Join(tempVault, ".daneel", "cache", "index.db")
	engine, err := search.Open(dbPath, tempVault)
	if err != nil {
		t.Fatalf("failed to open search engine: %v", err)
	}
	defer engine.Close() //nolint:errcheck

	opts := AgentEvalOptions{
		Since:       24 * time.Hour,
		AgentFilter: "all",
		Apply:       false,
		DryRun:      true, // Dry run!
		BrainDir:    tempBrain,
		VaultRoot:   tempVault,
	}

	evaluator := NewAgentEvaluator(engine, opts)
	report, err := evaluator.Evaluate(context.Background())
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if report.AppliedCount != 0 {
		t.Errorf("expected AppliedCount 0 in dry run, got %d", report.AppliedCount)
	}

	// Read .daneel/cache/agent_learnings.json
	cacheFile := filepath.Join(tempVault, ".daneel", "cache", "agent_learnings.json")
	data, err := os.ReadFile(cacheFile)
	if err != nil {
		t.Fatalf("expected agent_learnings.json to be created in dry run, got err: %v", err)
	}

	var learnings dream.AgentLearnings
	if err := json.Unmarshal(data, &learnings); err != nil {
		t.Fatalf("failed to unmarshal agent_learnings.json: %v", err)
	}

	if learnings.ScannedTurns != report.ScannedInteractions {
		t.Errorf("expected ScannedTurns %d, got %d", report.ScannedInteractions, learnings.ScannedTurns)
	}
	if learnings.SuspiciousDeltas != report.SuspiciousDeltas {
		t.Errorf("expected SuspiciousDeltas %d, got %d", report.SuspiciousDeltas, learnings.SuspiciousDeltas)
	}
	if len(learnings.FeedbackHistory) == 0 {
		t.Errorf("expected feedback history to contain findings, got 0")
	}
}
