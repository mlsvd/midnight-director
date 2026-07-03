package picker

import (
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/malisev/midnight-director/internal/prompts"
)

type state int

const (
	stateList state = iota
	stateFill
	stateEdit
)

type Model struct {
	allPrompts []prompts.Prompt
	filtered   []prompts.Prompt
	cursor     int
	filter     textinput.Model
	state      state

	selected     prompts.Prompt
	placeholders []string
	fillIdx      int
	fillValues   map[string]string
	fillInput    textinput.Model

	editor  textarea.Model
	session string
	width   int
	height  int

	done   bool
	sendFn func(session, text string) error
}

func New(ps []prompts.Prompt, session string, sendFn func(string, string) error) Model {
	filter := textinput.New()
	filter.Placeholder = "filter prompts…"
	filter.Focus()

	fill := textinput.New()
	fill.CharLimit = 500

	ta := textarea.New()
	ta.ShowLineNumbers = false
	ta.CharLimit = 0

	return Model{
		allPrompts: ps,
		filtered:   ps,
		filter:     filter,
		fillInput:  fill,
		editor:     ta,
		session:    session,
		sendFn:     sendFn,
	}
}

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

var (
	borderColor        = lipgloss.Color("240")
	borderColorFocused = lipgloss.Color("99")
	styleTitle         = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99"))
	styleItem          = lipgloss.NewStyle().Foreground(lipgloss.Color("255"))
	styleItemFocused   = lipgloss.NewStyle().Foreground(lipgloss.Color("99")).Bold(true)
	styleHint          = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	styleLabel         = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	stylePreview       = lipgloss.NewStyle().Foreground(lipgloss.Color("222")).Italic(true)
	styleHighlight     = lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Bold(true)
)

func applyFilter(all []prompts.Prompt, q string) []prompts.Prompt {
	if q == "" {
		return all
	}
	q = strings.ToLower(q)
	var out []prompts.Prompt
	for _, p := range all {
		if strings.Contains(strings.ToLower(p.Name), q) || strings.Contains(strings.ToLower(p.Text), q) {
			out = append(out, p)
		}
	}
	return out
}
