//nolint:revive // package comment not required
package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/rjfonseca/daneel/internal/config"
	"github.com/rjfonseca/daneel/internal/dream"
	"github.com/rjfonseca/daneel/internal/lore"
)

// RoutinesView displays REM dream cycle results and interactive handoff questions.
type RoutinesView struct {
	vaultRoot     string
	styles        Styles
	report        *dream.FullDreamReport
	engine        *dream.DreamEngine
	selectedIndex int
	statusMsg     string
	width         int
	height        int
}

// NewRoutinesView creates a new dream cycle view.
func NewRoutinesView(vaultRoot string, styles Styles) *RoutinesView {
	dv := &RoutinesView{
		vaultRoot:     vaultRoot,
		styles:        styles,
		engine:        dream.NewDreamEngine(vaultRoot),
		selectedIndex: 0,
	}
	dv.LoadCache()
	return dv
}

// LoadCache loads the latest dream report from cache.
func (v *RoutinesView) LoadCache() {
	cacheFile := filepath.Join(config.ResolveVaultCacheDir(v.vaultRoot), "dream_handoff.json")
	data, err := os.ReadFile(cacheFile) //nolint:gosec // intentional
	if err == nil {
		var rep dream.FullDreamReport
		if err := json.Unmarshal(data, &rep); err == nil {
			v.report = &rep
			return
		}
	}

	// Fallback to fresh run if no cache exists
	v.RunDream()
}

// RunDream executes the dream cycle.
func (v *RoutinesView) RunDream() {
	today := time.Now().Format("2006-01-02")
	rep, err := v.engine.RunFullCycle(today, true, false)
	if err == nil {
		v.report = rep
		v.statusMsg = "✨ Dream Cycle completed successfully!"
	}
}

// Init initializes the view.
func (v *RoutinesView) Init() tea.Cmd {
	return nil
}

// Update handles tea events for the dream view.
func (v *RoutinesView) Update(msg tea.Msg) (*RoutinesView, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "d", "r":
			v.statusMsg = "⏳ Running Dream Cycle..."
			v.RunDream()
			return v, nil
		case "up", "k":
			if v.selectedIndex > 0 {
				v.selectedIndex--
			}
			return v, nil
		case "down", "j":
			if v.report != nil && v.selectedIndex < len(v.report.HandoffQuestions)-1 {
				v.selectedIndex++
			}
			return v, nil
		}
	}
	return v, nil
}

// SetSize updates dimensions.
func (v *RoutinesView) SetSize(w, h int) {
	v.width = w
	v.height = h
}

