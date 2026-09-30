package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/malisev/midnight-director/internal/ai"
	"github.com/malisev/midnight-director/internal/session"
	"github.com/malisev/midnight-director/internal/tmux"
)

type viewMode int

const (
	modeList viewMode = iota
	modeMenu
	modeNewSession
	modeCommandInput
	modeQuickInput
	modeScreenView
	modeScreenInput
	modeKillConfirm
	modeRenameInput
	modeAnnotateInput
	modeMarkSelect
)

type menuItem int

const (
	menuCommand menuItem = iota
	menuGetScreen
	menuConnect
	menuKill
	menuSummarize
	menuPrompt
	menuMark
	menuResetMark
)

type menuEntry struct {
	item  menuItem
	label string
}

// menuEntries returns the dialog actions for the focused session. "mark" and
// "reset mark" are mutually exclusive depending on whether the session is
// already marked.
func (m Model) menuEntries() []menuEntry {
	entries := []menuEntry{
		{menuConnect, "connect"},
		{menuCommand, "command"},
		{menuPrompt, "use prompt"},
		{menuGetScreen, "get screen"},
		{menuSummarize, "summarize"},
	}
	if len(m.sessions) > 0 && m.sessions[m.focused].Mark != "" {
		entries = append(entries, menuEntry{menuResetMark, "reset mark"})
	} else {
		entries = append(entries, menuEntry{menuMark, "mark"})
	}
	entries = append(entries, menuEntry{menuKill, "kill"})
	return entries
}

type tickMsg time.Time
type screenTickMsg time.Time
type pollMsg struct{ idx int }
type screenCaptureMsg struct {
	idx     int
	content string
}

type Model struct {
	sessions         []*session.Session
	focused          int
	mode             viewMode
	menuCursor       int
	markCursor       int
	input            textinput.Model
	screenInput      textarea.Model // multi-line composer for the preview overlay's "send" action
	viewport         viewport.Model
	spinner          spinner.Model
	help             help.Model
	screenViewport   viewport.Model
	width            int
	height           int
	darkMode         bool
	theme            Theme
	autoSummarize    bool
	aiCmd            string
	mySession        string // tmux session midnight-director itself runs in
	renamedSession   string // briefly set after rename to drive the moved indicator
	pickerFromScreen bool   // true if the open picker was launched from the screen overlay, not the list/menu — see pickerSentMsg handling
	err              error
}

// screenInputRows is the composer's fixed height in the preview overlay — tall
// enough to see a long prompt while composing, without eating the whole screen.
const screenInputRows = 6

func New() Model {
	ti := textinput.New()
	ti.Placeholder = ""
	ti.CharLimit = 500

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))

	ta := textarea.New()
	ta.ShowLineNumbers = false
	ta.Prompt = "  "
	ta.CharLimit = 0
	ta.SetHeight(screenInputRows)
	// "enter" is intercepted by handleScreenInputKey to submit — same convention as
	// every other single-line input in this app — so ctrl+j/alt+enter are bound here
	// for an explicit in-place newline instead.
	ta.KeyMap.InsertNewline = key.NewBinding(key.WithKeys("ctrl+j", "alt+enter"))

	return Model{
		input:       ti,
		screenInput: ta,
		spinner:     sp,
		help:        help.New(),
		darkMode:    true,
		theme:       darkTheme(),
		aiCmd:       ai.Detect(),
		mySession:   tmux.CurrentSession(),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		discoverSessions(),
		tickEvery(5*time.Second),
		m.spinner.Tick,
	)
}
