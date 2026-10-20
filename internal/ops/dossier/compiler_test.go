//nolint:revive // package comment not required
package dossier

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAVDCompiler(t *testing.T) {
	tempDir := t.TempDir()

	dailyDir := filepath.Join(tempDir, "daily")
	carreiraDir := filepath.Join(tempDir, "meta", "carreira")
	pessoasDir := filepath.Join(tempDir, "meta", "pessoas")
	avaliacoesDir := filepath.Join(tempDir, "meta", "avaliacoes")

	_ = os.MkdirAll(dailyDir, 0750)
	_ = os.MkdirAll(carreiraDir, 0750)
	_ = os.MkdirAll(pessoasDir, 0750)
	_ = os.MkdirAll(avaliacoesDir, 0750)

	// Create a daily note with frontmatter telemetry
	dailyFile := filepath.Join(dailyDir, "2026-08-10.md")
	dailyContent := `---
type: daily
tempo-A_Estrategico: 3.5
tempo-B_ImpactoLocal: 2.0
tempo-C_GlueWork: 1.5
tempo-E_Overhead: 0.5
---
# 2026-08-10

- [A] Trabalhei na RFC de Arquitetura do Daneel.
- [B] Fix no bug de rate limiting.
- [C] 1:1 com [[matheus-constanzo]].
`
	_ = os.WriteFile(dailyFile, []byte(dailyContent), 0600)

	// Create meta individual
	metaFile := filepath.Join(carreiraDir, "meta-individual-2026-plataforma.md")
	metaContent := `---
type: carreira
description: Construção da Plataforma de Agentes
---
# Meta 2026: Plataforma de Agentes
`
	_ = os.WriteFile(metaFile, []byte(metaContent), 0600)

	// Create person profile
	personFile := filepath.Join(pessoasDir, "matheus-constanzo.md")
	personContent := `---
type: person
---
# 👤 Matheus Constanzo

**Cargo / Papel:** Staff Engineer
**Squad / Tribo:** Platform

## 🤝 Histórico de Interações
- **2026-08-10** - Alinhamento técnico de arquitetura.
- **2026-07-15** - Mentoria e revisão de PRD.
`
	_ = os.WriteFile(personFile, []byte(personContent), 0600)

	comp := NewCompiler(tempDir)

	outPath := filepath.Join(avaliacoesDir, "dossie-test.md")
	dossier, err := comp.Compile(2026, 0, outPath)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if dossier.Telemetry.TotalHours != 7.5 {
		t.Errorf("expected 7.5 total hours, got %.1f", dossier.Telemetry.TotalHours)
	}
	if len(dossier.Metas) != 1 {
		t.Errorf("expected 1 meta, got %d", len(dossier.Metas))
	}
	if len(dossier.PeopleImpacted) != 1 {
		t.Errorf("expected 1 person impacted, got %d", len(dossier.PeopleImpacted))
	}
	if dossier.PeopleImpacted[0].InteractionsCount != 2 {
		t.Errorf("expected 2 interactions, got %d", dossier.PeopleImpacted[0].InteractionsCount)
	}

	if _, err := os.Stat(outPath); os.IsNotExist(err) {
		t.Errorf("dossier output file was not created: %s", outPath)
	}
}
