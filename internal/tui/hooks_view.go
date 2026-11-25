//nolint:revive // package comment not required
package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	graphv1 "github.com/rjfonseca/daneel/api/v1"
	"github.com/rjfonseca/daneel/internal/ops/overview"
	"github.com/rjfonseca/daneel/internal/spooler"
)

// HooksView provides a live TUI monitor of active & recent agent sessions (Antigravity, Claude Code, Codex, Spooler).
type HooksView struct {
	vaultRoot     string
	styles        Styles
	backend       *Backend
	sp            *spooler.Spooler
	sessions      []*graphv1.AgentSessionItem
	selectedIndex int
	events        []*graphv1.AgentEventItem
	loaded        bool
	loading       bool
	statusMsg     string
	width         int
	height        int
}

type hooksLoadedMsg struct {
	resp      *graphv1.GetAgentActivityResponse
	sessionID string
	err       error
}

// NewHooksView creates a new HooksView model without synchronous startup I/O.
func NewHooksView(vaultRoot string, styles Styles) *HooksView {
	return &HooksView{
		vaultRoot:     vaultRoot,
		styles:        styles,
		selectedIndex: 0,
	}
}

// SetBackend injects the daemon-first backend.
func (v *HooksView) SetBackend(b *Backend) {
	v.backend = b
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

// EnsureLoaded triggers async agent activity loading on first activation.
func (v *HooksView) EnsureLoaded() tea.Cmd {
	if v.loaded || v.loading {
		return nil
	}
	return v.LoadAsync("")
}

// LoadAsync loads active and recent sessions (and optional session events) via daemon or local builders.
func (v *HooksView) LoadAsync(sessionID string) tea.Cmd {
	if v.sp != nil {
		v.refreshFromCustomSpooler(sessionID)
		return nil
	}
	v.loading = true
	backend := v.backend
	if backend == nil {
		backend = NewLocalBackend(v.vaultRoot, nil, nil)
	}
	return func() tea.Msg {
		resp, err := backend.GetAgentActivity(context.Background(), 24, sessionID)
		return hooksLoadedMsg{resp: resp, sessionID: sessionID, err: err}
	}
}

// Refresh synchronously reloads agent activity (used by hermetic unit tests).
func (v *HooksView) Refresh() {
	if v.sp != nil {
		v.refreshFromCustomSpooler("")
		return
	}
	backend := v.backend
	if backend == nil {
		backend = NewLocalBackend(v.vaultRoot, nil, nil)
	}
	if resp, err := backend.GetAgentActivity(context.Background(), 24, ""); err == nil && resp != nil {
		v.applyActivityResponse(resp, "")
	}
	v.loaded = true
}

func (v *HooksView) refreshFromCustomSpooler(sessionID string) {
	v.loaded = true
	v.loading = false
	resp, err := overview.BuildAgentActivity(context.Background(), v.vaultRoot, nil, nil, v.sp.BaseDir(), 24, sessionID)
	if err != nil {
		v.statusMsg = fmt.Sprintf("Error loading spool sessions: %v", err)
		return
	}
	v.applyActivityResponse(resp, sessionID)
}

func (v *HooksView) applyActivityResponse(resp *graphv1.GetAgentActivityResponse, requestedSID string) {
	v.loading = false
	v.loaded = true
	if resp == nil {
		return
	}
	v.statusMsg = ""
	if requestedSID == "" || len(v.sessions) == 0 {
		v.sessions = resp.Sessions
		if v.selectedIndex >= len(v.sessions) {
			v.selectedIndex = 0
		}
	}
	v.events = resp.Events
}

// Init initializes the view.
func (v *HooksView) Init() tea.Cmd {
	return nil
}

// Update handles keyboard and async messages for the hooks view.
func (v *HooksView) Update(msg tea.Msg) (*HooksView, tea.Cmd) {
	switch msg := msg.(type) {
	case hooksLoadedMsg:
		v.loading = false
		v.loaded = true
		if msg.err != nil {
			v.statusMsg = fmt.Sprintf("⚠️ Agent activity error: %v", msg.err)
		} else if msg.resp != nil {
			v.applyActivityResponse(msg.resp, msg.sessionID)
		}
		return v, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if v.selectedIndex > 0 {
				v.selectedIndex--
				return v, v.LoadAsync(v.sessions[v.selectedIndex].SessionId)
			}
		case "down", "j":
			if v.selectedIndex < len(v.sessions)-1 {
				v.selectedIndex++
				return v, v.LoadAsync(v.sessions[v.selectedIndex].SessionId)
			}
		case "r":
			return v, v.LoadAsync("")
		}
	}
	return v, nil
}

