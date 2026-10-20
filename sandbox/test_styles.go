//go:build ignore

package main

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"

	"github.com/rjfonseca/daneel/internal/tui"
)

func main() {
	styles := tui.NewStyles()
	ti := textinput.New()
	ti.Width = 30
	ti.SetValue("hello")

	fmt.Printf("SearchBox Width: %d\n", lipgloss.Width(styles.SearchBox.Render(ti.View())))
	fmt.Printf("PanelFocused Width: %d\n", lipgloss.Width(styles.PanelFocused.Render(ti.View())))
}
