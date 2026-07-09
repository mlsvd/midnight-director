package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/spinner"
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
)

type menuItem int

const (
	menuCommand menuItem = iota
	menuGetScreen
	menuConnect
	menuKill
	menuSummarize
	menuPrompt
)

var menuItems = []struct {
	item  menuItem
	label string
}{
	{menuConnect, "connect"},
	{menuCommand, "command"},
	{menuPrompt, "use prompt"},
	{menuGetScreen, "get screen"},
	{menuSummarize, "summarize"},
	{menuKill, "kill"},
}

type tickMsg time.Time
type pollMsg struct{ idx int }
type screenCaptureMsg struct {
	idx     int
	content string
}

type Model struct {
	sessions      []*session.Session
	focused       int
	mode          viewMode
	menuCursor    int
	input         textinput.Model
	viewport      viewport.Model
	spinner       spinner.Model
	help          help.Model
	screenText    string
	width         int
	height        int
	darkMode      bool
	theme         Theme
	autoSummarize bool
	aiCmd         string
	mySession     string // tmux session midnight-director itself runs in
	err           error
}

func New() Model {
	ti := textinput.New()
	ti.Placeholder = ""
	ti.CharLimit = 500

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))

	return Model{
		input:     ti,
		spinner:   sp,
		help:      help.New(),
		darkMode:  true,
		theme:     darkTheme(),
		aiCmd:     ai.Detect(),
		mySession: tmux.CurrentSession(),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		discoverSessions(),
		tickEvery(5*time.Second),
		m.spinner.Tick,
	)
}