// View renders the dual-pane agent activity and spool monitor.
func (v *HooksView) View() string {
	if !v.loaded && len(v.sessions) == 0 {
		return "Loading active & recent agent sessions (Antigravity, Claude Code, Codex)..."
	}

	if len(v.sessions) == 0 {
		emptyBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(v.styles.Theme.Border).
			Padding(2, 4).
			Render(fmt.Sprintf("%s\n\n%s",
				lipgloss.NewStyle().Bold(true).Foreground(v.styles.Theme.Warning).Render("🤖 No active sessions in the last 24h"),
				lipgloss.NewStyle().Foreground(v.styles.Theme.TextMuted).Render("Active sessions from Antigravity, Claude Code, Codex, or spool hooks appear here. Press 'r' to refresh.")))
		return emptyBox
	}

	leftWidth, rightWidth, availH := CalculateLayoutDimensions(v.width, v.height, 0, 0.40)

	var leftSB strings.Builder
	leftSB.WriteString(v.styles.PanelTitle.Render(fmt.Sprintf("🤖 AGENT SESSIONS (%d / 24h)", len(v.sessions))) + "\n\n")

	maxVisible := max(3, (availH-4)/3)
	start := 0
	if v.selectedIndex >= maxVisible {
		start = v.selectedIndex - maxVisible + 1
	}
	end := min(len(v.sessions), start+maxVisible)

	for i := start; i < end; i++ {
		sess := v.sessions[i]
		statusDot := "⚪"
		if sess.IsActive {
			statusDot = "🟢"
		}
		agentTag := shortAgentTag(sess.Agent)
		shortID := sess.SessionId
		if len(shortID) > 10 {
			shortID = shortID[:10]
		}
		ago := formatRelativeTime(sess.UpdatedAt)
		countLabel := fmt.Sprintf("%dt", sess.TurnsCount)
		if sess.EventCount > 0 {
			countLabel = fmt.Sprintf("%dt/%de", sess.TurnsCount, sess.EventCount)
		}

		line := fmt.Sprintf("%s %-7s %-10s %s %s", statusDot, agentTag, shortID, countLabel, ago)
		rowStyle := v.styles.ListItem
		prefix := "  "
		if i == v.selectedIndex {
			rowStyle = v.styles.ListItemSelected
			prefix = "▸ "
		}
		leftSB.WriteString(rowStyle.Width(max(10, leftWidth-4)).Render(prefix+line) + "\n")
	}

	var rightSB strings.Builder
	detailW := max(10, rightWidth-4)
	if v.selectedIndex >= 0 && v.selectedIndex < len(v.sessions) {
		curr := v.sessions[v.selectedIndex]
		statusLabel := "⚪ RECENT"
		if curr.IsActive {
			statusLabel = "🟢 ACTIVE"
		}

		rightSB.WriteString(v.styles.PanelTitle.Render(fmt.Sprintf("🤖 SESSION: %s", curr.SessionId)) + "\n\n")
		fmt.Fprintf(&rightSB, "Agent: %-12s  Status: %s\n", curr.Agent, statusLabel)
		model := curr.Model
		if model == "" {
			model = "—"
		}
		fmt.Fprintf(&rightSB, "Model: %-12s  Turns: %d  Events: %d\n", model, curr.TurnsCount, curr.EventCount)
		if curr.InputTokens > 0 || curr.CacheReadTokens > 0 || curr.OutputTokens > 0 {
			fmt.Fprintf(&rightSB, "Tokens: In %s | Cache %s | Out %s | Cost: $%.4f\n",
				formatTokens(curr.InputTokens),
				formatTokens(curr.CacheReadTokens),
				formatTokens(curr.OutputTokens),
				curr.EstimatedCostUsd)
		}
		if curr.SkillsUsed != "" {
			fmt.Fprintf(&rightSB, "Skills: %s\n", truncateText(curr.SkillsUsed, detailW-8))
		}
		fmt.Fprintf(&rightSB, "Updated: %s (%s)\n",
			time.Unix(curr.UpdatedAt, 0).Format("2006-01-02 15:04:05"),
			formatRelativeTime(curr.UpdatedAt))

		rightSB.WriteString("\n" + lipgloss.NewStyle().Foreground(v.styles.Theme.Border).Render(strings.Repeat("─", max(1, rightWidth-6))) + "\n")

		if len(v.events) > 0 {
			rightSB.WriteString(lipgloss.NewStyle().Bold(true).Foreground(v.styles.Theme.Secondary).Render("📡 Recent Spool Events:\n\n"))
			maxEv := min(6, len(v.events))
			for _, evt := range v.events[len(v.events)-maxEv:] {
				header := fmt.Sprintf("[%s] %s (%s)", evt.Timestamp, evt.Event, evt.Agent)
				rightSB.WriteString(lipgloss.NewStyle().Foreground(v.styles.Theme.Success).Bold(true).Render(truncateText(header, detailW)) + "\n")
				if evt.Cwd != "" {
					rightSB.WriteString(lipgloss.NewStyle().Foreground(v.styles.Theme.Text).Render("  cwd: "+truncateText(evt.Cwd, detailW-7)) + "\n")
				}
				if evt.PayloadSnippet != "" {
					rightSB.WriteString(lipgloss.NewStyle().Foreground(v.styles.Theme.TextMuted).Render("  "+truncateText(evt.PayloadSnippet, detailW-2)) + "\n")
				}
			}
		} else {
			rightSB.WriteString(lipgloss.NewStyle().Foreground(v.styles.Theme.TextMuted).Render("No raw hook spool events for this session (tracked via transcript telemetry).\n"))
		}
	}

	return RenderSplitLayout(SplitLayoutConfig{
		Width:        v.width,
		Height:       v.height,
		LeftRatio:    0.40,
		TopBar:       "",
		LeftContent:  leftSB.String(),
		RightContent: rightSB.String(),
		LeftStyle:    v.styles.Panel,
		RightStyle:   v.styles.Panel,
	})
}

func shortAgentTag(agent string) string {
	switch strings.ToLower(strings.TrimSpace(agent)) {
	case "antigravity", "agy":
		return "agy"
	case "claude-code", "claude":
		return "claude"
	case "codex":
		return "codex"
	default:
		if agent == "" {
			return "agent"
		}
		return truncateText(agent, 7)
	}
}

func formatRelativeTime(unixTs int64) string {
	if unixTs <= 0 {
		return "—"
	}
	d := time.Since(time.Unix(unixTs, 0))
	if d < time.Minute {
		return "now"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	return fmt.Sprintf("%dd ago", int(d.Hours()/24))
}
