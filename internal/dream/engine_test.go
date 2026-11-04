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
	pessoasDir := filepath.Join(tempDir, "notes", "pessoas")
	dailyDir := filepath.Join(tempDir, "daily")
	inboxDir := filepath.Join(tempDir, "inbox")
	metaDir := filepath.Join(tempDir, "meta", "pessoas")
	carreiraDir := filepath.Join(tempDir, "meta", "carreira")

	_ = os.MkdirAll(notesDir, 0750)
	_ = os.MkdirAll(pessoasDir, 0750)
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

	// Create note in notes/pessoas/
	janeFile := filepath.Join(pessoasDir, "jane-doe.md")
	janeContent := `---
type: pessoa
description: Engenheira de Software.
---
# Jane Doe
`
	_ = os.WriteFile(janeFile, []byte(janeContent), 0600)

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

	mocPessoasPath := filepath.Join(tempDir, "notes", "pessoas", "moc-pessoas.md")
	content, err := os.ReadFile(mocPessoasPath)
	if err != nil {
		t.Fatalf("failed to read moc file: %v", err)
	}

	if !strings.Contains(string(content), "jane-doe") {
		t.Errorf("expected moc-pessoas.md to contain synchronized jane-doe link")
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

func TestDreamEngine_SetForce(t *testing.T) {
	engine := NewDreamEngine(t.TempDir())
	if engine.force {
		t.Errorf("expected force to be false by default")
	}
	engine.SetForce(true)
	if !engine.force {
		t.Errorf("expected force to be true after SetForce(true)")
	}
	engine.SetForce(false)
	if engine.force {
		t.Errorf("expected force to be false after SetForce(false)")
	}
}

func TestDreamEngine_IncrementalHealsCachedWikiLinks(t *testing.T) {
	tempDir := t.TempDir()

	engDir := filepath.Join(tempDir, "notes", "eng")
	if err := os.MkdirAll(engDir, 0750); err != nil {
		t.Fatalf("failed to create notes/eng: %v", err)
	}

	targetDoc := filepath.Join(engDir, "target-doc.md")
	callerDoc := filepath.Join(engDir, "caller.md")

	if err := os.WriteFile(targetDoc, []byte("# Target Doc\n"), 0600); err != nil {
		t.Fatalf("failed to write target-doc.md: %v", err)
	}
	if err := os.WriteFile(callerDoc, []byte("See [[Target Doc]]\n"), 0600); err != nil {
		t.Fatalf("failed to write caller.md: %v", err)
	}

	engine := NewDreamEngine(tempDir)

	// Step 1: dryRun=true (autoFix=false), populates file_watermarks while leaving [[Target Doc]] unhealed on disk
	rep1, err := engine.RunFullCycle("2026-11-04", false, true)
	if err != nil {
		t.Fatalf("dry run failed: %v", err)
	}
	if rep1.FixedLinksCount != 1 {
		t.Fatalf("expected dry run to detect 1 fixable link, got %d", rep1.FixedLinksCount)
	}

	// Verify caller.md is still unhealed on disk
	dataBefore, err := os.ReadFile(callerDoc)
	if err != nil {
		t.Fatalf("failed to read caller doc: %v", err)
	}
	if !strings.Contains(string(dataBefore), "[[Target Doc]]") {
		t.Fatalf("expected caller doc to still contain [[Target Doc]], got: %s", string(dataBefore))
	}

	// Step 2: Incremental run where both files have unchanged mtime in file_watermarks
	// caller.md has Content == "" initially, but Links has IsWikiLink: true
	rep2, err := engine.RunFullCycle("2026-11-04", true, false)
	if err != nil {
		t.Fatalf("incremental auto-fix run failed: %v", err)
	}
	if rep2.FixedLinksCount != 1 {
		t.Fatalf("expected incremental run to heal 1 link, got %d", rep2.FixedLinksCount)
	}

	// Verify caller.md on disk now contains [Target Doc](target-doc.md)
	dataAfter, err := os.ReadFile(callerDoc)
	if err != nil {
		t.Fatalf("failed to read caller doc after heal: %v", err)
	}
	expectedLink := "[Target Doc](target-doc.md)"
	if !strings.Contains(string(dataAfter), expectedLink) {
		t.Fatalf("expected caller doc to contain %q, got:\n%s", expectedLink, string(dataAfter))
	}
}

func TestDreamEngine_AutoIngestsMeetingsFromInbox(t *testing.T) {
	tmpDir := t.TempDir()
	inboxDir := filepath.Join(tmpDir, "inbox")
	if err := os.MkdirAll(inboxDir, 0750); err != nil {
		t.Fatalf("failed to create inbox directory: %v", err)
	}

	meetingContent := `Convidados: Ada Lovelace, Alan Turing
### Resumo
Alinhamento técnico sobre arquitetura de mensageria.
### Próximas etapas
- Documentar contrato de eventos
`
	meetingFile := filepath.Join(inboxDir, "Reuniao Arquitetura - 2026-11-04 - Anotações do Gemini.md")
	if err := os.WriteFile(meetingFile, []byte(meetingContent), 0600); err != nil {
		t.Fatalf("failed to write meeting draft: %v", err)
	}

	engine := NewDreamEngine(tmpDir)
	rep, err := engine.RunFullCycle("2026-11-04", true, false)
	if err != nil {
		t.Fatalf("RunFullCycle failed: %v", err)
	}

	if rep.IngestedMeetingsCount != 1 {
		t.Errorf("expected IngestedMeetingsCount 1, got %d", rep.IngestedMeetingsCount)
	}

	if len(rep.InboxTriaged) != 0 {
		t.Errorf("expected 0 inbox triaged items, got %d", len(rep.InboxTriaged))
	}

	expectedPath := filepath.Join(tmpDir, "sources", "reunioes", "2026-11-04-reuniao-arquitetura.md")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("expected %s to exist on disk", expectedPath)
	}
}
