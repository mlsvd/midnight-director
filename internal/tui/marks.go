package tui

import "github.com/charmbracelet/lipgloss"

type markOption struct {
	key   string // persisted value, stored on the tmux session
	label string
	bg    lipgloss.Color
	fg    lipgloss.Color // chosen for contrast against bg
}

var markOptions = []markOption{
	{"red", "red", lipgloss.Color("196"), lipgloss.Color("255")},
	{"orange", "orange", lipgloss.Color("208"), lipgloss.Color("235")},
	{"yellow", "yellow", lipgloss.Color("220"), lipgloss.Color("235")},
	{"green", "green", lipgloss.Color("76"), lipgloss.Color("235")},
	{"cyan", "cyan", lipgloss.Color("44"), lipgloss.Color("235")},
	{"blue", "blue", lipgloss.Color("63"), lipgloss.Color("255")},
	{"purple", "purple", lipgloss.Color("135"), lipgloss.Color("235")},
}

// markStyle returns the badge style for a mark key: a bold, padded pill
// using the mark's background with a foreground chosen for contrast.
func markStyle(key string) (lipgloss.Style, bool) {
	for _, o := range markOptions {
		if o.key == key {
			return lipgloss.NewStyle().Background(o.bg).Foreground(o.fg).Bold(true).Padding(0, 1), true
		}
	}
	return lipgloss.Style{}, false
}
