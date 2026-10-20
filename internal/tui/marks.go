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
	{"carrot", "carrot", lipgloss.Color("202"), lipgloss.Color("235")},
	{"yellow", "yellow", lipgloss.Color("220"), lipgloss.Color("235")},
	{"green", "green", lipgloss.Color("76"), lipgloss.Color("235")},
	{"cyan", "cyan", lipgloss.Color("44"), lipgloss.Color("235")},
	{"blue", "blue", lipgloss.Color("63"), lipgloss.Color("255")},
	{"purple", "purple", lipgloss.Color("135"), lipgloss.Color("235")},
	{"pink", "pink", lipgloss.Color("213"), lipgloss.Color("235")},
	{"brown", "brown", lipgloss.Color("94"), lipgloss.Color("255")},
	{"gray", "gray", lipgloss.Color("244"), lipgloss.Color("235")},
	{"white", "white", lipgloss.Color("255"), lipgloss.Color("235")},
	{"black", "black", lipgloss.Color("0"), lipgloss.Color("255")},
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

// emojiOption is an emoji tag a session can carry alongside (not instead of) its color mark.
// Unlike markOption there's no separate persisted key — the emoji itself is the persisted value,
// since it's already a short, stable, self-describing string.
type emojiOption struct {
	emoji string
	label string
}

var emojiOptions = []emojiOption{
	{"✅", "done"},
	{"▶️", "play"},
	{"⏸️", "paused"},
	{"💬", "comment"},
	{"🐙", "github"},
	{"🐛", "bug"},
	{"🚧", "wip"},
	{"👀", "review"},
	{"🔥", "urgent"},
	{"🚀", "ship"},
}
