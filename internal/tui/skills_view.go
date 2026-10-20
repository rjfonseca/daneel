//nolint:revive // package comment not required
package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type SkillsView struct { //nolint:revive
	vaultRoot     string
	styles        Styles
	width         int
	height        int
	selectedIndex int
	items         []string
	isAction      []bool

	verbose   bool
	isRunning bool
	cmdOutput string
}

func NewSkillsView(vaultRoot string, styles Styles) *SkillsView { //nolint:revive
	sv := &SkillsView{
		vaultRoot: vaultRoot,
		styles:    styles,
		items:     []string{},
		isAction:  []bool{},
		verbose:   false,
	}
	sv.loadSkills()
	return sv
}

func (v *SkillsView) loadSkills() {
	// Deterministic Actions
	v.items = append(v.items,
		"Routine: Auto-Heal Links (tidy)",
		"Routine: Vault Diagnostics (doctor)",
		"Routine: Reindex FTS5 (index)",
		"Routine: Compile Staff Dossier (dossier)",
		"Routine: Workspace Sync (workspace)",
		"Routine: Security Firewall Audit (guard --check)",
		"Routine: Ingest Meeting Notes (ingest)",
		"Routine: Inbox Triage Overview (inbox)",
		"Routine: Staff Time Log & Allocation (log --recent=7)",
	)
	v.isAction = append(v.isAction, true, true, true, true, true, true, true, true, true)

	// AI Skills
	skillsDir := filepath.Join(v.vaultRoot, ".agents", "skills")
	entries, err := os.ReadDir(skillsDir)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() {
				v.items = append(v.items, "AI Skill: "+e.Name())
				v.isAction = append(v.isAction, false)
			}
		}
	}
	if len(v.items) == 9 { // Only actions loaded
		v.items = append(v.items, "AI Skill: vault-assistant", "AI Skill: meeting-preflight", "AI Skill: vault-dream-cycle", "AI Skill: staff-avd-compiler")
		v.isAction = append(v.isAction, false, false, false, false)
	}
}

func (v *SkillsView) Init() tea.Cmd { //nolint:revive
	return nil
}

type skillDoneMsg struct {
	output string
}

func (v *SkillsView) Update(msg tea.Msg) (*SkillsView, tea.Cmd) { //nolint:revive
	switch msg := msg.(type) {
	case skillDoneMsg:
		v.isRunning = false
		v.cmdOutput += "\n" + msg.output
		return v, nil

	case tea.KeyMsg:
		if v.isRunning {
			return v, nil
		}
		switch msg.String() {
		case "up", "k":
			v.cmdOutput = ""
			if v.selectedIndex > 0 {
				v.selectedIndex--
			}
		case "down", "j":
			v.cmdOutput = ""
			if v.selectedIndex < len(v.items)-1 {
				v.selectedIndex++
			}
		case "v":
			v.verbose = !v.verbose
		case "enter":
			if v.selectedIndex >= len(v.items) {
				return v, nil
			}
			item := v.items[v.selectedIndex]
			isAct := v.isAction[v.selectedIndex]

			v.isRunning = true
			v.cmdOutput = fmt.Sprintf("🚀 Starting execution of: %s\nVerbose Mode: %v\n\n", item, v.verbose)

			return v, func() tea.Msg {
				if isAct {
					parts := strings.Split(item, "(")
					if len(parts) > 1 {
						cmdStr := strings.TrimSuffix(parts[1], ")")
						cmdParts := strings.Fields(cmdStr)
						args := cmdParts
						if v.verbose {
							args = append(args, "--verbose")
						}
						cmd := exec.Command(os.Args[0], args...) //nolint:gosec // intentional
						out, err := cmd.CombinedOutput()
						res := string(out)
						if err != nil {
							res += "\n[Error]: " + err.Error()
						}
						return skillDoneMsg{output: res}
					}
				}
				// Mock AI Skill
				return skillDoneMsg{output: "AI Skill orchestrated successfully (Mocked headless mode)."}
			}
		}
	}
	return v, nil
}

func (v *SkillsView) SetSize(w, h int) { //nolint:revive
	v.width = w
	v.height = h
}

func (v *SkillsView) View() string { //nolint:revive

	var leftSB strings.Builder
	leftSB.WriteString(v.styles.PanelTitle.Render("⚡ Actions & Skills Deck") + "\n\n")

	for i, item := range v.items {
		badge := v.styles.BadgeClean.Render("⚡")
		if v.isAction[i] {
			badge = lipgloss.NewStyle().Foreground(v.styles.Theme.Warning).Render("⚙️")
		}

		line := fmt.Sprintf("%s %s", badge, item)
		if i == v.selectedIndex {
			leftSB.WriteString(v.styles.ListItemSelected.Render("▸ " + line))
		} else {
			leftSB.WriteString(v.styles.ListItem.Render("  " + line))
		}
		leftSB.WriteString("\n")
	}

	leftSB.WriteString("\n\n" + lipgloss.NewStyle().Foreground(v.styles.Theme.TextMuted).Render("Press [Enter] to execute item. Press [v] to toggle verbose."))

	var rightSB strings.Builder
	if v.cmdOutput != "" {
		rightSB.WriteString(v.styles.PanelTitle.Render("💻 Execution Logs"))
		if v.isRunning {
			rightSB.WriteString(" " + lipgloss.NewStyle().Foreground(v.styles.Theme.Warning).Render("(Running...)"))
		}
		rightSB.WriteString("\n\n")
		rightSB.WriteString(v.cmdOutput)
	} else if len(v.items) > 0 && v.selectedIndex < len(v.items) {
		item := v.items[v.selectedIndex]
		isAct := v.isAction[v.selectedIndex]

		rightSB.WriteString(v.styles.PanelTitle.Render(fmt.Sprintf("📖 %s", item)) + "\n\n")

		if isAct {
			rightSB.WriteString(lipgloss.NewStyle().Bold(true).Foreground(v.styles.Theme.Secondary).Render("Description:\n"))
			rightSB.WriteString("Deterministic native Golang routine.\n\n")
			rightSB.WriteString(lipgloss.NewStyle().Bold(true).Foreground(v.styles.Theme.Primary).Render("Usage:\n"))
			rightSB.WriteString("Runs directly in the Daneel runtime without LLM inference.\n")
		} else {
			rightSB.WriteString(lipgloss.NewStyle().Bold(true).Foreground(v.styles.Theme.Secondary).Render("Description:\n"))
			skillName := strings.TrimPrefix(item, "AI Skill: ")
			rightSB.WriteString("Skill file loaded from .agents/skills/" + skillName + "/SKILL.md\n\n")
			rightSB.WriteString(lipgloss.NewStyle().Bold(true).Foreground(v.styles.Theme.Primary).Render("Usage:\n"))
			rightSB.WriteString("This skill is orchestrated headlessly by the LLM agent.\n")
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
