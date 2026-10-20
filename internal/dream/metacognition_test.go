package dream

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMetacognition_PassAndBottlenecks(t *testing.T) {
	tempVault := t.TempDir()

	report := &FullDreamReport{
		Date:              "2026-09-18",
		GeneratedAt:       time.Now().Format(time.RFC3339),
		TotalFilesScanned: 150,
		FixedLinksCount:   5,
		BrokenLinksCount:  3,
		MissingMOCs:       []string{"notes/sistemas"},
		InboxTriaged: []InboxTriageItem{
			{File: "1.md"}, {File: "2.md"}, {File: "3.md"}, {File: "4.md"}, {File: "5.md"},
			{File: "6.md"}, {File: "7.md"}, {File: "8.md"}, {File: "9.md"}, {File: "10.md"},
		},
		TemporalAnalysis: &TemporalWindowData{
			DriftAlerts: []string{"Strategic time below target (20% vs 30%)"},
		},
	}

	timings := []StageTiming{
		{Name: "Phase 1: Deterministic Graph Hygiene", DurationMs: 40, ItemsCount: 150},
		{Name: "Phase 2: Strategic Telemetry & Radar", DurationMs: 30, ItemsCount: 4},
		{Name: "Phase 3: Zettelkasten Inbox Triage", DurationMs: 25, ItemsCount: 10},
	}

	metaRep := RunMetacognitivePass(tempVault, report, timings)
	if metaRep == nil {
		t.Fatalf("RunMetacognitivePass returned nil")
	}

	if metaRep.TotalDurationMs != 95 {
		t.Errorf("expected total duration 95ms, got %d", metaRep.TotalDurationMs)
	}

	// Verify bottlenecks flagged
	if len(metaRep.Bottlenecks) < 3 {
		t.Errorf("expected at least 3 bottlenecks (broken links, inbox density, missing MOCs), got %d", len(metaRep.Bottlenecks))
	}

	// Verify insights generated
	if len(metaRep.Insights) < 3 {
		t.Errorf("expected at least 3 insights, got %d", len(metaRep.Insights))
	}
}

func TestMetacognition_KnowledgeSynthesisInduction(t *testing.T) {
	tempVault := t.TempDir()

	report := &FullDreamReport{
		Date: "2026-09-18",
		EmergingConcepts: []EmergingConcept{
			{
				Title:   "Lock-Free RingBuffer",
				Context: "Estudo sobre estruturas de dados sem lock em Go para mensageria de alta vazão.",
			},
		},
		TemporalAnalysis: &TemporalWindowData{
			TopSystems: []string{"kafka-producer", "billing-gateway"},
		},
	}

	syntheses, err := InduceKnowledgeSyntheses(tempVault, report, true, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(syntheses) == 0 {
		t.Fatalf("expected at least 1 knowledge synthesis draft, got 0")
	}

	first := syntheses[0]
	if !strings.Contains(first.Title, "Lock-Free RingBuffer") {
		t.Errorf("expected synthesis title to reference emerging concept, got %s", first.Title)
	}

	if first.CreatedPath == "" {
		t.Errorf("expected CreatedPath to be populated when autoFix=true and dryRun=false")
	}

	// Check file written to disk
	absCreated := filepath.Join(tempVault, first.CreatedPath)
	data, err := os.ReadFile(absCreated)
	if err != nil {
		t.Fatalf("failed to read created synthesis draft: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "type: sintese") {
		t.Errorf("expected synthesis to have OKF type: sintese, got content:\n%s", content)
	}
}

func TestMetacognition_EvolutionLogAndDashboard(t *testing.T) {
	tempVault := t.TempDir()

	report := &FullDreamReport{
		Date:              "2026-09-18",
		GeneratedAt:       time.Now().Format(time.RFC3339),
		TotalFilesScanned: 80,
		FixedLinksCount:   2,
		BrokenLinksCount:  0,
		Metacognition: &MetacognitiveReport{
			TotalDurationMs: 120,
			Timings: []StageTiming{
				{Name: "Phase 1: Deterministic Graph Hygiene", DurationMs: 50, ItemsCount: 80},
				{Name: "Phase 2: Strategic Telemetry & Radar", DurationMs: 30, ItemsCount: 2},
				{Name: "Phase 3: Zettelkasten Inbox Triage", DurationMs: 20, ItemsCount: 1},
				{Name: "Phase 4: Metacognition & Self-Evaluation", DurationMs: 10, ItemsCount: 2},
				{Name: "Phase 5: Knowledge Synthesis Induction", DurationMs: 10, ItemsCount: 1},
			},
			Bottlenecks: []string{},
			Insights: []MetacognitiveInsight{
				{
					Category:        "synthesis",
					Observation:     "Conexão cruzada estabelecida entre Kafka e Billing",
					Impact:          "Novas oportunidades arquiteturais mapeadas",
					SuggestedAction: "Revisar nota de síntese induzida",
				},
			},
			Syntheses: []SynthesisDraft{
				{
					Title:       "Síntese: Kafka e Billing",
					Slug:        "sintese-kafka-billing",
					CreatedPath: "notes/sinteses/sintese-kafka-billing.md",
				},
			},
			EvolutionLogPath: "meta/guias/agente-evolucao.md",
		},
	}

	// 1. Test AppendEvolutionLog
	err := AppendEvolutionLog(tempVault, "2026-09-18", report)
	if err != nil {
		t.Fatalf("AppendEvolutionLog failed: %v", err)
	}

	logPath := filepath.Join(tempVault, "meta", "guias", "agente-evolucao.md")
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read evolution log: %v", err)
	}

	logContent := string(data)
	if !strings.Contains(logContent, "Diário de Evolução do Agente") {
		t.Errorf("expected evolution log header, got:\n%s", logContent)
	}
	if !strings.Contains(logContent, "120ms") {
		t.Errorf("expected evolution log to record duration 120ms, got:\n%s", logContent)
	}
	if !strings.Contains(logContent, "Sínteses Ativas Induzidas") {
		t.Errorf("expected evolution log to mention syntheses, got:\n%s", logContent)
	}

	// 2. Test UpdateDashboardMetricas
	if err := UpdateDashboardMetricas(tempVault, report, 80); err != nil {
		t.Fatal(err)
	}
	dashPath := filepath.Join(tempVault, "meta", "dashboard-metricas.md")
	dashData, err := os.ReadFile(dashPath)
	if err != nil {
		t.Fatalf("failed to read dashboard: %v", err)
	}

	dashContent := string(dashData)
	if !strings.Contains(dashContent, "## 🧠 4. Metacognição & Evolução Autônoma do Agente") {
		t.Errorf("expected dashboard to contain section 4 Metacognição, got:\n%s", dashContent)
	}
	if !strings.Contains(dashContent, "Tempo Total do Ciclo:** 120ms") {
		t.Errorf("expected dashboard to contain cycle duration, got:\n%s", dashContent)
	}
}
