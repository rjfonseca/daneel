//nolint:revive // package comment not required
package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/rjfonseca/daneel/internal/ops/briefing"
)

type personItem struct {
	Name         string
	Slug         string
	Cargo        string
	Squad        string
	Interactions int
}

func (p personItem) Title() string {
	return fmt.Sprintf("%2d 1:1s  %s", p.Interactions, p.Name)
}

func (p personItem) Description() string { return "" }
func (p personItem) FilterValue() string { return p.Name }

// PeopleView displays colleagues and instant 1:1 briefings.
type PeopleView struct {
	vaultRoot    string
	styles       Styles
	service      *briefing.PreflightService
	peopleList   []personItem
	listModel    *DaneelList
	currentBrief *briefing.PersonProfile
	width        int
	height       int
}

// NewPeopleView creates a new people view.
func NewPeopleView(vaultRoot string, styles Styles) *PeopleView {
	pv := &PeopleView{
		vaultRoot: vaultRoot,
		styles:    styles,
		service:   briefing.NewPreflightService(vaultRoot),
	}
	pv.listModel = NewDaneelList([]list.Item{}, 30, 10, styles)
	pv.Refresh()
	return pv
}

// Refresh reloads all people profiles from notes/pessoas/.
func (v *PeopleView) Refresh() {
	peopleDir := filepath.Join(v.vaultRoot, "notes", "pessoas")
	if _, err := os.Stat(peopleDir); os.IsNotExist(err) {
		peopleDir = filepath.Join(v.vaultRoot, "meta", "pessoas")
	}
	var items []personItem

	entries, err := os.ReadDir(peopleDir)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") || entry.Name() == "moc-pessoas.md" {
				continue
			}

			slug := strings.TrimSuffix(entry.Name(), ".md")
			prof, err := v.service.GenerateBriefing(slug)
			if err == nil && prof != nil {
				items = append(items, personItem{
					Name:         prof.Name,
					Slug:         prof.Slug,
					Cargo:        prof.Cargo,
					Squad:        prof.Squad,
					Interactions: len(prof.RecentInteractions),
				})
			}
		}
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Interactions > items[j].Interactions
	})

	v.peopleList = items

	// Convert to list.Item interface
	var listItems []list.Item //nolint:prealloc
	for _, p := range items {
		listItems = append(listItems, p)
	}
	v.listModel.SetItems(listItems)
	v.listModel.Select(0)

	v.updateCurrentBrief()
}

// updateCurrentBrief loads the selected profile profile.
func (v *PeopleView) updateCurrentBrief() {
	if v.listModel.Index() < 0 || v.listModel.Index() >= len(v.peopleList) {
		return
	}
	p := v.peopleList[v.listModel.Index()]
	b, err := v.service.GenerateBriefing(p.Slug)
	if err == nil {
		v.currentBrief = b
	}
}

// Init initializes the view.
func (v *PeopleView) Init() tea.Cmd {
	return nil
}

// Update handles tea events for the people view.
func (v *PeopleView) Update(msg tea.Msg) (*PeopleView, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "r" {
			v.Refresh()
			return v, nil
		}
	case ListItemSelectedMsg:
		// Optional: handled globally, or open in editor
	}

	oldIndex := v.listModel.Index()
	v.listModel, cmd = v.listModel.Update(msg)

	if v.listModel.Index() != oldIndex {
		v.updateCurrentBrief()
	}

	return v, cmd
}

// SetSize updates dimensions.
func (v *PeopleView) SetSize(w, h int) {
	v.width = w
	v.height = h

	leftWidth, _, availableHeight := CalculateLayoutDimensions(w, h, 0, 0.40)

	v.listModel.SetSize(leftWidth-2, availableHeight-2) // subtract border paddings
	// Also inform DaneelList of its bounding box offset for mouse clicks!
	v.listModel.XOffset = 1 // 1 for left border
	v.listModel.YOffset = 5 // 5 lines from the absolute top of the screen (Header + TabBar + PanelTitle)
}

// View renders the 1:1 People Hub.
func (v *PeopleView) View() string {
	if len(v.peopleList) == 0 {
		return "No people profiles found in notes/pessoas/."
	}

	var listSB strings.Builder
	fmt.Fprintf(&listSB, "%s (%d people):\n\n", v.styles.PanelTitle.Render("🤝 Peers & Leadership"), len(v.peopleList))

	listSB.WriteString(v.listModel.View())

	var briefSB strings.Builder
	if v.currentBrief != nil {
		b := v.currentBrief
		briefSB.WriteString(v.styles.PanelTitle.Render(fmt.Sprintf("👤 %s", b.Name)) + "\n\n")
		fmt.Fprintf(&briefSB, "💼 **Role / Title:** %s\n", b.Cargo)
		fmt.Fprintf(&briefSB, "🏢 **Squad / Tribe:** %s\n", b.Squad)
		fmt.Fprintf(&briefSB, "📄 **Profile File:** `%s`\n\n", b.RelPath)
		briefSB.WriteString("---\n\n")

		if len(b.RecentInteractions) > 0 {
			briefSB.WriteString(lipgloss.NewStyle().Bold(true).Foreground(v.styles.Theme.Secondary).Render("🤝 Recent Interaction History & Alignments:") + "\n")
			limit := 4
			if len(b.RecentInteractions) < limit {
				limit = len(b.RecentInteractions)
			}
			for _, inter := range b.RecentInteractions[:limit] {
				fmt.Fprintf(&briefSB, "  • %s\n", inter)
			}
			briefSB.WriteString("\n")
		}

		if len(b.PendingTasks) > 0 {
			briefSB.WriteString(lipgloss.NewStyle().Bold(true).Foreground(v.styles.Theme.Warning).Render("🎯 Shared Actions & Pending Backlog Tasks:") + "\n")
			for _, task := range b.PendingTasks {
				fmt.Fprintf(&briefSB, "  - [ ] %s\n", task)
			}
			briefSB.WriteString("\n")
		}

		if len(b.RecentDailyMentions) > 0 {
			briefSB.WriteString(lipgloss.NewStyle().Bold(true).Foreground(v.styles.Theme.Primary).Render("📅 Recent Daily Log Mentions:") + "\n")
			for _, m := range b.RecentDailyMentions[:2] {
				fmt.Fprintf(&briefSB, "  • %s\n", m)
			}
		}
	} else {
		briefSB.WriteString(lipgloss.NewStyle().Foreground(v.styles.Theme.TextMuted).Render("No person selected.\n"))
	}

	return RenderSplitLayout(SplitLayoutConfig{
		Width:        v.width,
		Height:       v.height,
		LeftRatio:    0.40,
		TopBar:       "",
		LeftContent:  listSB.String(),
		RightContent: briefSB.String(),
		LeftStyle:    v.styles.Panel,
		RightStyle:   v.styles.Panel,
	})
}
