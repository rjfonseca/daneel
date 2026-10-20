//nolint:revive // package comment not required
package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// Theme defines the visual palette of Daneel TUI (Tokyo Night / Catppuccin inspired).
type Theme struct {
	Primary      lipgloss.Color
	Secondary    lipgloss.Color
	Success      lipgloss.Color
	Warning      lipgloss.Color
	Error        lipgloss.Color
	Info         lipgloss.Color
	Background   lipgloss.Color
	Surface      lipgloss.Color
	SurfaceLight lipgloss.Color
	Text         lipgloss.Color
	TextMuted    lipgloss.Color
	TextDark     lipgloss.Color
	Border       lipgloss.Color
	BorderFocus  lipgloss.Color
}

// DefaultTheme returns the standard Daneel color theme.
func DefaultTheme() Theme {
	return Theme{
		Primary:      lipgloss.Color("#7dcfff"), // Cyan
		Secondary:    lipgloss.Color("#bb9af7"), // Violet
		Success:      lipgloss.Color("#9ece6a"), // Green
		Warning:      lipgloss.Color("#e0af68"), // Yellow/Orange
		Error:        lipgloss.Color("#f7768e"), // Red
		Info:         lipgloss.Color("#2ac3de"), // Blue
		Background:   lipgloss.Color("#1a1b26"), // Deep dark
		Surface:      lipgloss.Color("#24283b"), // Card bg
		SurfaceLight: lipgloss.Color("#2f3549"), // Hover bg
		Text:         lipgloss.Color("#c0caf5"), // Light text
		TextMuted:    lipgloss.Color("#565f89"), // Gray text
		TextDark:     lipgloss.Color("#15161e"), // Dark text for pills
		Border:       lipgloss.Color("#3b4261"), // Subdued border
		BorderFocus:  lipgloss.Color("#7dcfff"), // Focused border
	}
}

// Styles holds all pre-compiled Lipgloss styles for high-performance rendering.
type Styles struct {
	Theme Theme

	// Header & Tabs
	HeaderTitle lipgloss.Style
	HeaderVault lipgloss.Style
	TabActive   lipgloss.Style
	TabInactive lipgloss.Style
	TabBorder   lipgloss.Style
	TabBar      lipgloss.Style

	// Containers & Panels
	Panel        lipgloss.Style
	PanelFocused lipgloss.Style
	PanelTitle   lipgloss.Style
	SplitLeft    lipgloss.Style
	SplitRight   lipgloss.Style

	// Lists & Items
	ListItem         lipgloss.Style
	ListItemSelected lipgloss.Style
	ItemTitle        lipgloss.Style
	ItemTitleSel     lipgloss.Style
	ItemDesc         lipgloss.Style

	// Badges & Pills
	BadgeTech   lipgloss.Style
	BadgeDaily  lipgloss.Style
	BadgeMeta   lipgloss.Style
	BadgeSource lipgloss.Style
	BadgePerson lipgloss.Style
	BadgeInbox  lipgloss.Style
	BadgeClean  lipgloss.Style
	BadgeDirty  lipgloss.Style

	// Status & Footer Bar
	StatusBar  lipgloss.Style
	StatusKey  lipgloss.Style
	StatusDesc lipgloss.Style
	StatusMsg  lipgloss.Style

	// Search & Inputs
	SearchBox    lipgloss.Style
	SearchPrompt lipgloss.Style

	// Cards & Metrics
	MetricCard   lipgloss.Style
	MetricVal    lipgloss.Style
	MetricLbl    lipgloss.Style
	ProgressBar  lipgloss.Style
	ProgressFill lipgloss.Style

	// Help Overlay
	HelpModal lipgloss.Style
	HelpTitle lipgloss.Style
	HelpKey   lipgloss.Style
	HelpDesc  lipgloss.Style
}

