//nolint:revive // package comment not required
package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/filepicker"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/rjfonseca/daneel/internal/daemon"
	"github.com/rjfonseca/daneel/internal/ingest"
	"github.com/rjfonseca/daneel/internal/search"
)

// IngestHistoryItem records an ingested file during the current TUI session.
type IngestHistoryItem struct {
	RelPath    string `json:"rel_path"`
	SourcePath string `json:"source_path"`
	Time       string `json:"time"`
	Status     string `json:"status"`
}

// IngestView manages the interactive filepicker and ingestion panel.
type IngestView struct {
	vaultRoot    string
	searchEngine *search.Engine
	ingestEngine *ingest.Engine
	styles       Styles
	filepicker   filepicker.Model
	destInput    textinput.Model
	destination  string
	history      []IngestHistoryItem
	statusMsg    string
	width        int
	height       int
	focusPane    int // 0: file browser, 1: details / destination
	inputMode    bool
	previewPath  string
	previewText  string
}

// NewIngestView constructs a new IngestView.
func NewIngestView(vaultRoot string, engine *search.Engine, styles Styles) *IngestView {
	fp := filepicker.New()
	fp.AllowedTypes = []string{".md", ".markdown", ".txt"}
	cwd, err := os.Getwd()
	if err != nil {
		home, _ := os.UserHomeDir()
		cwd = home
	}
	fp.CurrentDirectory = cwd
	fp.ShowHidden = false
	fp.AutoHeight = false

	ti := textinput.New()
	ti.Placeholder = "inbox"
	ti.SetValue("inbox")
	ti.Prompt = "📂 Destination: "
	ti.PromptStyle = lipgloss.NewStyle().Foreground(styles.Theme.Primary).Bold(true)
	ti.Blur()

	return &IngestView{
		vaultRoot:    vaultRoot,
		searchEngine: engine,
		styles:       styles,
		filepicker:   fp,
		destInput:    ti,
		destination:  "inbox",
		history:      make([]IngestHistoryItem, 0),
		statusMsg:    "Ready. Select a file or press 'd' to ingest current directory.",
	}
}

// SetIngestEngine allows injecting a custom ingest engine (e.g. for hermetic testing).
func (v *IngestView) SetIngestEngine(eng *ingest.Engine) {
	v.ingestEngine = eng
}

// SetCurrentDirectory sets the initial directory of the filepicker.
func (v *IngestView) SetCurrentDirectory(dir string) {
	v.filepicker.CurrentDirectory = dir
}

// SetDestination overrides the target vault destination directory.
func (v *IngestView) SetDestination(dest string) {
	v.destination = dest
	v.destInput.SetValue(dest)
}

// Destination returns the currently configured target destination directory.
func (v *IngestView) Destination() string {
	if v.destination == "" {
		return "inbox"
	}
	return v.destination
}

// History returns a copy of the session ingestion history.
func (v *IngestView) History() []IngestHistoryItem {
	return v.history
}

// IsInputMode returns whether the user is currently editing the destination text input.
func (v *IngestView) IsInputMode() bool {
	return v.inputMode
}

// SetSize updates view dimensions and adjusts filepicker viewport height.
func (v *IngestView) SetSize(w, h int) {
	v.width = w
	v.height = h
	pickerHeight := h - 6
	if pickerHeight < 6 {
		pickerHeight = 6
	}
	v.filepicker.SetHeight(pickerHeight)
}

// Init initializes the child filepicker model.
func (v *IngestView) Init() tea.Cmd {
	return v.filepicker.Init()
}

