//nolint:revive // package comment not required
package dream

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestFullDreamCycle(t *testing.T) {
	tempDir := t.TempDir()

	notesDir := filepath.Join(tempDir, "notes", "ia")
	dailyDir := filepath.Join(tempDir, "daily")
	inboxDir := filepath.Join(tempDir, "inbox")
	metaDir := filepath.Join(tempDir, "meta", "pessoas")
	carreiraDir := filepath.Join(tempDir, "meta", "carreira")

	_ = os.MkdirAll(notesDir, 0750)
	_ = os.MkdirAll(dailyDir, 0750)
	_ = os.MkdirAll(inboxDir, 0750)
	_ = os.MkdirAll(metaDir, 0750)
	_ = os.MkdirAll(carreiraDir, 0750)

	// Create MOC file
	mocFile := filepath.Join(notesDir, "moc-ia.md")
	_ = os.WriteFile(mocFile, []byte("# MOC IA\n\n"), 0600)

	// Create note in notes/ia/
	noteFile := filepath.Join(notesDir, "rag-avancado.md")
	noteContent := `---
type: tecnologia
description: RAG com FTS5 e BM25.
---
# RAG Avançado
`
	_ = os.WriteFile(noteFile, []byte(noteContent), 0600)

	// Create daily note
	dailyFile := filepath.Join(dailyDir, "2026-08-15.md")
	dailyContent := `---
type: daily
tempo-A_Estrategico: 4.0
tempo-B_ImpactoLocal: 2.0
tempo-C_GlueWork: 1.0
tempo-E_Overhead: 0.5
---
# 2026-08-15

## 🧠 Conhecimento & Descobertas
- [Arquitetura de Arenas de Memória](https://go.dev/arena)

## 📌 Atividades Realizadas
- [A] Trabalhei no Daneel.
`
	_ = os.WriteFile(dailyFile, []byte(dailyContent), 0600)

	// Create draft in inbox
	inboxDraft := filepath.Join(inboxDir, "rascunho-mcp.md")
	_ = os.WriteFile(inboxDraft, []byte("# MCP Tools\n\nNotas rápidas sobre MCP."), 0600)

	engine := NewDreamEngine(tempDir)
	rep, err := engine.RunFullCycle("2026-08-15", true, false)
	if err != nil {
		t.Fatalf("RunFullCycle failed: %v", err)
	}

	if rep.DailyTelemetry == nil {
		t.Errorf("expected daily telemetry to be collected")
	}

	if len(rep.EmergingConcepts) != 1 {
		t.Errorf("expected 1 emerging concept, got %d", len(rep.EmergingConcepts))
	}

	if len(rep.InboxTriaged) != 1 {
		t.Errorf("expected 1 inbox triaged item, got %d", len(rep.InboxTriaged))
	}

	// Verify MOC sync auto-fix
	mocData, _ := os.ReadFile(mocFile) //nolint:gosec // intentional
	if !strings.Contains(string(mocData), "rag-avancado.md") {
		t.Errorf("expected moc-ia.md to contain synchronized rag-avancado.md link")
	}

	// Verify feedback recording
	err = RecordFeedback(tempDir, "mcp-tools", "manual", "promote_to_ia", "Categorizado com sucesso")
	if err != nil {
		t.Errorf("RecordFeedback failed: %v", err)
	}
}

func TestFullDreamCycle_MissingWorkspaceHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	vaultDir := t.TempDir()
	report, err := NewDreamEngine(vaultDir).RunFullCycle("2026-10-20", true, false)
	if err != nil {
		t.Fatal(err)
	}
	if report.WorkspaceSummary == nil || report.WorkspaceSummary.TotalRepos != 0 {
		t.Fatalf("expected an empty workspace summary: %+v", report.WorkspaceSummary)
	}
	path := filepath.Join(vaultDir, "notes", "organizacao", "moc-workspace-work.md")
	if _, err := os.ReadFile(path); err != nil {
		t.Fatalf("expected a catalog inside the vault: %v", err)
	}
}

func TestFullDreamCycle_MetacognitionHandoffPersistence(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	tempDir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(tempDir, ".daneel", "cache"), 0750)

	notesDir := filepath.Join(tempDir, "notes", "sistemas")
	_ = os.MkdirAll(notesDir, 0750)
	noteFile := filepath.Join(notesDir, "gateway.md")
	_ = os.WriteFile(noteFile, []byte("# Gateway\nSistema de proxy reverso.\n"), 0600)

	engine := NewDreamEngine(tempDir)
	rep, err := engine.RunFullCycle("2026-09-22", false, false)
	if err != nil {
		t.Fatalf("RunFullCycle failed: %v", err)
	}

	if rep.Metacognition == nil {
		t.Fatalf("expected in-memory report.Metacognition to not be nil")
	}

	cacheFile := filepath.Join(tempDir, ".daneel", "cache", "dream_handoff.json")
	data, err := os.ReadFile(cacheFile)
	if err != nil {
		t.Fatalf("failed to read persisted dream_handoff.json: %v", err)
	}

	var cached FullDreamReport
	if err := json.Unmarshal(data, &cached); err != nil {
		t.Fatalf("failed to unmarshal cached dream_handoff.json: %v", err)
	}

	if cached.Metacognition == nil {
		t.Fatalf("regression: expected persisted dream_handoff.json to have Metacognition, got nil")
	}

	if len(cached.Metacognition.Timings) == 0 {
		t.Errorf("expected cached Metacognition.Timings to have items, got 0")
	}

	// Millisecond timings may legitimately be zero on a fast, empty workspace.
	if !reflect.DeepEqual(cached.Metacognition, rep.Metacognition) {
		t.Errorf("persisted metacognition differs from the completed in-memory report")
	}
}
