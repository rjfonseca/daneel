package tui

import (
	"strings"
	"testing"

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
