//go:build ignore

package main

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

func main() {
	ti := textinput.New()
	ti.Width = 30
	ti.SetValue("hello")

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(0, 1)

	rendered := style.Render(ti.View())
	fmt.Printf("Visual Width: %d\n", lipgloss.Width(rendered))
}
