//nolint:revive // package comment not required
package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	graphv1 "github.com/rjfonseca/daneel/api/v1"
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
	backend      *Backend
	service      *briefing.PreflightService
	peopleList   []personItem
	listModel    *DaneelList
	currentBrief *briefing.PersonProfile
	loaded       bool
	loading      bool
	width        int
	height       int
}

type peopleDirectoryLoadedMsg struct {
	people []personItem
	err    error
}

type personBriefingLoadedMsg struct {
	slug    string
	profile *briefing.PersonProfile
	err     error
}

// NewPeopleView creates a new people view without synchronous vault scanning.
func NewPeopleView(vaultRoot string, styles Styles) *PeopleView {
	pv := &PeopleView{
		vaultRoot: vaultRoot,
		styles:    styles,
		service:   briefing.NewPreflightService(vaultRoot),
	}
	pv.listModel = NewDaneelList([]list.Item{}, 30, 10, styles)
	return pv
}

// SetBackend injects the daemon-first backend.
func (v *PeopleView) SetBackend(b *Backend) {
	v.backend = b
}

// EnsureLoaded triggers async directory loading on first activation.
func (v *PeopleView) EnsureLoaded() tea.Cmd {
	if v.loaded || v.loading {
		return nil
	}
	return v.LoadAsync()
}

// LoadAsync loads the lightweight colleague directory via daemon or local metadata.
func (v *PeopleView) LoadAsync() tea.Cmd {
	v.loading = true
	backend := v.backend
	if backend == nil {
		backend = NewLocalBackend(v.vaultRoot, nil, nil)
	}
	return func() tea.Msg {
		resp, err := backend.GetPeopleDirectory(context.Background())
		if err != nil {
			return peopleDirectoryLoadedMsg{err: err}
		}
		var items []personItem
		if resp != nil {
			for _, p := range resp.People {
				items = append(items, personItem{
					Name:         p.Name,
					Slug:         p.Slug,
					Cargo:        p.Role,
					Squad:        p.Squad,
					Interactions: int(p.Interactions),
				})
			}
		}
		return peopleDirectoryLoadedMsg{people: items}
	}
}

func (v *PeopleView) loadSelectedBriefingAsync() tea.Cmd {
	if v.listModel.Index() < 0 || v.listModel.Index() >= len(v.peopleList) {
		return nil
	}
	p := v.peopleList[v.listModel.Index()]
	backend := v.backend
	if backend == nil {
		backend = NewLocalBackend(v.vaultRoot, nil, nil)
	}
	slug := p.Slug
	return func() tea.Msg {
		resp, err := backend.GetPersonBriefing(context.Background(), slug)
		if err != nil || resp == nil {
			return personBriefingLoadedMsg{slug: slug, err: err}
		}
		return personBriefingLoadedMsg{
			slug:    slug,
			profile: briefingFromProto(resp),
		}
	}
}

func briefingFromProto(resp *graphv1.GetPersonBriefingResponse) *briefing.PersonProfile {
	return &briefing.PersonProfile{
		Name:                resp.Name,
		Slug:                resp.Slug,
		RelPath:             resp.RelPath,
		Cargo:               resp.Cargo,
		Squad:               resp.Squad,
		Relacao:             resp.Relacao,
		RecentInteractions:  append([]string(nil), resp.RecentInteractions...),
		Topics:              append([]string(nil), resp.Topics...),
		PendingTasks:        append([]string(nil), resp.PendingTasks...),
		RecentDailyMentions: append([]string(nil), resp.RecentDailyMentions...),
	}
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
	case peopleDirectoryLoadedMsg:
		v.loading = false
		v.loaded = true
		if msg.err == nil {
			v.peopleList = msg.people
			var listItems []list.Item //nolint:prealloc
			for _, p := range msg.people {
				listItems = append(listItems, p)
			}
			v.listModel.SetItems(listItems)
			v.listModel.Select(0)
			return v, v.loadSelectedBriefingAsync()
		}
		return v, nil

	case personBriefingLoadedMsg:
		if msg.err == nil && msg.profile != nil {
			if v.listModel.Index() >= 0 && v.listModel.Index() < len(v.peopleList) &&
				v.peopleList[v.listModel.Index()].Slug == msg.slug {
				v.currentBrief = msg.profile
			}
		}
		return v, nil

	case tea.KeyMsg:
		if msg.String() == "r" {
			return v, v.LoadAsync()
		}
	case ListItemSelectedMsg:
		// Optional: handled globally, or open in editor
	}

	oldIndex := v.listModel.Index()
	v.listModel, cmd = v.listModel.Update(msg)

	if v.listModel.Index() != oldIndex {
		return v, tea.Batch(cmd, v.loadSelectedBriefingAsync())
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
	if !v.loaded && len(v.peopleList) == 0 {
		return "Loading 1:1 People Directory..."
	}
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
			for _, m := range b.RecentDailyMentions[:min(2, len(b.RecentDailyMentions))] {
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