// IngestPath executes ingestion of a host file or directory into the vault.
func (v *IngestView) IngestPath(path string) (*ingest.IngestReport, error) {
	if v.ingestEngine == nil {
		v.ingestEngine = ingest.NewEngine(v.vaultRoot, v.searchEngine, daemon.GetDefaultHub())
	}

	dest := v.destination
	if dest == "" {
		dest = "inbox"
	}

	opts := ingest.IngestOptions{
		Destination: dest,
		Recursive:   true,
	}

	report, err := v.ingestEngine.IngestPath(context.Background(), path, opts)
	if err != nil {
		v.statusMsg = fmt.Sprintf("Error ingesting %s: %v", filepath.Base(path), err)
		v.history = append([]IngestHistoryItem{{
			SourcePath: path,
			Time:       time.Now().Format("15:04:05"),
			Status:     fmt.Sprintf("❌ Error: %v", err),
		}}, v.history...)
		return nil, err
	}

	if report != nil {
		for _, item := range report.Items {
			status := "🟢 Ingested"
			if item.Warning != "" {
				status = "🟡 " + item.Warning
			}
			v.history = append([]IngestHistoryItem{{
				RelPath:    item.RelPath,
				SourcePath: item.SourcePath,
				Time:       time.Now().Format("15:04:05"),
				Status:     status,
			}}, v.history...)
		}

		v.previewPath = path
		if len(report.Items) == 1 {
			targetVaultPath := filepath.Join(v.vaultRoot, report.Items[0].RelPath)
			if data, err := os.ReadFile(targetVaultPath); err == nil {
				lines := strings.Split(string(data), "\n")
				if len(lines) > 8 {
					v.previewText = strings.Join(lines[:8], "\n") + "\n..."
				} else {
					v.previewText = string(data)
				}
			}
			v.statusMsg = fmt.Sprintf("Successfully ingested %s -> %s", filepath.Base(path), report.Items[0].RelPath)
		} else {
			v.statusMsg = fmt.Sprintf("Successfully ingested %d notes into %s/", len(report.Items), dest)
		}
	}

	return report, nil
}

// Update handles keyboard navigation and messages for IngestView.
func (v *IngestView) Update(msg tea.Msg) (*IngestView, tea.Cmd) {
	cmds := make([]tea.Cmd, 0, 1)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// 1. Text input mode for destination
		if v.inputMode {
			switch msg.String() {
			case "esc":
				v.inputMode = false
				v.destInput.Blur()
				v.destInput.SetValue(v.destination)
				return v, nil
			case "enter":
				v.inputMode = false
				v.destInput.Blur()
				val := strings.TrimSpace(v.destInput.Value())
				if val != "" {
					v.destination = val
				} else {
					v.destination = "inbox"
					v.destInput.SetValue("inbox")
				}
				v.statusMsg = fmt.Sprintf("Target destination set to '%s'", v.destination)
				return v, nil
			}

			var tiCmd tea.Cmd
			v.destInput, tiCmd = v.destInput.Update(msg)
			return v, tiCmd
		}

		// 2. Normal mode commands
		switch msg.String() {
		case "tab":
			v.focusPane = (v.focusPane + 1) % 2
			return v, nil
		case "t":
			v.inputMode = true
			v.destInput.Focus()
			return v, textinput.Blink
		case "d", "D":
			if v.filepicker.CurrentDirectory != "" {
				_, _ = v.IngestPath(v.filepicker.CurrentDirectory)
			}
			return v, nil
		}
	}

	// 3. Delegate to filepicker
	prevDir := v.filepicker.CurrentDirectory
	var fpCmd tea.Cmd
	v.filepicker, fpCmd = v.filepicker.Update(msg)
	cmds = append(cmds, fpCmd)

	if didSelect, path := v.filepicker.DidSelectFile(msg); didSelect {
		_, _ = v.IngestPath(path)
	}

	if didSelectDisabled, path := v.filepicker.DidSelectDisabledFile(msg); didSelectDisabled {
		v.statusMsg = fmt.Sprintf("⚠️ File '%s' is not supported (allowed: .md, .markdown, .txt)", filepath.Base(path))
	}

	if v.filepicker.CurrentDirectory != prevDir {
		v.statusMsg = fmt.Sprintf("Browsing: %s", v.filepicker.CurrentDirectory)
	}

	return v, tea.Batch(cmds...)
}

