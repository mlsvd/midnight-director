package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/malisev/midnight-director/internal/ai"
	"github.com/malisev/midnight-director/internal/session"
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
)

var menuItems = []struct {
	item  menuItem
	label string
}{
	{menuConnect, "connect"},
	{menuCommand, "command"},
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
	screenText    string
	spinnerTick   int
	width         int
	height        int
	darkMode      bool
	theme         Theme
	autoSummarize bool
	aiCmd         string
	err           error
}

func New() Model {
	ti := textinput.New()
	ti.Placeholder = ""
	ti.CharLimit = 500

	return Model{
		input:    ti,
		darkMode: true,
		theme:    darkTheme(),
		aiCmd:    ai.Detect(),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		discoverSessions(),
		tickEvery(5*time.Second),
	)
}
