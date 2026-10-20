//nolint:revive // package comment not required
package tracking

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDailyTracking(t *testing.T) {
	tempDir := t.TempDir()

	dailyDir := filepath.Join(tempDir, "daily")
	_ = os.MkdirAll(dailyDir, 0750)

	dailyFile := filepath.Join(dailyDir, "2026-08-18.md")
	content := `---
data: 2026-08-18
tempo-A_Estrategico: 2.5
tempo-B_ImpactoLocal: 1.5
tempo-C_GlueWork: 1.0
tempo-D_LowImpact: 0
tempo-E_Overhead: 0.5
tags:
  - daily
---
# Diário de Bordo: 18 de Agosto de 2026

## 🗓️ Log de Atividades
- Reunião de arquitetura do Gateway API.
- Code review no SDK de Golang.

## 👥 Alinhamentos e Interações
- [Jane Doe](../notes/pessoas/jane-doe.md) - Alinhamento de infra.

## 🧠 Conhecimento & Descobertas do Dia
- [Kubernetes Gateway API](../notes/infra/kubernetes-gateway-api.md)
`
	_ = os.WriteFile(dailyFile, []byte(content), 0600)

	log, err := GetDailyLog(tempDir, "2026-08-18")
	if err != nil {
		t.Fatalf("GetDailyLog failed: %v", err)
	}

	if log.TempoA != 2.5 {
		t.Errorf("expected TempoA 2.5, got %v", log.TempoA)
	}
	if log.TempoB != 1.5 {
		t.Errorf("expected TempoB 1.5, got %v", log.TempoB)
	}
	if log.TotalHours != 5.5 {
		t.Errorf("expected TotalHours 5.5, got %v", log.TotalHours)
	}
	if len(log.Activities) != 2 {
		t.Errorf("expected 2 activities, got %d", len(log.Activities))
	}
	if len(log.Alignments) != 1 {
		t.Errorf("expected 1 alignment, got %d", len(log.Alignments))
	}
}

func TestCalculateLeverageRate(t *testing.T) {
	tests := []struct {
		tempoA   float64
		tempoC   float64
		total    float64
		expected float64
	}{
		{tempoA: 0, tempoC: 0, total: 0, expected: 0},
		{tempoA: 2.0, tempoC: 1.5, total: 7.0, expected: 0.5},
		{tempoA: 4.0, tempoC: 2.0, total: 6.0, expected: 1.0},
		{tempoA: 5.0, tempoC: 2.0, total: 5.0, expected: 1.0}, // clamped to 1.0
	}

	for _, tc := range tests {
		got := CalculateLeverageRate(tc.tempoA, tc.tempoC, tc.total)
		if got != tc.expected {
			t.Errorf("CalculateLeverageRate(%.1f, %.1f, %.1f) = %v, expected %v",
				tc.tempoA, tc.tempoC, tc.total, got, tc.expected)
		}
	}
}

func TestClassifyPeriodDiagnosis(t *testing.T) {
	// Zero hours
	if diag := ClassifyPeriodDiagnosis(PeriodSummary{TotalHours: 0}); diag != "💤 No Activity Logged" {
		t.Errorf("expected No Activity Logged, got %s", diag)
	}

	// Healthy leverage: LeverageRate >= 0.50 and TempoA / Total >= 0.35
	pStrategic := PeriodSummary{
		TotalHours:   10,
		TempoA:       4.0, // 40%
		TempoC:       2.0, // 20% -> LeverageRate = 60%
		LeverageRate: 0.60,
	}
	if diag := ClassifyPeriodDiagnosis(pStrategic); diag != "🚀 Strategic Multiplication" {
		t.Errorf("expected Strategic Multiplication, got %s", diag)
	}

	// Firefighting / Local Impact: TempoB / Total >= 0.45
	pLocal := PeriodSummary{
		TotalHours:   10,
		TempoB:       5.0, // 50%
		TempoA:       2.0,
		LeverageRate: 0.20,
	}
	if diag := ClassifyPeriodDiagnosis(pLocal); diag != "🔥 Local Reactivity & Firefighting" {
		t.Errorf("expected Local Reactivity, got %s", diag)
	}

	// High Glue & Alignment: TempoC / Total >= 0.35
	pGlue := PeriodSummary{
		TotalHours:   10,
		TempoC:       4.0, // 40%
		TempoA:       0.5,
		LeverageRate: 0.45,
	}
	if diag := ClassifyPeriodDiagnosis(pGlue); diag != "🤝 High Glue & Alignment Load" {
		t.Errorf("expected High Glue & Alignment Load, got %s", diag)
	}

	// High Overhead: TempoE / Total >= 0.25
	pOverhead := PeriodSummary{
		TotalHours:   10,
		TempoE:       3.0, // 30%
		TempoA:       2.0,
		LeverageRate: 0.20,
	}
	if diag := ClassifyPeriodDiagnosis(pOverhead); diag != "🤝 High Glue & Alignment Load" {
		t.Errorf("expected High Glue & Alignment Load, got %s", diag)
	}

	// Balanced
	pBalanced := PeriodSummary{
		TotalHours:   10,
		TempoA:       3.0, // 30% (< 35%)
		TempoB:       3.0, // 30% (< 45%)
		TempoC:       2.0, // 20% (< 35%)
		TempoD:       1.0,
		TempoE:       1.0, // 10% (< 25%)
		LeverageRate: 0.50,
	}
	if diag := ClassifyPeriodDiagnosis(pBalanced); diag != "⚖️ Balanced Operational Focus" {
		t.Errorf("expected Balanced Operational Focus, got %s", diag)
	}
}

