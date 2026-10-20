//go:build ignore

package main

import (
	"fmt"
	"strings"

	"github.com/rjfonseca/daneel/internal/tui"
)

func main() {
	styles := tui.NewStyles()
	var listContent strings.Builder
	listContent.WriteString(fmt.Sprintf("%s (%d people):\n\n", styles.PanelTitle.Render("🤝 Peers"), 10))
	listContent.WriteString(styles.ListItem.Width(20).Render("  Person 1") + "\n")

	layout := tui.RenderSplitLayout(tui.SplitLayoutConfig{
		Width:        80,
		Height:       20,
		LeftRatio:    0.4,
		TopBar:       "", // NO TOP BAR
		LeftContent:  listContent.String(),
		RightContent: "Right",
		LeftStyle:    styles.Panel,
		RightStyle:   styles.Panel,
	})

	lines := strings.Split(layout, "\n")
	for i, l := range lines {
		if strings.Contains(l, "Person 1") {
			fmt.Printf("Line %d: %s\n", i+2, l) // +2 for Header and Tabs
		}
	}
}
