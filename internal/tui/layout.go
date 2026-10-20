//nolint:revive // package comment not required
package tui

import "github.com/charmbracelet/lipgloss"

type SplitLayoutConfig struct { //nolint:revive
	Width        int
	Height       int
	LeftRatio    float64
	TopBar       string
	LeftContent  string
	RightContent string
	LeftStyle    lipgloss.Style
	RightStyle   lipgloss.Style
}

// CalculateLayoutDimensions computes width and height allocations for standard split panes.
func CalculateLayoutDimensions(width, height int, topBarHeight int, leftRatio float64) (leftW, rightW, availH int) {
	availH = height - topBarHeight
	if availH < 5 {
		availH = 5
	}

	leftW = int(float64(width) * leftRatio)
	if leftW < 30 {
		leftW = 30
	}
	rightW = width - leftW - 3
	if rightW < 20 {
		rightW = 20
	}
	return leftW, rightW, availH
}

// RenderSplitLayout creates a consistent dual-pane UI with optional top bar.
func RenderSplitLayout(cfg SplitLayoutConfig) string {
	topBarHeight := lipgloss.Height(cfg.TopBar)
	if topBarHeight > 0 {
		topBarHeight++ // account for the blank line spacer //nolint:revive
	}
	leftW, rightW, availH := CalculateLayoutDimensions(cfg.Width, cfg.Height, topBarHeight, cfg.LeftRatio)

	leftPane := cfg.LeftStyle.
		Width(leftW).MaxWidth(leftW).
		Height(availH).MaxHeight(availH).
		Render(cfg.LeftContent)

	rightPane := cfg.RightStyle.
		Width(rightW).MaxWidth(rightW).
		Height(availH).MaxHeight(availH).
		Render(cfg.RightContent)

	splitView := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, " ", rightPane)

	if cfg.TopBar != "" {
		// Use empty string to add a blank line spacing between top bar and panels
		return lipgloss.JoinVertical(lipgloss.Left, cfg.TopBar, "", splitView)
	}
	return splitView
}