// NewStyles constructs the compiled styling suite.
func NewStyles() Styles {
	theme := DefaultTheme()

	return Styles{
		Theme: theme,

		// Header & Tabs
		HeaderTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(theme.Primary).
			Padding(0, 1),

		HeaderVault: lipgloss.NewStyle().
			Foreground(theme.TextMuted).
			Padding(0, 1),

		TabActive: lipgloss.NewStyle().
			Bold(true).
			Foreground(theme.TextDark).
			Background(theme.Primary).
			Padding(0, 2),

		TabInactive: lipgloss.NewStyle().
			Foreground(theme.Text).
			Background(theme.Surface).
			Padding(0, 2),

		TabBorder: lipgloss.NewStyle().
			Foreground(theme.Border),

		TabBar: lipgloss.NewStyle().
			MarginBottom(1),

		// Panels
		Panel: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.Border).
			Padding(0, 1),

		PanelFocused: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.BorderFocus).
			Padding(0, 1),

		PanelTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(theme.Secondary).
			Padding(0, 1),

		SplitLeft: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.Border).
			Padding(0, 1),

		SplitRight: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.Border).
			Padding(0, 1),

		// Lists
		ListItem: lipgloss.NewStyle().
			Padding(0, 1),

		ListItemSelected: lipgloss.NewStyle().
			Background(theme.SurfaceLight).
			Foreground(theme.Primary).
			Bold(true).
			Padding(0, 1),

		ItemTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(theme.Text),

		ItemTitleSel: lipgloss.NewStyle().
			Bold(true).
			Foreground(theme.Primary),

		ItemDesc: lipgloss.NewStyle().
			Foreground(theme.TextMuted),

		// Badges
		BadgeTech: lipgloss.NewStyle().
			Background(lipgloss.Color("#2ac3de")).
			Foreground(theme.TextDark).
			Bold(true).
			Padding(0, 1),

		BadgeDaily: lipgloss.NewStyle().
			Background(lipgloss.Color("#e0af68")).
			Foreground(theme.TextDark).
			Bold(true).
			Padding(0, 1),

		BadgeMeta: lipgloss.NewStyle().
			Background(lipgloss.Color("#bb9af7")).
			Foreground(theme.TextDark).
			Bold(true).
			Padding(0, 1),

		BadgeSource: lipgloss.NewStyle().
			Background(lipgloss.Color("#73daca")).
			Foreground(theme.TextDark).
			Bold(true).
			Padding(0, 1),

		BadgePerson: lipgloss.NewStyle().
			Background(lipgloss.Color("#ff9e64")).
			Foreground(theme.TextDark).
			Bold(true).
			Padding(0, 1),

		BadgeInbox: lipgloss.NewStyle().
			Background(lipgloss.Color("#f7768e")).
			Foreground(theme.TextDark).
			Bold(true).
			Padding(0, 1),

		BadgeClean: lipgloss.NewStyle().
			Foreground(theme.Success).
			Bold(true),

		BadgeDirty: lipgloss.NewStyle().
			Foreground(theme.Warning).
			Bold(true),

		// Status Bar
		StatusBar: lipgloss.NewStyle().
			Background(theme.Surface).
			Foreground(theme.Text).
			Padding(0, 1),

		StatusKey: lipgloss.NewStyle().
			Bold(true).
			Foreground(theme.Primary),

		StatusDesc: lipgloss.NewStyle().
			Foreground(theme.TextMuted),

		StatusMsg: lipgloss.NewStyle().
			Foreground(theme.Success).
			Bold(true),

		// Search
		SearchBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.BorderFocus).
			Padding(0, 1),

		SearchPrompt: lipgloss.NewStyle().
			Bold(true).
			Foreground(theme.Primary),

		// Cards & Metrics
		MetricCard: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.Border).
			Background(theme.Surface).
			Padding(1, 2),

		MetricVal: lipgloss.NewStyle().
			Bold(true).
			Foreground(theme.Primary),

		MetricLbl: lipgloss.NewStyle().
			Foreground(theme.TextMuted),

		ProgressBar: lipgloss.NewStyle().
			Background(theme.SurfaceLight).
			Height(1),

		ProgressFill: lipgloss.NewStyle().
			Background(theme.Primary).
			Height(1),

		// Help Modal
		HelpModal: lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(theme.Primary).
			Background(theme.Surface).
			Padding(1, 3),

		HelpTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(theme.Secondary).
			MarginBottom(1),

		HelpKey: lipgloss.NewStyle().
			Bold(true).
			Foreground(theme.Primary),

		HelpDesc: lipgloss.NewStyle().
			Foreground(theme.Text),
	}
}
