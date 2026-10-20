//nolint:revive,misspell // package comment not required, portuguese templates
package dream

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/rjfonseca/daneel/internal/safefile"
)

// StageTiming records timing and item throughput for a dream cycle stage.
type StageTiming struct {
	Name        string `json:"name"`
	DurationMs  int64  `json:"duration_ms"`
	ItemsCount  int    `json:"items_count"`
	Description string `json:"description"`
}

// SynthesisDraft represents a generated cross-domain knowledge connection note.
type SynthesisDraft struct {
	Title         string   `json:"title"`
	Slug          string   `json:"slug"`
	Category      string   `json:"category"`
	SourceDomains []string `json:"source_domains"`
	SourceNotes   []string `json:"source_notes"`
	DraftContent  string   `json:"draft_content"`
	CreatedPath   string   `json:"created_path,omitempty"`
}

// MetacognitiveInsight represents an introspective reflection on bottlenecks, tool performance, or skill refactoring.
type MetacognitiveInsight struct {
	Category        string `json:"category"` // "bottleneck", "skill_refactoring", "synthesis", "performance"
	Observation     string `json:"observation"`
	Impact          string `json:"impact"`
	SuggestedAction string `json:"suggested_action"`
}

// MetacognitiveReport summarizes phase 4 & 5 execution and agent self-evolution metrics.
type MetacognitiveReport struct {
	TotalDurationMs  int64                  `json:"total_duration_ms"`
	Timings          []StageTiming          `json:"timings"`
	Bottlenecks      []string               `json:"bottlenecks"`
	Insights         []MetacognitiveInsight `json:"insights"`
	Syntheses        []SynthesisDraft       `json:"syntheses"`
	EvolutionLogPath string                 `json:"evolution_log_path"`
}

// RunMetacognitivePass executes Phase 4 (introspective bottleneck analysis & skill refactoring proposals).
func RunMetacognitivePass(vaultRoot string, report *FullDreamReport, timings []StageTiming) *MetacognitiveReport {
	metaRep := &MetacognitiveReport{
		Timings:          timings,
		Bottlenecks:      []string{},
		Insights:         []MetacognitiveInsight{},
		Syntheses:        []SynthesisDraft{},
		EvolutionLogPath: "meta/guias/agente-evolucao.md",
	}

	var totalMs int64
	for _, t := range timings {
		totalMs += t.DurationMs
	}
	metaRep.TotalDurationMs = totalMs

	// 1. Analyze Bottlenecks
	if report.BrokenLinksCount > 0 {
		msg := fmt.Sprintf("%d broken relative links detected across graph nodes, causing syntactic entropy", report.BrokenLinksCount)
		metaRep.Bottlenecks = append(metaRep.Bottlenecks, msg)
		metaRep.Insights = append(metaRep.Insights, MetacognitiveInsight{
			Category:        "bottleneck",
			Observation:     msg,
			Impact:          "Slows graph traversal and risks dead ends in agent navigation",
			SuggestedAction: "Run 'daneel tidy --dry-run=false' or invoke automated link healing in next cycle",
		})
	}

	if len(report.InboxTriaged) >= 10 {
		msg := fmt.Sprintf("High inbox density (%d unprocessed drafts in inbox/)", len(report.InboxTriaged))
		metaRep.Bottlenecks = append(metaRep.Bottlenecks, msg)
		metaRep.Insights = append(metaRep.Insights, MetacognitiveInsight{
			Category:        "skill_refactoring",
			Observation:     msg,
			Impact:          "Nocturnal batch execution accumulates processing delay",
			SuggestedAction: "Promote draft meeting processor to an asynchronous event hook or webhook",
		})
	}

	if report.TemporalAnalysis != nil && len(report.TemporalAnalysis.DriftAlerts) > 0 {
		for _, alert := range report.TemporalAnalysis.DriftAlerts {
			metaRep.Insights = append(metaRep.Insights, MetacognitiveInsight{
				Category:        "performance",
				Observation:     alert,
				Impact:          "Staff Engineer strategic leverage deviating from calibrated quarterly targets",
				SuggestedAction: "Rebalance week activities to favor strategic architecture over glue work",
			})
		}
	}

	if len(report.MissingMOCs) > 0 {
		msg := fmt.Sprintf("%d dense categories lack canonical Map of Content (MOC)", len(report.MissingMOCs))
		metaRep.Bottlenecks = append(metaRep.Bottlenecks, msg)
		metaRep.Insights = append(metaRep.Insights, MetacognitiveInsight{
			Category:        "bottleneck",
			Observation:     msg,
			Impact:          "Reduces agent retrieval speed when discovering domain entry points",
			SuggestedAction: "Generate missing MOCs via 'daneel tidy'",
		})
	}

	// 2. Skill Refactoring Insights based on repetitive patterns
	learnings := LoadLearnings(vaultRoot)
	if len(learnings.FeedbackHistory) > 5 {
		metaRep.Insights = append(metaRep.Insights, MetacognitiveInsight{
			Category:        "skill_refactoring",
			Observation:     fmt.Sprintf("%d cumulative user calibrations in agent feedback history", len(learnings.FeedbackHistory)),
			Impact:          "Frequent micro-corrections indicate prompt or skill divergence",
			SuggestedAction: "Consolidate active rules from meta/guias/aprendizados-agente.md into root AGENTS.md system instructions",
		})
	}

	return metaRep
}

