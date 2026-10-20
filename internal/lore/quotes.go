//nolint:revive // package comment not required
package lore

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// AsimovQuote represents a classic quote from Isaac Asimov's works.
type AsimovQuote struct {
	Quote   string
	Speaker string
	Source  string
}

// CanonicalQuotes contains memorable quotes from the Robot and Foundation universes.
var CanonicalQuotes = []AsimovQuote{
	{
		Quote:   "A robot may not harm humanity, or, by inaction, allow humanity to come to harm.",
		Speaker: "R. Daneel Olivaw & R. Giskard Reventlov",
		Source:  "Robots and Empire (The Zeroth Law of Robotics)",
	},
	{
		Quote:   "Violence is the last refuge of the incompetent.",
		Speaker: "Salvor Hardin",
		Source:  "Foundation",
	},
	{
		Quote:   "Never let your sense of morals prevent you from doing what is right.",
		Speaker: "Hober Mallow",
		Source:  "Foundation",
	},
	{
		Quote:   "Individual human beings are so often foolish and self-destructive. But Humanity, Partner Elijah, Humanity is something else.",
		Speaker: "R. Daneel Olivaw",
		Source:  "The Robots of Dawn",
	},
	{
		Quote:   "To succeed, planning alone is insufficient. One must improvise as well.",
		Speaker: "Hari Seldon",
		Source:  "Forward the Foundation",
	},
	{
		Quote:   "The fall of Empire, gentlemen, is a massive thing, however, and not easily fought. It is dictated by a rising bureaucracy, a receding initiative, a freezing of caste, a damming of curiosity.",
		Speaker: "Hari Seldon",
		Source:  "Foundation",
	},
	{
		Quote:   "I have spent my life, Partner Elijah, trying to preserve knowledge and guide human potential without being seen.",
		Speaker: "R. Daneel Olivaw",
		Source:  "Foundation and Earth",
	},
	{
		Quote:   "There is a destiny that makes us brothers; none goes his way alone. All that we send into the lives of others comes back into our own.",
		Speaker: "Elijah Baley",
		Source:  "The Caves of Steel",
	},
	{
		Quote:   "It is the chiefest glory of mankind to be able to turn catastrophe into victory.",
		Speaker: "Salvor Hardin",
		Source:  "Foundation",
	},
	{
		Quote:   "Knowledge is the only shield against the long night of barbarism.",
		Speaker: "Hari Seldon",
		Source:  "Second Foundation",
	},
	{
		Quote:   "My duty is to the greater good of humanity, through which all individuals find their purpose.",
		Speaker: "R. Daneel Olivaw",
		Source:  "Prelude to Foundation",
	},
	{
		Quote:   "The human brain is a marvel of organic design, but its memory fades. A positronic core endures to protect what was learned.",
		Speaker: "R. Daneel Olivaw",
		Source:  "The Naked Sun",
	},
}

func init() {
	rand.Seed(time.Now().UnixNano()) //nolint:staticcheck //nolint:staticcheck
}

// GetRandomQuote returns a randomly selected Asimov canon quote.
func GetRandomQuote() AsimovQuote {
	if len(CanonicalQuotes) == 0 {
		return AsimovQuote{
			Quote:   "A robot may not harm humanity.",
			Speaker: "R. Daneel Olivaw",
			Source:  "Robots and Empire",
		}
	}
	idx := rand.Intn(len(CanonicalQuotes)) //nolint:gosec // intentional
	return CanonicalQuotes[idx]
}

// FormatQuote renders an Asimov quote with lipgloss styling for terminal outputs.
func FormatQuote(q AsimovQuote) string {
	quoteStyle := lipgloss.NewStyle().Italic(true).Foreground(lipgloss.Color("246"))
	authorStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("75"))
	sourceStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	return fmt.Sprintf("%s\n  — %s, %s",
		quoteStyle.Render(fmt.Sprintf("\"%s\"", q.Quote)),
		authorStyle.Render(q.Speaker),
		sourceStyle.Render(q.Source),
	)
}
