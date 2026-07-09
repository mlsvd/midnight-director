package picker

import (
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
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
	stateWarn
)

type pickerStyles struct {
	title     lipgloss.Style
	item      lipgloss.Style
	itemFocus lipgloss.Style
	hint      lipgloss.Style
	label     lipgloss.Style
	preview   lipgloss.Style
	highlight lipgloss.Style
	border    lipgloss.Color
}

func darkPickerStyles() pickerStyles {
	return pickerStyles{
		title:     lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99")),
		item:      lipgloss.NewStyle().Foreground(lipgloss.Color("255")),
		itemFocus: lipgloss.NewStyle().Foreground(lipgloss.Color("99")).Bold(true),
		hint:      lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		label:     lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
		preview:   lipgloss.NewStyle().Foreground(lipgloss.Color("222")).Italic(true),
		highlight: lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Bold(true),
		border:    lipgloss.Color("99"),
	}
}

func lightPickerStyles() pickerStyles {
	return pickerStyles{
		title:     lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("55")),
		item:      lipgloss.NewStyle().Foreground(lipgloss.Color("235")),
		itemFocus: lipgloss.NewStyle().Foreground(lipgloss.Color("55")).Bold(true),
		hint:      lipgloss.NewStyle().Foreground(lipgloss.Color("243")),
		label:     lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
		preview:   lipgloss.NewStyle().Foreground(lipgloss.Color("94")).Italic(true),
		highlight: lipgloss.NewStyle().Foreground(lipgloss.Color("130")).Bold(true),
		border:    lipgloss.Color("55"),
	}
}

// promptItem implements list.Item for a prompt.
type promptItem struct{ prompts.Prompt }

func (p promptItem) FilterValue() string { return p.Name + " " + p.Text }
func (p promptItem) Title() string       { return p.Name }
func (p promptItem) Description() string { return p.Text }

// compactDelegate renders one line per prompt using per-instance styles.
type compactDelegate struct {
	normal  lipgloss.Style
	focused lipgloss.Style
}

func (d compactDelegate) Height() int                              { return 1 }
func (d compactDelegate) Spacing() int                             { return 0 }
func (d compactDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d compactDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	p := item.(promptItem)
	if index == m.Index() {
		fmt.Fprint(w, d.focused.Render("› "+p.Name))
	} else {
		fmt.Fprint(w, d.normal.Render("  "+p.Name))
	}
}

type Model struct {
	list   list.Model
	state  state
	styles pickerStyles

	selected     prompts.Prompt
	placeholders []string
	fillIdx      int
	fillValues   map[string]string
	fillInput    textinput.Model

	editor   textarea.Model
	session  string
	warnText string // text pending user confirmation in stateWarn
	width    int
	height   int

	done   bool
	sendFn func(session, text string) error
}

func promptsToItems(ps []prompts.Prompt) []list.Item {
	items := make([]list.Item, len(ps))
	for i, p := range ps {
		items[i] = promptItem{p}
	}
	return items
}

func New(ps []prompts.Prompt, sess string, darkMode bool, sendFn func(string, string) error) Model {
	fill := textinput.New()
	fill.CharLimit = 500

	ta := textarea.New()
	ta.ShowLineNumbers = false
	ta.CharLimit = 0

	st := darkPickerStyles()
	if !darkMode {
		st = lightPickerStyles()
	}

	l := list.New(promptsToItems(ps), compactDelegate{normal: st.item, focused: st.itemFocus}, 60, 20)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(true)
	l.KeyMap.CursorUp = key.NewBinding(
		key.WithKeys("up", "k", "ctrl+p"),
		key.WithHelp("↑/k", "up"),
	)
	l.KeyMap.CursorDown = key.NewBinding(
		key.WithKeys("down", "j", "ctrl+n"),
		key.WithHelp("↓/j", "down"),
	)

	return Model{
		list:      l,
		styles:    st,
		fillInput: fill,
		editor:    ta,
		session:   sess,
		sendFn:    sendFn,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}
