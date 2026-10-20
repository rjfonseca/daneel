//go:build ignore

package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func main() {
	theme := lipgloss.Color("#ffffff")
	TabInactive := lipgloss.NewStyle().Foreground(theme).Background(theme).Padding(0, 2)

	pills := []string{"ALL", "TECH", "DAILY", "META", "SOURCES"}
	var scopePills strings.Builder
	for i, s := range pills {
		scopePills.WriteString(TabInactive.Render(s))
		if i < len(pills)-1 {
			scopePills.WriteString(" ")
		}
	}

	fmt.Printf("Total pills string width: %d\n", lipgloss.Width(scopePills.String()))

	x := 0
	for _, s := range pills {
		pillW := lipgloss.Width(TabInactive.Render(s))
		fmt.Printf("Pill %s width: %d\n", s, pillW)
		x += pillW + 1
	}
	fmt.Printf("Calculated total width: %d\n", x-1) // -1 for last space
}
