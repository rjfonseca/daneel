//go:build ignore

package main

import (
	"fmt"
	"strings"

	"github.com/rjfonseca/daneel/internal/tui"
)

func main() {
	styles := tui.NewStyles()

	// Mock SearchView
	var listContent strings.Builder
	listContent.WriteString(fmt.Sprintf("%s (%d found):\n\n", styles.PanelTitle.Render("📋 Notes"), 10))
	listContent.WriteString(styles.ListItem.Width(20).Render("  Item 1") + "\n")
	listContent.WriteString(styles.ListItem.Width(20).Render("  Item 2") + "\n")

	_ = styles.SplitLeft.Width(30).Render(listContent.String())

	topRow := "SearchBox\nText\nBottom"

	layout := tui.RenderSplitLayout(tui.SplitLayoutConfig{
		Width:        80,
		Height:       20,
		LeftRatio:    0.4,
		TopBar:       topRow,
		LeftContent:  listContent.String(),
		RightContent: "Right",
		LeftStyle:    styles.SplitLeft,
		RightStyle:   styles.SplitRight,
	})

	lines := strings.Split(layout, "\n")
	for i, l := range lines {
		// Just to see where Item 1 is
		if strings.Contains(l, "Item 1") {
			fmt.Printf("Line %d: %s\n", i+2, l) // +2 for Header and Tabs
		}
	}
}