// InduceKnowledgeSyntheses executes Phase 5 (active Zettelkasten cross-domain synthesis).
func InduceKnowledgeSyntheses(vaultRoot string, report *FullDreamReport, autoFix bool, dryRun bool) ([]SynthesisDraft, error) {
	var drafts []SynthesisDraft

	// Check if emerging concepts or cross-domain opportunities exist
	if len(report.EmergingConcepts) == 0 && (report.TemporalAnalysis == nil || len(report.TemporalAnalysis.TopSystems) < 2) {
		return drafts, nil
	}

	// Strategy A: Connect emerging concept with active systems
	for _, ec := range report.EmergingConcepts {
		title := fmt.Sprintf("Síntese: %s e Arquitetura de Sistemas", ec.Title)
		slug := slugifyText(ec.Title) + "-sintese-arquitetura"
		targetDir := filepath.Join(vaultRoot, "notes", "sinteses")
		relPath := filepath.Join("notes", "sinteses", slug+".md")

		// If notes/sinteses does not exist, place in inbox/ as candidate
		if _, err := os.Stat(targetDir); os.IsNotExist(err) {
			relPath = filepath.Join("inbox", "sintese-"+slug+".md")
		}

		content := fmt.Sprintf(`---
title: "%s"
type: sintese
status: draft
draft: true
tags:
  - sintese
  - zettelkasten
  - arquitetura
---

# 🌐 %s

Síntese ativa de conhecimento induzida autonomamente pelo Daneel Dream Cycle a partir do conceito emergente '%s'.

## 1. Contexto de Origem
%s

## 2. Pontes e Implicações Estruturais
- Cruzamento com a base de conhecimento de arquitetura e sistemas.
- Avaliar aplicabilidade prática no ecossistema local.

## 3. Próximos Passos
- [ ] Revisar e validar conexões induzidas.
- [ ] Promover para 'notes/arquitetura/' ou integrar no MOC temático.
`, title, title, ec.Title, ec.Context)

		draft := SynthesisDraft{
			Title:         title,
			Slug:          slug,
			Category:      "notes/sinteses",
			SourceDomains: []string{"daily", "notes/arquitetura"},
			SourceNotes:   []string{ec.Title},
			DraftContent:  content,
		}

		if autoFix && !dryRun {
			absTarget := filepath.Join(vaultRoot, relPath)
			if _, err := os.Stat(absTarget); os.IsNotExist(err) {
				if err := safefile.WriteFile(vaultRoot, absTarget, []byte(content), 0600); err != nil {
					return drafts, fmt.Errorf("save synthesis: %w", err)
				}
				draft.CreatedPath = relPath
			}
		}

		drafts = append(drafts, draft)
		if len(drafts) >= 2 {
			break
		}
	}

	// Strategy B: Cross-Domain Pair Synthesis if TopSystems >= 2 and drafts < 2
	if len(drafts) < 2 && report.TemporalAnalysis != nil && len(report.TemporalAnalysis.TopSystems) >= 2 {
		sys1 := report.TemporalAnalysis.TopSystems[0]
		sys2 := report.TemporalAnalysis.TopSystems[1]
		title := fmt.Sprintf("Síntese Arquitetural: Interoperabilidade %s e %s", strings.Title(sys1), strings.Title(sys2)) //nolint:staticcheck
		slug := fmt.Sprintf("%s-%s-interoperabilidade", slugifyText(sys1), slugifyText(sys2))
		targetDir := filepath.Join(vaultRoot, "notes", "sinteses")
		relPath := filepath.Join("notes", "sinteses", slug+".md")

		if _, err := os.Stat(targetDir); os.IsNotExist(err) {
			relPath = filepath.Join("inbox", "sintese-"+slug+".md")
		}

		content := fmt.Sprintf(`---
title: "%s"
type: sintese
status: draft
draft: true
tags:
  - sintese
  - zettelkasten
  - interoperabilidade
---

# 🔗 %s

Síntese cruzada induzida entre os dois sistemas de maior volume de interação recente: **%s** e **%s**.

## 1. Escopo de Interoperabilidade
Análise de acoplamento, contratos e protocolos compartilhados entre %s e %s.

## 2. Pontos de Fricção e Oportunidades
- Identificar dependências circulares ou duplicações de domínio.
- Alinhar esquemas e observabilidade compartilhada.
`, title, title, sys1, sys2, sys1, sys2)

		draft := SynthesisDraft{
			Title:         title,
			Slug:          slug,
			Category:      "notes/sinteses",
			SourceDomains: []string{"notes/sistemas", "notes/arquitetura"},
			SourceNotes:   []string{sys1, sys2},
			DraftContent:  content,
		}

		if autoFix && !dryRun {
			absTarget := filepath.Join(vaultRoot, relPath)
			if _, err := os.Stat(absTarget); os.IsNotExist(err) {
				if err := safefile.WriteFile(vaultRoot, absTarget, []byte(content), 0600); err != nil {
					return drafts, fmt.Errorf("save synthesis: %w", err)
				}
				draft.CreatedPath = relPath
			}
		}

		drafts = append(drafts, draft)
	}

	return drafts, nil
}

