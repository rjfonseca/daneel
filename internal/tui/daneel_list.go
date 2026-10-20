//nolint:revive // package comment not required
package tui

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ListItemSelectedMsg is emitted when a user selects an item via Enter or Click
type ListItemSelectedMsg struct {
	Index int
	Item  list.Item
}

// DaneelList wraps bubbles/list to add mouse click support and consistent styling
type DaneelList struct {
	list.Model
	YOffset int
	XOffset int
}

func NewDaneelList(items []list.Item, width, height int, styles Styles) *DaneelList { //nolint:revive
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = false
	delegate.SetSpacing(0)

	delegate.Styles.SelectedTitle = styles.ListItemSelected.
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(styles.Theme.Primary).
		Padding(0, 0, 0, 1)
	delegate.Styles.NormalTitle = styles.ListItem.
		Padding(0, 0, 0, 2)

	l := list.New(items, delegate, width, height)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false) // Custom filtering per view if needed
	l.SetShowHelp(false)
	l.SetShowPagination(false)
	l.DisableQuitKeybindings()

	return &DaneelList{
		Model:   l,
		YOffset: 0,
		XOffset: 0,
	}
}

func (m *DaneelList) Update(msg tea.Msg) (*DaneelList, tea.Cmd) { //nolint:revive
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "enter" {
			if m.Index() >= 0 && m.Index() < len(m.Items()) {
				return m, func() tea.Msg {
					return ListItemSelectedMsg{
						Index: m.Index(),
						Item:  m.SelectedItem(),
					}
				}
			}
		}
	case tea.MouseMsg:
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			if msg.X >= m.XOffset && msg.X < m.XOffset+m.Width() {
				if msg.Y >= m.YOffset && msg.Y < m.YOffset+m.Height() {
					clickY := msg.Y - m.YOffset
					startIdx := m.Paginator.Page * m.Paginator.PerPage
					clickedIdx := startIdx + clickY

					if clickedIdx >= 0 && clickedIdx < len(m.Items()) {
						m.Select(clickedIdx)
						return m, func() tea.Msg {
							return ListItemSelectedMsg{
								Index: clickedIdx,
								Item:  m.SelectedItem(),
							}
						}
					}
				}
			}
		}
	}

	newList, cmd := m.Model.Update(msg)
	m.Model = newList
	return m, cmd
}
