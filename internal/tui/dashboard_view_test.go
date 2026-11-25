package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	graphv1 "github.com/rjfonseca/daneel/api/v1"
	"github.com/rjfonseca/daneel/internal/dream"
	"github.com/rjfonseca/daneel/internal/ops/backlog"
	"github.com/rjfonseca/daneel/internal/ops/briefing"
)

func TestDashboardView_View_SingleFrictionSignalAndNarrowWidth(t *testing.T) {
	dv := &DashboardView{
		styles: NewStyles(),
		width:  120,
		height: 40,
		temporalData: &dream.TemporalWindowData{
			Percentages: map[string]float64{"A": 40},
			FrictionSignals: []dream.FrictionSignal{
				{Date: "2026-11-25", Line: "single signal"},
			},
		},
		backlogRep: &backlog.BacklogReport{
			Items: []backlog.BacklogItem{
				{
					Priority:    "P0",
					Title:       strings.Repeat("a", 50),
					Description: "details",
				},
			},
		},
	}

	out := dv.View()
	if !strings.Contains(out, "single signal") {
		t.Fatalf("expected View() output to contain %q, got:\n%s", "single signal", out)
	}

	dv.SetSize(30, 20)
	narrowOut := dv.View()
	if narrowOut == "" {
		t.Fatal("expected narrow View() output not to be empty")
	}
}

func TestPeopleView_View_SingleDailyMention(t *testing.T) {
	pv := NewPeopleView(t.TempDir(), NewStyles())
	pv.SetSize(120, 40)
	pv.peopleList = []personItem{{Name: "Alice", Slug: "alice"}}
	pv.currentBrief = &briefing.PersonProfile{
		Name:                "Alice",
		RecentDailyMentions: []string{"2026-11-25: aligned on IMP"},
	}

	out := pv.View()
	if !strings.Contains(out, "aligned on IMP") {
		t.Fatalf("expected PeopleView.View() output to contain %q, got:\n%s", "aligned on IMP", out)
	}
}

func TestDashboardView_TaxonomyPercentagesBacklogFilterAndHandoff(t *testing.T) {
	dv := NewDashboardView(t.TempDir(), NewStyles())
	dv.SetSize(120, 30)

	var items []*graphv1.BacklogItemProto
	items = append(items, &graphv1.BacklogItemProto{
		Priority: "P0", Title: "Completed legacy rollout", Description: "Done", Completed: true,
	})
	for i := 0; i < 20; i++ {
		items = append(items, &graphv1.BacklogItemProto{
			Priority: "P1", Title: fmt.Sprintf("Open priority task %02d", i), Description: "Active work", Completed: false,
		})
	}

	dv, _ = dv.Update(dashboardLoadedMsg{
		summary: &graphv1.GetDashboardSummaryResponse{
			WindowDays:   7,
			DaysWithLogs: 4,
			TotalHours:   16.0,
			Taxonomy: []*graphv1.TaxonomyMetricItem{
				{Id: "A_Estrategico", Name: "Estratégico", Hours: 10.0, Percentage: 62.5, TargetMin: 30},
				{Id: "B_ImpactoLocal", Name: "Impacto Local", Hours: 6.0, Percentage: 37.5, TargetMin: 30},
			},
			Handoff: &graphv1.HandoffDigest{
				Date:              "2026-11-25",
				DreamCyclesCount:  2,
				AutonomousActions: []string{"Synced 4 timelines"},
				Alerts:            []string{"Review pending inbox"},
			},
			TotalNotes:   150,
			TotalLinks:   300,
			OrphanNotes:  5,
			BacklogItems: items,
		},
	})

	out := dv.View()
	if !strings.Contains(out, "62.5%") || !strings.Contains(out, "37.5%") {
		t.Errorf("expected real taxonomy percentages 62.5%% and 37.5%%, got:\n%s", out)
	}
	if !strings.Contains(out, "Morning Handoff") || !strings.Contains(out, "REM Cycles: 2") {
		t.Errorf("expected Morning Handoff digest in DashboardView, got:\n%s", out)
	}
	if strings.Contains(out, "Completed legacy rollout") {
		t.Errorf("expected completed backlog items to be hidden by default, got:\n%s", out)
	}
	if !strings.Contains(out, "Open priority task 00") {
		t.Errorf("expected open backlog item to be visible, got:\n%s", out)
	}
	if gotH := lipgloss.Height(out); gotH > 30 {
		t.Errorf("expected DashboardView height <= 30, got %d", gotH)
	}

	// Toggle completed items with 'c'
	dv, _ = dv.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	outWithDone := dv.View()
	if !strings.Contains(outWithDone, "Completed legacy rollout") {
		t.Errorf("expected completed backlog item after pressing 'c', got:\n%s", outWithDone)
	}
}