// AppendEvolutionLog appends the dream cycle execution entry to meta/guias/agente-evolucao.md.
func AppendEvolutionLog(vaultRoot string, date string, report *FullDreamReport) error {
	metaDir := filepath.Join(vaultRoot, "meta", "guias")

	logPath := filepath.Join(metaDir, "agente-evolucao.md")
	var existingContent string

	if data, err := os.ReadFile(logPath); err == nil { //nolint:gosec
		existingContent = string(data)
	} else {
		existingContent = `# 🧬 Diário de Evolução do Agente (Metacognitive Evolution Log)

Registro histórico e introspectivo das execuções do Dream Cycle, métricas de eficiência, gargalos de performance e refatorações de skills.

---
`
	}

	var sb strings.Builder
	totalMs := int64(0)
	if report.Metacognition != nil {
		totalMs = report.Metacognition.TotalDurationMs
	}

	fmt.Fprintf(&sb, "\n## [%s] Ciclo REM Autônomo — Execução em %dms\n", date, totalMs)
	fmt.Fprintf(&sb, "- **Data de Referência:** %s (Executado em: %s)\n", date, time.Now().Format("15:04:05"))
	fmt.Fprintf(&sb, "- **Saúde do Grafo:** %d arquivos escaneados, %d links corrigidos, %d links quebrados.\n",
		report.TotalFilesScanned, report.FixedLinksCount, report.BrokenLinksCount)

	if report.Metacognition != nil {
		if len(report.Metacognition.Timings) > 0 {
			sb.WriteString("- **Perfil de Execução por Fase:**\n")
			for _, t := range report.Metacognition.Timings {
				fmt.Fprintf(&sb, "  • %s: %dms (%d itens)\n", t.Name, t.DurationMs, t.ItemsCount)
			}
		}

		if len(report.Metacognition.Bottlenecks) > 0 {
			sb.WriteString("- **Gargalos Detectados:**\n")
			for _, b := range report.Metacognition.Bottlenecks {
				fmt.Fprintf(&sb, "  • ⚠️ %s\n", b)
			}
		} else {
			sb.WriteString("- **Gargalos Detectados:** Nenhum gargalo crítico identificado. Execução nominal.\n")
		}

		if len(report.Metacognition.Syntheses) > 0 {
			sb.WriteString("- **Sínteses Ativas Induzidas:**\n")
			for _, s := range report.Metacognition.Syntheses {
				target := s.CreatedPath
				if target == "" {
					target = s.Slug
				}
				fmt.Fprintf(&sb, "  • 🌐 [[%s]] — *%s*\n", target, s.Title)
			}
		}

		if len(report.Metacognition.Insights) > 0 {
			sb.WriteString("- **Insights Metacognitivos & Refatoração:**\n")
			for _, in := range report.Metacognition.Insights {
				fmt.Fprintf(&sb, "  • *[%s]* %s ➔ %s\n", strings.ToUpper(in.Category), in.Observation, in.SuggestedAction)
			}
		}
	}

	sb.WriteString("\n---\n")

	newContent := existingContent + sb.String()
	return safefile.WriteFile(vaultRoot, logPath, []byte(newContent), 0600)
}

func slugifyText(s string) string {
	reg := regexp.MustCompile(`[^a-zA-Z0-9]+`)
	slug := reg.ReplaceAllString(strings.ToLower(s), "-")
	return strings.Trim(slug, "-")
}
