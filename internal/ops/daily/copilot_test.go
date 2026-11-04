package daily_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rjfonseca/daneel/internal/config"
	"github.com/rjfonseca/daneel/internal/dream"
	"github.com/rjfonseca/daneel/internal/ops/daily"
)

func TestRunMorning_ScaffoldsDailyAndIsIdempotent(t *testing.T) {
	tempVault := t.TempDir()

	// 1. Setup .daneel/cache/dream_handoff.json
	cacheDir := filepath.Join(tempVault, ".daneel", "cache")
	if err := os.MkdirAll(cacheDir, 0750); err != nil {
		t.Fatalf("failed to create cache dir: %v", err)
	}

	dreamHandoff := dream.FullDreamReport{
		Date:                  "2026-11-03",
		FixedLinksCount:       3,
		MissingMOCs:           []string{"notes/arch/service.md"},
		SyncedPeople:          []string{"Ada Lovelace"},
		IngestedMeetingsCount: 2,
		BrokenLinksCount:      1,
		TemporalAnalysis: &dream.TemporalWindowData{
			DriftAlerts: []string{"Strategic time below 30%"},
		},
		Metacognition: &dream.MetacognitiveReport{
			Insights: []dream.MetacognitiveInsight{
				{
					Observation:     "Bottleneck in review",
					SuggestedAction: "Automate linting",
				},
			},
		},
		HandoffQuestions: []dream.HandoffQuestion{
			{
				Type:     "strategic",
				Question: "Should we deprecate v1 API?",
			},
		},
	}
	handoffBytes, err := json.Marshal(dreamHandoff)
	if err != nil {
		t.Fatalf("failed to marshal dream handoff: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "dream_handoff.json"), handoffBytes, 0600); err != nil {
		t.Fatalf("failed to write dream handoff: %v", err)
	}

	// 2. Setup inbox/ with .md file, subfolder, and non-md file
	inboxDir := filepath.Join(tempVault, "inbox")
	if err := os.MkdirAll(filepath.Join(inboxDir, "subfolder"), 0750); err != nil {
		t.Fatalf("failed to create inbox dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(inboxDir, "task1.md"), []byte("# Task 1"), 0600); err != nil {
		t.Fatalf("failed to write inbox task: %v", err)
	}
	if err := os.WriteFile(filepath.Join(inboxDir, "image.png"), []byte("binary data"), 0600); err != nil {
		t.Fatalf("failed to write inbox image: %v", err)
	}

	// 3. Setup notes/carreira/sumario-semanal-atividades-impactos-2026.md
	backlogDir := filepath.Join(tempVault, "notes", "carreira")
	if err := os.MkdirAll(backlogDir, 0750); err != nil {
		t.Fatalf("failed to create backlog dir: %v", err)
	}
	sumarioContent := `# Weekly Summary
## Foco Atual & Backlog
### 🔴 P0
- [ ] Architecture RFC: Complete design doc
- [x] Done P0 task
### 🟡 P1
- [ ] Refactor telemetry module
`
	if err := os.WriteFile(filepath.Join(backlogDir, "sumario-semanal-atividades-impactos-2026.md"), []byte(sumarioContent), 0600); err != nil {
		t.Fatalf("failed to write backlog file: %v", err)
	}

	// 4. Setup sources/reunioes/ with matching and non-matching date
	reunioesDir := filepath.Join(tempVault, "sources", "reunioes")
	if err := os.MkdirAll(reunioesDir, 0750); err != nil {
		t.Fatalf("failed to create reunioes dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(reunioesDir, "2026-11-04-architecture-sync.md"), []byte("# Architecture Sync"), 0600); err != nil {
		t.Fatalf("failed to write meeting: %v", err)
	}
	if err := os.WriteFile(filepath.Join(reunioesDir, "2026-11-04-1on1-ada.md"), []byte("# 1:1 Ada"), 0600); err != nil {
		t.Fatalf("failed to write meeting: %v", err)
	}
	if err := os.WriteFile(filepath.Join(reunioesDir, "2026-11-03-past-meeting.md"), []byte("# Past Meeting"), 0600); err != nil {
		t.Fatalf("failed to write meeting: %v", err)
	}

	cfg := &config.Config{
		Vault: config.VaultConfig{
			Owner: "Ada Lovelace",
		},
	}

	// First run: Daily note should be created
	rep, err := daily.RunMorning(tempVault, "2026-11-04", cfg)
	if err != nil {
		t.Fatalf("RunMorning failed: %v", err)
	}

	if rep.Date != "2026-11-04" {
		t.Errorf("expected date 2026-11-04, got %s", rep.Date)
	}
	if rep.FilePath != "daily/2026-11-04.md" {
		t.Errorf("expected FilePath daily/2026-11-04.md, got %s", rep.FilePath)
	}
	if !rep.Created {
		t.Errorf("expected Created to be true on first run")
	}

	if len(rep.AutonomousActions) != 4 {
		t.Errorf("expected 4 autonomous actions, got %d: %v", len(rep.AutonomousActions), rep.AutonomousActions)
	}
	if len(rep.InboxItems) != 1 || rep.InboxItems[0] != "task1.md" {
		t.Errorf("expected inbox items [task1.md], got %v", rep.InboxItems)
	}
	if len(rep.Alerts) != 2 {
		t.Errorf("expected 2 alerts, got %d: %v", len(rep.Alerts), rep.Alerts)
	}
	if len(rep.Suggestions) != 2 {
		t.Errorf("expected 2 suggestions, got %d: %v", len(rep.Suggestions), rep.Suggestions)
	}
	if len(rep.BacklogP0) != 1 || rep.BacklogP0[0] != "Architecture RFC: Complete design doc" {
		t.Errorf("expected BacklogP0 [Architecture RFC: Complete design doc], got %v", rep.BacklogP0)
	}
	if len(rep.BacklogP1) != 1 || rep.BacklogP1[0] != "Refactor telemetry module" {
		t.Errorf("expected BacklogP1 [Refactor telemetry module], got %v", rep.BacklogP1)
	}
	if len(rep.IngestedMeetings) != 2 {
		t.Errorf("expected 2 ingested meetings, got %d: %v", len(rep.IngestedMeetings), rep.IngestedMeetings)
	}

	// Verify daily file on disk
	dailyPath := filepath.Join(tempVault, "daily", "2026-11-04.md")
	contentBytes, err := os.ReadFile(dailyPath)
	if err != nil {
		t.Fatalf("failed to read daily file: %v", err)
	}
	content := string(contentBytes)

	// Frontmatter assertions
	if !strings.Contains(content, "type: daily") {
		t.Errorf("content missing 'type: daily'")
	}
	if !strings.Contains(content, "date: 2026-11-04") {
		t.Errorf("content missing 'date: 2026-11-04'")
	}
	if !strings.Contains(content, "tempo-A_Estrategico: 0") {
		t.Errorf("content missing 'tempo-A_Estrategico: 0'")
	}
	if !strings.Contains(content, "tags: [daily]") && !strings.Contains(content, "- daily") {
		t.Errorf("content missing 'tags: [daily]'")
	}

	// Handoff assertions
	if !strings.Contains(content, "## 🤝 Handoff do Copiloto (Daneel ➔ Ada)") {
		t.Errorf("content missing Handoff title with owner Ada")
	}
	if !strings.Contains(content, "3 WikiLinks healed into relative Markdown links") {
		t.Errorf("content missing healed links autonomous action")
	}
	if !strings.Contains(content, "- [ ] `task1.md`") {
		t.Errorf("content missing inbox item checklist '- [ ] `task1.md`'")
	}
	if !strings.Contains(content, "1 broken links pending review") {
		t.Errorf("content missing broken links alert")
	}
	if !strings.Contains(content, "Bottleneck in review -> Automate linting") {
		t.Errorf("content missing metacognition suggestion")
	}
	if !strings.Contains(content, "Should we deprecate v1 API?") {
		t.Errorf("content missing handoff question suggestion")
	}

	// Backlog & Meetings assertions
	if !strings.Contains(content, "- [ ] Architecture RFC: Complete design doc") {
		t.Errorf("content missing P0 backlog item in focus section")
	}
	if !strings.Contains(content, "- [ ] Refactor telemetry module") {
		t.Errorf("content missing P1 backlog item in focus section")
	}
	if !strings.Contains(content, "[sources/reunioes/2026-11-04-1on1-ada.md](../sources/reunioes/2026-11-04-1on1-ada.md)") {
		t.Errorf("content missing relative link to ingested meeting 1on1")
	}

	// 5. Test idempotency: Calling RunMorning again on same date must NOT duplicate handoff
	rep2, err := daily.RunMorning(tempVault, "2026-11-04", cfg)
	if err != nil {
		t.Fatalf("RunMorning second run failed: %v", err)
	}
	if rep2.Created {
		t.Errorf("expected Created to be false on second run")
	}
	contentBytes2, err := os.ReadFile(dailyPath)
	if err != nil {
		t.Fatalf("failed to read daily file after second run: %v", err)
	}
	content2 := string(contentBytes2)
	if content2 != content {
		t.Errorf("content changed on second run; RunMorning is not idempotent")
	}

	// 6. Test insertion into pre-existing file lacking handoff
	preExistingFile := filepath.Join(tempVault, "daily", "2026-11-05.md")
	preExistingContent := `---
type: daily
date: 2026-11-05
tags: [daily]
---
# 📅 Diário de Bordo — 2026-11-05

## 🎯 Existing User Notes
- Some notes
`
	if err := os.WriteFile(preExistingFile, []byte(preExistingContent), 0600); err != nil {
		t.Fatalf("failed to write pre-existing daily file: %v", err)
	}

	rep3, err := daily.RunMorning(tempVault, "2026-11-05", cfg)
	if err != nil {
		t.Fatalf("RunMorning on pre-existing file failed: %v", err)
	}
	if rep3.Created {
		t.Errorf("expected Created = false for pre-existing file")
	}
	injectedBytes, err := os.ReadFile(preExistingFile)
	if err != nil {
		t.Fatalf("failed to read injected file: %v", err)
	}
	injectedContent := string(injectedBytes)
	if !strings.Contains(injectedContent, "## 🤝 Handoff do Copiloto (Daneel ➔ Ada)") {
		t.Errorf("pre-existing file missing injected Handoff block")
	}
	if !strings.Contains(injectedContent, "## 🎯 Existing User Notes") {
		t.Errorf("pre-existing file lost existing user notes")
	}
}

func TestRunEvening_UpdatesFrontmatterFromExplicitAnnotationsAndPreservesWhenNone(t *testing.T) {
	tempVault := t.TempDir()
	dailyDir := filepath.Join(tempVault, "daily")
	if err := os.MkdirAll(dailyDir, 0750); err != nil {
		t.Fatalf("failed to create daily dir: %v", err)
	}

	cfg := &config.Config{
		Vault: config.VaultConfig{
			Owner: "Alan Turing",
		},
	}

	// 1. Non-existent file should return error
	_, err := daily.RunEvening(tempVault, "2026-11-01", cfg)
	if err == nil {
		t.Fatalf("expected error for non-existent daily file, got nil")
	}

	// 2. Daily file WITH explicit time annotations
	fileWithAnnotations := filepath.Join(dailyDir, "2026-11-06.md")
	initialContent := `---
type: daily
date: 2026-11-06
custom_metadata: preserve_this_value
tempo-A_Estrategico: 0
tempo-B_ImpactoLocal: 0
tempo-C_GlueWork: 0
tempo-D_LowImpact: 0
tempo-E_Overhead: 0
tags: [daily]
---
# 📅 Diário de Bordo — 2026-11-06

## ⏱️ Notas de Tempo & Log de Atividades
- [A] 2.0h: Architecture review
- 1.5h [C] Mentoring 1:1
- [B] (1.5h) Implementation of search feature
- tempo-D 0.5h Email cleanup
- [E] 0.5h Incident triage overhead
- [A] 1,5h Additional architecture RFC work
- [ ] Task without duration [A]
`
	if err := os.WriteFile(fileWithAnnotations, []byte(initialContent), 0600); err != nil {
		t.Fatalf("failed to write initial daily file: %v", err)
	}

	rep, err := daily.RunEvening(tempVault, "2026-11-06", cfg)
	if err != nil {
		t.Fatalf("RunEvening failed: %v", err)
	}

	if !rep.Updated {
		t.Errorf("expected Updated to be true when annotations are found")
	}
	if rep.HoursByCategory["A"] != 3.5 {
		t.Errorf("expected A: 3.5, got %v", rep.HoursByCategory["A"])
	}
	if rep.HoursByCategory["B"] != 1.5 {
		t.Errorf("expected B: 1.5, got %v", rep.HoursByCategory["B"])
	}
	if rep.HoursByCategory["C"] != 1.5 {
		t.Errorf("expected C: 1.5, got %v", rep.HoursByCategory["C"])
	}
	if rep.HoursByCategory["D"] != 0.5 {
		t.Errorf("expected D: 0.5, got %v", rep.HoursByCategory["D"])
	}
	if rep.HoursByCategory["E"] != 0.5 {
		t.Errorf("expected E: 0.5, got %v", rep.HoursByCategory["E"])
	}
	if rep.TotalHours != 7.5 {
		t.Errorf("expected TotalHours: 7.5, got %v", rep.TotalHours)
	}

	// Verify frontmatter updated on disk while preserving metadata & body
	updatedBytes, err := os.ReadFile(fileWithAnnotations)
	if err != nil {
		t.Fatalf("failed to read updated daily file: %v", err)
	}
	updatedContent := string(updatedBytes)

	if !strings.Contains(updatedContent, "custom_metadata: preserve_this_value") {
		t.Errorf("custom_metadata was not preserved in frontmatter")
	}
	if !strings.Contains(updatedContent, "tempo-A_Estrategico: 3.5") {
		t.Errorf("tempo-A_Estrategico was not updated to 3.5 in %s", updatedContent)
	}
	if !strings.Contains(updatedContent, "tempo-B_ImpactoLocal: 1.5") {
		t.Errorf("tempo-B_ImpactoLocal was not updated to 1.5")
	}
	if !strings.Contains(updatedContent, "tempo-C_GlueWork: 1.5") {
		t.Errorf("tempo-C_GlueWork was not updated to 1.5")
	}
	if !strings.Contains(updatedContent, "tempo-D_LowImpact: 0.5") {
		t.Errorf("tempo-D_LowImpact was not updated to 0.5")
	}
	if !strings.Contains(updatedContent, "tempo-E_Overhead: 0.5") {
		t.Errorf("tempo-E_Overhead was not updated to 0.5")
	}
	if !strings.Contains(updatedContent, "- [A] 2.0h: Architecture review") {
		t.Errorf("markdown body was altered")
	}

	// 3. Daily file WITHOUT explicit time annotations: should preserve existing frontmatter values
	fileWithoutAnnotations := filepath.Join(dailyDir, "2026-11-07.md")
	unannotatedContent := `---
type: daily
date: 2026-11-07
tempo-A_Estrategico: 4
tempo-B_ImpactoLocal: 2
tempo-C_GlueWork: 1
tempo-D_LowImpact: 0
tempo-E_Overhead: 1
tags: [daily]
---
# 📅 Diário de Bordo — 2026-11-07

## ⏱️ Notas de Tempo & Log de Atividades
- General brainstorming without explicit tags
`
	if err := os.WriteFile(fileWithoutAnnotations, []byte(unannotatedContent), 0600); err != nil {
		t.Fatalf("failed to write unannotated daily file: %v", err)
	}

	rep2, err := daily.RunEvening(tempVault, "2026-11-07", cfg)
	if err != nil {
		t.Fatalf("RunEvening failed on unannotated file: %v", err)
	}

	if rep2.Updated {
		t.Errorf("expected Updated to be false when no annotations exist")
	}
	if rep2.HoursByCategory["A"] != 4.0 {
		t.Errorf("expected A to read 4.0 from existing frontmatter, got %v", rep2.HoursByCategory["A"])
	}
	if rep2.TotalHours != 8.0 {
		t.Errorf("expected TotalHours: 8.0, got %v", rep2.TotalHours)
	}

	// File on disk must remain unchanged (values not zeroed)
	unannotatedRead, err := os.ReadFile(fileWithoutAnnotations)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(unannotatedRead) != unannotatedContent {
		t.Errorf("unannotated file was unexpectedly modified on disk")
	}
}
