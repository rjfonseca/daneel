//nolint:revive // package comment not required
package tui

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/rjfonseca/daneel/internal/spooler"
)

// HooksView provides a live TUI dashboard of active agent sessions and spooled events.
type HooksView struct {
	vaultRoot      string
	styles         Styles
	sp             *spooler.Spooler
	sessions       []spooler.SessionSummary
	selectedIndex  int
	selectedEvents []spooler.SpoolEntry
	width          int
	height         int
}

// NewHooksView creates a new HooksView model.
func NewHooksView(vaultRoot string, styles Styles) *HooksView {
	return &HooksView{
		vaultRoot:     vaultRoot,
		styles:        styles,
		sp:            spooler.New(""),
		selectedIndex: 0,
	}
}

// SetSpooler sets a custom spooler instance (useful for hermetic unit testing).
func (v *HooksView) SetSpooler(sp *spooler.Spooler) {
	v.sp = sp
}

// SetSize updates the view dimensions.
func (v *HooksView) SetSize(w, h int) {
	v.width = w
	v.height = h
}

// Refresh reloads session summaries from the spool directory.
func (v *HooksView) Refresh() {
	if v.sp == nil {
		v.sp = spooler.New("")
	}
	sessions, err := v.sp.ListSessions()
	if err == nil {
		v.sessions = sessions
	}
	if v.selectedIndex >= len(v.sessions) {
		v.selectedIndex = 0
	}
	v.loadSelectedSessionEvents()
}

func (v *HooksView) loadSelectedSessionEvents() {
	v.selectedEvents = nil
	if len(v.sessions) == 0 || v.selectedIndex < 0 || v.selectedIndex >= len(v.sessions) {
		return
	}

	target := v.sessions[v.selectedIndex]
	f, err := os.Open(target.FilePath)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	var entries []spooler.SpoolEntry
	for scanner.Scan() {
		var entry spooler.SpoolEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err == nil {
			entries = append(entries, entry)
		}
	}
	v.selectedEvents = entries
}

// Init initializes the view.
func (v *HooksView) Init() tea.Cmd {
	v.Refresh()
	return nil
}

// Update handles keyboard messages for the hooks view.
func (v *HooksView) Update(msg tea.Msg) (*HooksView, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if v.selectedIndex > 0 {
				v.selectedIndex--
				v.loadSelectedSessionEvents()
			}
		case "down", "j":
			if v.selectedIndex < len(v.sessions)-1 {
				v.selectedIndex++
				v.loadSelectedSessionEvents()
			}
		case "r":
			v.Refresh()
		}
	}
	return v, nil
}

// View renders the dual-pane hooks and telemetry monitor.
func (v *HooksView) View() string {
	if len(v.sessions) == 0 {
		emptyBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(v.styles.Theme.Border).
			Padding(2, 4).
			Render(fmt.Sprintf("%s\n\n%s",
				lipgloss.NewStyle().Bold(true).Foreground(v.styles.Theme.Warning).Render("📦 No active sessions in spool"),
				lipgloss.NewStyle().Foreground(v.styles.Theme.TextMuted).Render("Events captured via 'daneel hook capture' will appear here in real-time.")))
		return emptyBox
	}

	leftWidth := 40
	rightWidth := v.width - leftWidth - 4
	if rightWidth < 30 {
		rightWidth = 30
	}

	// 1. Left Pane: Sessions List
	var leftItems []string
	leftItems = append(leftItems, lipgloss.NewStyle().Bold(true).Foreground(v.styles.Theme.Primary).Render("ACTIVE SESSIONS"))

	for i, sess := range v.sessions {
		isSelected := i == v.selectedIndex
		agentName := "agent"
		if len(sess.SessionID) > 8 {
			agentName = sess.SessionID[:8]
		}

		line := fmt.Sprintf("%-16s %3d evts", agentName, sess.EventCount)
		subLine := sess.SessionID
		if len(subLine) > 28 {
			subLine = subLine[:25] + "..."
		}

		itemStyle := lipgloss.NewStyle().Padding(0, 1)
		if isSelected {
			itemStyle = itemStyle.
				Background(v.styles.Theme.SurfaceLight).
				Foreground(v.styles.Theme.Primary).
				Bold(true)
		} else {
			itemStyle = itemStyle.Foreground(v.styles.Theme.Text)
		}

		itemBlock := fmt.Sprintf("%s\n%s",
			itemStyle.Render("▶ "+line),
			lipgloss.NewStyle().Foreground(v.styles.Theme.TextMuted).Render("  "+subLine),
		)
		leftItems = append(leftItems, itemBlock)
	}

	leftPane := lipgloss.NewStyle().
		Width(leftWidth).
		Border(lipgloss.NormalBorder(), false, true, false, false).
		BorderForeground(v.styles.Theme.Border).
		Render(strings.Join(leftItems, "\n\n"))

	// 2. Right Pane: Session Event Stream
	var rightItems []string
	if v.selectedIndex >= 0 && v.selectedIndex < len(v.sessions) {
		curr := v.sessions[v.selectedIndex]
		rightItems = append(rightItems,
			lipgloss.NewStyle().Bold(true).Foreground(v.styles.Theme.Secondary).
				Render(fmt.Sprintf("SESSION TELEMETRY: %s", curr.SessionID)))
		rightItems = append(rightItems,
			lipgloss.NewStyle().Foreground(v.styles.Theme.TextMuted).
				Render(fmt.Sprintf("File: %s | Events: %d | Size: %d B", curr.FilePath, curr.EventCount, curr.SizeBytes)))
		rightItems = append(rightItems, "")

		maxEntries := 8
		if len(v.selectedEvents) < maxEntries {
			maxEntries = len(v.selectedEvents)
		}

		for idx := len(v.selectedEvents) - maxEntries; idx < len(v.selectedEvents); idx++ {
			if idx < 0 {
				continue
			}
			evt := v.selectedEvents[idx]
			header := fmt.Sprintf("[%s] %-14s (%s)", evt.Timestamp, evt.Event, evt.Agent)
			payloadSnippet := string(evt.Payload)
			if len(payloadSnippet) > 120 {
				payloadSnippet = payloadSnippet[:117] + "..."
			}

			evtBlock := fmt.Sprintf("%s\n  %s\n  %s",
				lipgloss.NewStyle().Foreground(v.styles.Theme.Success).Bold(true).Render(header),
				lipgloss.NewStyle().Foreground(v.styles.Theme.Text).Render("cwd: "+evt.CWD),
				lipgloss.NewStyle().Foreground(v.styles.Theme.TextMuted).Render(payloadSnippet),
			)
			rightItems = append(rightItems, evtBlock)
		}
	}

	rightPane := lipgloss.NewStyle().
		Width(rightWidth).
		PaddingLeft(2).
		Render(strings.Join(rightItems, "\n"))

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)
}