// View renders the Dream Handoff screen.
func (v *RoutinesView) View() string {
	if v.report == nil {
		return "Loading Dream Cycle Report..."
	}

	_, rightWidth, _ := CalculateLayoutDimensions(v.width, v.height, 0, 0.48)

	var leftSB strings.Builder
	leftSB.WriteString(v.styles.PanelTitle.Render(fmt.Sprintf("🌙 REM Sleep & Consolidation [%s]", v.report.Date)) + "\n\n")

	leftSB.WriteString(lipgloss.NewStyle().Bold(true).Foreground(v.styles.Theme.Primary).Render("🩹 Graph Integrity & Self-Healing:\n"))
	fmt.Fprintf(&leftSB, "   • WikiLinks Normalized: %s\n", v.styles.BadgeClean.Render(fmt.Sprintf("%d", v.report.FixedLinksCount)))
	fmt.Fprintf(&leftSB, "   • Broken / Orphan Links: %d\n", v.report.BrokenLinksCount)
	fmt.Fprintf(&leftSB, "   • MOCs Synchronized: %d notes connected\n\n", len(v.report.MissingMOCs))

	if v.report.WorkspaceSummary != nil {
		leftSB.WriteString(lipgloss.NewStyle().Bold(true).Foreground(v.styles.Theme.Info).Render("🗺️ Workspace (~/Work):\n"))
		fmt.Fprintf(&leftSB, "   • Total: %d repositories (%d clean / %d modified)\n\n",
			v.report.WorkspaceSummary.TotalRepos,
			v.report.WorkspaceSummary.CleanCount,
			v.report.WorkspaceSummary.DirtyCount)
	}

	if len(v.report.InboxTriaged) > 0 {
		leftSB.WriteString(lipgloss.NewStyle().Bold(true).Foreground(v.styles.Theme.Warning).Render("📥 Inbox Triage (Learned Heuristics):\n"))
		for _, it := range v.report.InboxTriaged {
			badge := "🟢"
			switch it.Confidence {
			case "MEDIUM":
				badge = "🟡"
			case "LOW":
				badge = "🔴"
			}
			fmt.Fprintf(&leftSB, "   %s `%s` ➔ `%s` (%d%%)\n", badge, it.File, it.SuggestedCategory, it.Score)
		}
		leftSB.WriteString("\n")
	}

	if len(v.report.EmergingConcepts) > 0 {
		leftSB.WriteString(lipgloss.NewStyle().Bold(true).Foreground(v.styles.Theme.Success).Render("🌱 Discoveries & Emerging Concepts:\n"))
		for _, ec := range v.report.EmergingConcepts {
			fmt.Fprintf(&leftSB, "   • `%s`\n", ec.Title)
		}
	}

	if v.statusMsg != "" {
		leftSB.WriteString("\n" + v.styles.StatusMsg.Render(v.statusMsg) + "\n")
	}

	var rightSB strings.Builder
	rightSB.WriteString(v.styles.PanelTitle.Render("❓ Morning Handoff & Strategic Decisions") + "\n\n")

	if len(v.report.HandoffQuestions) > 0 {
		for i, q := range v.report.HandoffQuestions {
			typeBadge := lipgloss.NewStyle().Background(v.styles.Theme.SurfaceLight).Foreground(v.styles.Theme.Primary).Bold(true).Padding(0, 1).Render(strings.ToUpper(q.Type))
			line := fmt.Sprintf("%s %s", typeBadge, q.Question)

			if i == v.selectedIndex {
				rightSB.WriteString(v.styles.ListItemSelected.Width(rightWidth - 4).Render("▸ " + line))
			} else {
				rightSB.WriteString(v.styles.ListItem.Width(rightWidth - 4).Render("  " + line))
			}
			rightSB.WriteString("\n\n")
		}

		if v.selectedIndex < len(v.report.HandoffQuestions) {
			curQ := v.report.HandoffQuestions[v.selectedIndex]
			if curQ.Context != "" {
				rightSB.WriteString(lipgloss.NewStyle().Foreground(v.styles.Theme.Border).Render(strings.Repeat("─", rightWidth-6)) + "\n")
				rightSB.WriteString(lipgloss.NewStyle().Bold(true).Foreground(v.styles.Theme.Secondary).Render("💡 Context:\n"))
				rightSB.WriteString(lipgloss.NewStyle().Foreground(v.styles.Theme.TextMuted).Render(curQ.Context) + "\n")
			}
		}
	} else {
		rightSB.WriteString(lipgloss.NewStyle().Foreground(v.styles.Theme.Success).Bold(true).Render("✅ No pending decisions at this time. Second Brain 100% consolidated!\n\n"))
		q := lore.GetRandomQuote()
		rightSB.WriteString(lipgloss.NewStyle().Foreground(v.styles.Theme.Border).Render(strings.Repeat("─", rightWidth-6)) + "\n\n")
		rightSB.WriteString(lore.FormatQuote(q) + "\n")
	}

	return RenderSplitLayout(SplitLayoutConfig{
		Width:        v.width,
		Height:       v.height,
		LeftRatio:    0.48,
		TopBar:       "",
		LeftContent:  leftSB.String(),
		RightContent: rightSB.String(),
		LeftStyle:    v.styles.Panel,
		RightStyle:   v.styles.Panel,
	})
}