func TestTemporalAggregation(t *testing.T) {
	tempDir := t.TempDir()
	dailyDir := filepath.Join(tempDir, "daily")
	_ = os.MkdirAll(dailyDir, 0750)

	// Create daily logs across 2 weeks in September 2026:
	// 2026-09-21 (Mon, W39)
	// 2026-09-22 (Tue, W39)
	// 2026-09-14 (Mon, W38)
	// 2026-08-25 (Tue, W35)
	files := map[string]string{
		"2026-09-21.md": `---
data: 2026-09-21
tempo-A_Estrategico: 4.0
tempo-B_ImpactoLocal: 1.0
tempo-C_GlueWork: 2.0
tempo-D_LowImpact: 0.5
tempo-E_Overhead: 0.5
---
`,
		"2026-09-22.md": `---
data: 2026-09-22
tempo-A_Estrategico: 3.0
tempo-B_ImpactoLocal: 2.0
tempo-C_GlueWork: 1.0
tempo-D_LowImpact: 0.0
tempo-E_Overhead: 1.0
---
`,
		"2026-09-14.md": `---
data: 2026-09-14
tempo-A_Estrategico: 1.0
tempo-B_ImpactoLocal: 5.0
tempo-C_GlueWork: 1.0
tempo-D_LowImpact: 0.0
tempo-E_Overhead: 1.0
---
`,
		"2026-08-25.md": `---
data: 2026-08-25
tempo-A_Estrategico: 2.0
tempo-B_ImpactoLocal: 2.0
tempo-C_GlueWork: 2.0
tempo-D_LowImpact: 1.0
tempo-E_Overhead: 1.0
---
`,
	}

	for name, content := range files {
		_ = os.WriteFile(filepath.Join(dailyDir, name), []byte(content), 0600)
	}

	// Test AggregateByWeek
	weeks, err := AggregateByWeek(tempDir, 4)
	if err != nil {
		t.Fatalf("AggregateByWeek failed: %v", err)
	}
	if len(weeks) != 3 {
		t.Fatalf("expected 3 weeks, got %d", len(weeks))
	}

	// Should be chronologically ascending: W35, W38, W39
	if weeks[0].PeriodKey != "2026-W35" || weeks[1].PeriodKey != "2026-W38" || weeks[2].PeriodKey != "2026-W39" {
		t.Errorf("unexpected week order: %v, %v, %v", weeks[0].PeriodKey, weeks[1].PeriodKey, weeks[2].PeriodKey)
	}

	// Verify W39 aggregation (2 days: 21 and 22)
	w39 := weeks[2]
	if w39.DaysCount != 2 {
		t.Errorf("expected 2 days for W39, got %d", w39.DaysCount)
	}
	if w39.TempoA != 7.0 || w39.TempoB != 3.0 || w39.TempoC != 3.0 || w39.TempoD != 0.5 || w39.TempoE != 1.5 {
		t.Errorf("unexpected hours for W39: %+v", w39)
	}
	if w39.TotalHours != 15.0 {
		t.Errorf("expected 15.0 total hours, got %v", w39.TotalHours)
	}
	// Leverage rate: (7.0 + 3.0) / 15.0 = 10/15 = 0.666...
	if w39.LeverageRate < 0.66 || w39.LeverageRate > 0.67 {
		t.Errorf("expected leverage rate ~0.667, got %v", w39.LeverageRate)
	}
	if w39.Diagnosis != "🚀 Strategic Multiplication" {
		t.Errorf("expected Strategic Multiplication for W39, got %s", w39.Diagnosis)
	}

	// Verify W38 (Local reactivity: 5h local out of 8h total = 62.5%)
	w38 := weeks[1]
	if w38.Diagnosis != "🔥 Local Reactivity & Firefighting" {
		t.Errorf("expected Local Reactivity for W38, got %s", w38.Diagnosis)
	}

	// Test AggregateByMonth
	months, err := AggregateByMonth(tempDir, 6)
	if err != nil {
		t.Fatalf("AggregateByMonth failed: %v", err)
	}
	if len(months) != 2 {
		t.Fatalf("expected 2 months (Aug and Sep), got %d", len(months))
	}
	if months[0].PeriodKey != "2026-08" || months[1].PeriodKey != "2026-09" {
		t.Errorf("unexpected month order: %v, %v", months[0].PeriodKey, months[1].PeriodKey)
	}
	sep := months[1]
	if sep.DaysCount != 3 {
		t.Errorf("expected 3 days in Sep, got %d", sep.DaysCount)
	}
	if sep.Label != "September 2026" {
		t.Errorf("expected 'September 2026', got %s", sep.Label)
	}
}
