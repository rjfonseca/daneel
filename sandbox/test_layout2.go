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
	listContent.WriteString(fmt.Sprintf("%s (%d found):\n\n", styles.PanelTitle.Render("📁 Repositories"), 10))
	listContent.WriteString(styles.ListItem.Width(20).Render("  Repo 1") + "\n")

	layout := tui.RenderSplitLayout(tui.SplitLayoutConfig{
		Width:        80,
		Height:       20,
		LeftRatio:    0.4,
		TopBar:       "Single Line Top Bar",
		LeftContent:  listContent.String(),
		RightContent: "Right",
		LeftStyle:    styles.Panel,
		RightStyle:   styles.Panel,
	})

	lines := strings.Split(layout, "\n")
	for i, l := range lines {
		if strings.Contains(l, "Repo 1") {
			fmt.Printf("Line %d: %s\n", i+2, l) // +2 for Header and Tabs
		}
	}
}