// View renders the dual-pane file browser and details interface.
func (v *IngestView) View() string {
	leftWidth := (v.width * 5) / 10
	if leftWidth < 35 {
		leftWidth = 35
	}
	rightWidth := v.width - leftWidth - 4
	if rightWidth < 35 {
		rightWidth = 35
	}

	// Left Pane: File Browser
	var leftSB strings.Builder
	leftSB.WriteString(v.styles.PanelTitle.Render("📂 File Browser") + "\n")
	leftSB.WriteString(lipgloss.NewStyle().Foreground(v.styles.Theme.TextMuted).Render("Directory: "+v.filepicker.CurrentDirectory) + "\n\n")
	leftSB.WriteString(v.filepicker.View())

	leftPanelStyle := v.styles.Panel
	if v.focusPane == 0 {
		leftPanelStyle = v.styles.PanelFocused
	}
	leftPane := leftPanelStyle.Width(leftWidth).MaxWidth(leftWidth).Render(leftSB.String())

	// Right Pane: Destination, Status, Preview, Instructions, History
	var rightSB strings.Builder
	rightSB.WriteString(v.styles.PanelTitle.Render("🎯 Target Vault Destination") + "\n\n")

	if v.inputMode {
		rightSB.WriteString(v.destInput.View() + "\n")
		rightSB.WriteString(lipgloss.NewStyle().Foreground(v.styles.Theme.TextMuted).Render("  (Press Enter to save, Esc to cancel)") + "\n\n")
	} else {
		destBadge := v.styles.BadgeInbox.Render(v.destination + "/")
		fmt.Fprintf(&rightSB, "Vault folder: %s  %s\n\n",
			destBadge,
			lipgloss.NewStyle().Foreground(v.styles.Theme.TextMuted).Render("[t] edit destination"))
	}

	// Notification Banner / Status
	rightSB.WriteString(v.styles.PanelTitle.Render("🔔 Status") + "\n")
	rightSB.WriteString(lipgloss.NewStyle().Foreground(v.styles.Theme.Primary).Render(v.statusMsg) + "\n\n")

	// Preview or Instructions
	if v.previewText != "" {
		rightSB.WriteString(v.styles.PanelTitle.Render(fmt.Sprintf("📄 Ingested Note Preview: %s", filepath.Base(v.previewPath))) + "\n")
		rightSB.WriteString(v.previewText + "\n\n")
	} else {
		rightSB.WriteString(v.styles.PanelTitle.Render("💡 Instructions") + "\n")
		rightSB.WriteString("  • " + v.styles.StatusKey.Render("Enter") + " " + v.styles.StatusDesc.Render("Ingest selected file or navigate into folder") + "\n")
		rightSB.WriteString("  • " + v.styles.StatusKey.Render("d") + " " + v.styles.StatusDesc.Render("Ingest entire current directory recursively") + "\n")
		rightSB.WriteString("  • " + v.styles.StatusKey.Render("t") + " " + v.styles.StatusDesc.Render("Edit target vault destination directory") + "\n")
		rightSB.WriteString("  • " + v.styles.StatusKey.Render("Tab") + " " + v.styles.StatusDesc.Render("Switch focus between browser and details") + "\n")
		rightSB.WriteString("  • " + v.styles.StatusKey.Render("Esc/h") + " " + v.styles.StatusDesc.Render("Navigate to parent directory") + "\n\n")
	}

	// Ingestion History
	rightSB.WriteString(v.styles.PanelTitle.Render("📜 Session Ingestion History") + "\n")
	if len(v.history) == 0 {
		rightSB.WriteString(lipgloss.NewStyle().Foreground(v.styles.Theme.TextMuted).Render("  (No files ingested in this session yet)\n"))
	} else {
		maxItems := 6
		if len(v.history) < maxItems {
			maxItems = len(v.history)
		}
		for i := 0; i < maxItems; i++ {
			item := v.history[i]
			timeStr := lipgloss.NewStyle().Foreground(v.styles.Theme.TextMuted).Render("[" + item.Time + "]")
			fmt.Fprintf(&rightSB, "  %s %s %s\n", timeStr, item.Status, item.RelPath)
		}
		if len(v.history) > maxItems {
			rightSB.WriteString(lipgloss.NewStyle().Foreground(v.styles.Theme.TextMuted).Render(fmt.Sprintf("  ... and %d more items\n", len(v.history)-maxItems)))
		}
	}

	rightPanelStyle := v.styles.Panel
	if v.focusPane == 1 {
		rightPanelStyle = v.styles.PanelFocused
	}
	rightPane := rightPanelStyle.Width(rightWidth).MaxWidth(rightWidth).Render(rightSB.String())

	topBar := lipgloss.NewStyle().Foreground(v.styles.Theme.TextMuted).Render("Universal Ingestion Engine • Select files (.md, .markdown, .txt) or directories to index into your Second Brain")

	var splitView string
	if v.width >= 80 {
		splitView = lipgloss.JoinHorizontal(lipgloss.Top, leftPane, " ", rightPane)
	} else {
		splitView = lipgloss.JoinVertical(lipgloss.Left, leftPane, rightPane)
	}

	return lipgloss.JoinVertical(lipgloss.Left, topBar, "", splitView)
}
