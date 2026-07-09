package tui

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/malisev/midnight-director/internal/ai"
	"github.com/malisev/midnight-director/internal/session"
	"github.com/malisev/midnight-director/internal/tmux"
)

type sessionsDiscoveredMsg []*session.Session
type sessionCreatedMsg *session.Session
type liveScreenMsg string
type summaryResultMsg struct {
	idx  int
	text string
}
type errMsg error

func discoverSessions() tea.Cmd {
	return func() tea.Msg {
		names, err := tmux.ListSessions()
		if err != nil {
			return errMsg(err)
		}
		var sessions []*session.Session
		for _, n := range names {
			s := &session.Session{Name: n}
			_ = session.Refresh(s)
			sessions = append(sessions, s)
		}
		return sessionsDiscoveredMsg(sessions)
	}
}

func tickEvery(d time.Duration) tea.Cmd {
	return tea.Every(d, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func pollSession(idx int, s *session.Session) tea.Cmd {
	return func() tea.Msg {
		_ = session.Refresh(s)
		return pollMsg{idx: idx}
	}
}

func captureScreen(idx int, name string) tea.Cmd {
	return func() tea.Msg {
		content, err := tmux.CapturePanePlain(name)
		if err != nil {
			content = "(error capturing screen)"
		}
		return screenCaptureMsg{idx: idx, content: content}
	}
}

func summarizeSession(idx int, paneText, aiCmd string) tea.Cmd {
	return func() tea.Msg {
		text, err := ai.Summarize(aiCmd, paneText)
		if err != nil {
			return summaryResultMsg{idx: idx, text: ""}
		}
		return summaryResultMsg{idx: idx, text: text}
	}
}

func refreshScreen(name string) tea.Cmd {
	return func() tea.Msg {
		content, err := tmux.CapturePanePlain(name)
		if err != nil {
			return nil
		}
		return liveScreenMsg(content)
	}
}

func rootName(name string) string {
	if idx := strings.Index(name, "/"); idx >= 0 {
		return name[:idx]
	}
	return name
}

func sortSessions(sessions []*session.Session) {
	sort.SliceStable(sessions, func(i, j int) bool {
		ni, nj := sessions[i].Name, sessions[j].Name
		ri, rj := rootName(ni), rootName(nj)
		if ri != rj {
			return ri < rj
		}
		// same root: parent before its children
		if strings.HasPrefix(nj, ni+"/") {
			return true
		}
		if strings.HasPrefix(ni, nj+"/") {
			return false
		}
		return ni < nj
	})
}

func (m Model) nextChildName() string {
	parent := m.sessions[m.focused].Name
	prefix := parent + "/"
	max := 0
	for _, s := range m.sessions {
		if !strings.HasPrefix(s.Name, prefix) {
			continue
		}
		rest := strings.TrimPrefix(s.Name, prefix)
		if strings.Contains(rest, "/") {
			continue // skip grandchildren
		}
		if n, err := strconv.Atoi(rest); err == nil && n > max {
			max = n
		}
	}
	return fmt.Sprintf("%s/%d", parent, max+1)
}

func indexByName(sessions []*session.Session, name string) int {
	for i, s := range sessions {
		if s.Name == name {
			return i
		}
	}
	return len(sessions) - 1
}

func (m Model) windowTitle() string {
	running := 0
	for _, s := range m.sessions {
		if s.State == session.StateRunning {
			running++
		}
	}
	if running > 0 {
		return fmt.Sprintf("midnight director · %d running", running)
	}
	return fmt.Sprintf("midnight director · %d sessions", len(m.sessions))
}

// commandBarHeight returns how many terminal rows the command bar occupies
// (top border + full help content lines).
func (m Model) commandBarHeight() int {
	maxRows := 0
	for _, g := range m.helpKeys().FullHelp() {
		if len(g) > maxRows {
			maxRows = len(g)
		}
	}
	return 2 + maxRows // 1 top border + help rows + 1 tips line
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m, cmd := m.innerUpdate(msg)

	if m.width > 0 {
		vpH := m.height - 1 - m.commandBarHeight()
		if vpH < 1 {
			vpH = 1
		}
		if m.viewport.Height != vpH {
			m.viewport.Height = vpH
		}
		m.clampFocused()
		m.viewport.SetContent(m.buildListContent())
		m.ensureFocusedVisible()
	}

	return m, cmd
}

func (m Model) innerUpdate(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		vpHeight := m.height - 1 - m.commandBarHeight()
		if vpHeight < 1 {
			vpHeight = 1
		}
		m.viewport = viewport.New(m.width-2, vpHeight)
		m.help.Width = m.width - 6
		return m, nil

	case tea.ResumeMsg:
		return m, tea.Batch(discoverSessions(), tickEvery(5*time.Second), m.spinner.Tick)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case sessionsDiscoveredMsg:
		m.sessions = msg
		sortSessions(m.sessions)
		return m, tea.SetWindowTitle(m.windowTitle())

	case sessionCreatedMsg:
		m.sessions = append(m.sessions, msg)
		sortSessions(m.sessions)
		m.focused = indexByName(m.sessions, msg.Name)
		return m, tea.SetWindowTitle(m.windowTitle())

	case pollMsg:
		if msg.idx < len(m.sessions) {
			s := m.sessions[msg.idx]
			if m.autoSummarize && m.aiCmd != "" &&
				(s.State == session.StateDone || s.State == session.StateWaiting || s.State == session.StateIdle) &&
				!s.StableStateSince.IsZero() &&
				time.Since(s.StableStateSince) >= session.SummaryDebounce &&
				!s.IsSummarizing && s.Summary == "" {
				s.IsSummarizing = true
				return m, summarizeSession(msg.idx, s.LastOutput, m.aiCmd)
			}
		}
		return m, nil

	case summaryResultMsg:
		if msg.idx < len(m.sessions) {
			s := m.sessions[msg.idx]
			s.IsSummarizing = false
			s.Summary = msg.text
		}
		return m, nil

	case screenCaptureMsg:
		m.screenText = msg.content
		m.mode = modeScreenView
		return m, nil

	case liveScreenMsg:
		m.screenText = string(msg)
		return m, nil

	case tickMsg:
		var cmds []tea.Cmd
		for i, s := range m.sessions {
			cmds = append(cmds, pollSession(i, s))
		}
		if (m.mode == modeScreenView || m.mode == modeScreenInput) && len(m.sessions) > 0 {
			cmds = append(cmds, refreshScreen(m.sessions[m.focused].Name))
		}
		cmds = append(cmds, tickEvery(5*time.Second))
		cmds = append(cmds, tea.SetWindowTitle(m.windowTitle()))
		return m, tea.Batch(cmds...)

	case errMsg:
		m.err = msg
		return m, nil

	case pickerSentMsg:
		return m, connectToSession(string(msg), m.backHint())

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m Model) handleKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch m.mode {
	case modeList:
		return m.handleListKey(msg)
	case modeMenu:
		return m.handleMenuKey(msg)
	case modeNewSession:
		return m.handleNewSessionKey(msg)
	case modeCommandInput, modeQuickInput, modeScreenInput:
		return m.handleInputKey(msg)
	case modeScreenView:
		return m.handleScreenKey(msg)
	case modeKillConfirm:
		return m.handleKillKey(msg)
	case modeRenameInput:
		return m.handleRenameKey(msg)
	case modeAnnotateInput:
		return m.handleAnnotateKey(msg)
	}
	return m, nil
}

func (m Model) handleListKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "ctrl+z":
		return m, tea.Suspend

	case "up", "k":
		if m.focused > 0 {
			m.focused--
		}

	case "down", "j":
		if m.focused < len(m.sessions)-1 {
			m.focused++
		}

	case "pgdown", "ctrl+d":
		if len(m.sessions) > 0 {
			m.focused += m.viewportPageSize()
			if m.focused >= len(m.sessions) {
				m.focused = len(m.sessions) - 1
			}
		}

	case "pgup", "ctrl+u":
		m.focused -= m.viewportPageSize()
		if m.focused < 0 {
			m.focused = 0
		}

	case " ":
		if len(m.sessions) > 0 {
			return m, captureScreen(m.focused, m.sessions[m.focused].Name)
		}

	case "p":
		if len(m.sessions) > 0 {
			return m, openPicker(m.sessions[m.focused].Name, m.darkMode)
		}

	case "g":
		if m.aiCmd != "" && len(m.sessions) > 0 {
			s := m.sessions[m.focused]
			if !s.IsSummarizing {
				s.IsSummarizing = true
				s.Summary = ""
				return m, summarizeSession(m.focused, s.LastOutput, m.aiCmd)
			}
		}

	case "right", "l":
		if len(m.sessions) > 0 {
			m.mode = modeMenu
			m.menuCursor = 0
		}

	case "i", "enter":
		if len(m.sessions) > 0 {
			s := m.sessions[m.focused]
			if s.State == session.StateWaiting {
				m.mode = modeQuickInput
				m.input.SetValue("")
				m.input.Placeholder = "send to " + s.Name + "…"
				m.input.Focus()
				return m, textinput.Blink
			}
		}

	case "e":
		if len(m.sessions) > 0 {
			s := m.sessions[m.focused]
			m.mode = modeRenameInput
			m.input.SetValue(s.Name)
			m.input.Placeholder = "new name"
			m.input.Focus()
			return m, textinput.Blink
		}

	case "a":
		if len(m.sessions) > 0 {
			s := m.sessions[m.focused]
			m.mode = modeAnnotateInput
			m.input.SetValue(s.Note)
			m.input.Placeholder = "note…"
			m.input.Focus()
			return m, textinput.Blink
		}

	case "c":
		if len(m.sessions) > 0 {
			return m, createSession(m.nextChildName())
		}

	case "n":
		m.mode = modeNewSession
		m.input.SetValue("")
		m.input.Placeholder = "session name…"
		m.input.Focus()
		return m, textinput.Blink

	case "t":
		m.darkMode = !m.darkMode
		if m.darkMode {
			m.theme = darkTheme()
		} else {
			m.theme = lightTheme()
		}

	case "s":
		if m.aiCmd != "" {
			m.autoSummarize = !m.autoSummarize
		}
	}

	return m, nil
}

func (m Model) handleMenuKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.menuCursor > 0 {
			m.menuCursor--
		}
	case "down", "j":
		if m.menuCursor < len(menuItems)-1 {
			m.menuCursor++
		}
	case "left", "esc":
		m.mode = modeList
	case "enter":
		return m.executeMenuItem()
	}
	return m, nil
}

func (m Model) executeMenuItem() (Model, tea.Cmd) {
	if len(m.sessions) == 0 {
		m.mode = modeList
		return m, nil
	}
	s := m.sessions[m.focused]

	switch menuItems[m.menuCursor].item {
	case menuCommand:
		m.mode = modeCommandInput
		m.input.SetValue("")
		m.input.Placeholder = "command to run in " + s.Name
		m.input.Focus()
		return m, textinput.Blink

	case menuGetScreen:
		m.mode = modeList
		return m, captureScreen(m.focused, s.Name)

	case menuConnect:
		m.mode = modeList
		return m, connectToSession(s.Name, m.backHint())

	case menuSummarize:
		m.mode = modeList
		if m.aiCmd != "" && !s.IsSummarizing {
			s.IsSummarizing = true
			s.Summary = ""
			return m, summarizeSession(m.focused, s.LastOutput, m.aiCmd)
		}

	case menuPrompt:
		m.mode = modeList
		return m, openPicker(s.Name, m.darkMode)

	case menuKill:
		m.mode = modeKillConfirm
	}
	return m, nil
}

func (m Model) handleNewSessionKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeList
		m.input.Blur()
		return m, nil
	case "enter", "ctrl+j":
		name := strings.TrimSpace(m.input.Value())
		m.input.Blur()
		m.mode = modeList
		if name != "" {
			return m, createSession(name)
		}
		return m, nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m Model) handleInputKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeList
		m.input.Blur()
		return m, nil

	case "enter", "ctrl+j":
		val := strings.TrimSpace(m.input.Value())
		m.input.Blur()

		switch m.mode {
		case modeCommandInput:
			if len(m.sessions) > 0 {
				s := m.sessions[m.focused]
				_ = tmux.SendKeys(s.Name, resolveFromRefs(val, s.Name))
			}
			m.mode = modeList

		case modeQuickInput:
			if len(m.sessions) > 0 {
				s := m.sessions[m.focused]
				_ = tmux.SendKeys(s.Name, resolveFromRefs(val, s.Name))
			}
			m.mode = modeList

		case modeScreenInput:
			if len(m.sessions) > 0 {
				s := m.sessions[m.focused]
				_ = tmux.SendKeys(s.Name, resolveFromRefs(val, s.Name))
			}
			m.mode = modeList
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m Model) viewportPageSize() int {
	n := m.viewport.Height / 2
	if n < 1 {
		n = 1
	}
	return n
}

func (m Model) handleScreenKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q", " ":
		m.mode = modeList
	case "i", "enter":
		if len(m.sessions) > 0 {
			s := m.sessions[m.focused]
			m.mode = modeScreenInput
			m.input.SetValue("")
			m.input.Placeholder = "send to " + s.Name + "…"
			m.input.Focus()
			return m, textinput.Blink
		}
	}
	return m, nil
}

func (m Model) handleKillKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		if len(m.sessions) > 0 {
			s := m.sessions[m.focused]
			_ = tmux.KillSession(s.Name)
			m.sessions = append(m.sessions[:m.focused], m.sessions[m.focused+1:]...)
			if m.focused >= len(m.sessions) && m.focused > 0 {
				m.focused--
			}
		}
		m.mode = modeList
		return m, tea.SetWindowTitle(m.windowTitle())
	default:
		m.mode = modeList
	}
	return m, nil
}

func (m Model) handleRenameKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeList
		m.input.Blur()
		return m, nil
	case "enter":
		val := strings.TrimSpace(m.input.Value())
		m.input.Blur()
		if val != "" && len(m.sessions) > 0 {
			s := m.sessions[m.focused]
			if err := tmux.RenameSession(s.Name, val); err == nil {
				s.Name = val
			}
		}
		m.mode = modeList
		return m, nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m Model) handleAnnotateKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeList
		m.input.Blur()
		return m, nil
	case "enter":
		val := m.input.Value()
		m.input.Blur()
		if len(m.sessions) > 0 {
			m.sessions[m.focused].Note = val
		}
		m.mode = modeList
		return m, nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func createSession(name string) tea.Cmd {
	return func() tea.Msg {
		if err := tmux.NewSession(name); err != nil {
			return errMsg(err)
		}
		s := &session.Session{Name: name}
		_ = session.Refresh(s)
		return sessionCreatedMsg(s)
	}
}

func sessionRows(s *session.Session) int {
	if s.Note != "" {
		return 2
	}
	return 1
}

func (m *Model) clampFocused() {
	if len(m.sessions) == 0 {
		m.focused = 0
		return
	}
	if m.focused < 0 {
		m.focused = 0
	} else if m.focused >= len(m.sessions) {
		m.focused = len(m.sessions) - 1
	}
}

func (m *Model) ensureFocusedVisible() {
	if len(m.sessions) == 0 || m.viewport.Height == 0 {
		return
	}
	top := 0
	for i, s := range m.sessions {
		if i == m.focused {
			break
		}
		top += sessionRows(s)
	}
	bottom := top + sessionRows(m.sessions[m.focused]) - 1
	if top < m.viewport.YOffset {
		m.viewport.SetYOffset(top)
	} else if bottom >= m.viewport.YOffset+m.viewport.Height {
		m.viewport.SetYOffset(bottom - m.viewport.Height + 1)
	}
}

func (m Model) backHint() string {
	if m.mySession == "" {
		return ""
	}
	return "M-b · back to midnight-director"
}

func connectToSession(name, backHint string) tea.Cmd {
	if backHint != "" {
		_ = exec.Command("tmux", "set-hook", "-t", name, "client-attached",
			fmt.Sprintf("display-message '%s'", backHint)).Run()
	}
	return tea.ExecProcess(
		exec.Command("tmux", "attach-session", "-t", name),
		func(err error) tea.Msg {
			if backHint != "" {
				_ = exec.Command("tmux", "set-hook", "-u", "-t", name, "client-attached").Run()
			}
			return nil
		},
	)
}

type pickerSentMsg string

func openPicker(sessionName string, darkMode bool) tea.Cmd {
	self, _ := os.Executable()
	theme := "dark"
	if !darkMode {
		theme = "light"
	}
	sentPath := pickerSentPath()
	return tea.ExecProcess(
		exec.Command(self, "--picker", sessionName, theme),
		func(err error) tea.Msg {
			data, readErr := os.ReadFile(sentPath)
			os.Remove(sentPath)
			if readErr == nil && len(data) > 0 {
				return pickerSentMsg(strings.TrimSpace(string(data)))
			}
			return nil
		},
	)
}

func pickerSentPath() string {
	return fmt.Sprintf("%s/.midnight-director-picker-sent", os.TempDir())
}

// resolveFromRefs replaces {{from:target}} and {{from:target:Nl}} references
// in text with live pane content captured from the target tmux session.
func resolveFromRefs(text, currentSession string) string {
	const open, close = "{{from:", "}}"
	for {
		start := strings.Index(text, open)
		if start < 0 {
			break
		}
		end := strings.Index(text[start:], close)
		if end < 0 {
			break
		}
		end += start + len(close)
		inner := text[start+len(open) : end-len(close)] // "target" or "target:Nl"

		target, lineSpec, _ := strings.Cut(inner, ":")
		if target == "parent" {
			if idx := strings.LastIndex(currentSession, "/"); idx >= 0 {
				target = currentSession[:idx]
			} else {
				text = text[:start] + text[end:] // no parent — remove placeholder
				continue
			}
		}

		content, err := tmux.CapturePanePlain(target)
		if err != nil {
			text = text[:start] + text[end:]
			continue
		}
		content = strings.TrimRight(content, "\n")

		if lineSpec != "" && strings.HasSuffix(lineSpec, "l") {
			if n, err2 := strconv.Atoi(strings.TrimSuffix(lineSpec, "l")); err2 == nil && n > 0 {
				lines := strings.Split(content, "\n")
				if len(lines) > n {
					lines = lines[len(lines)-n:]
				}
				content = strings.Join(lines, "\n")
			}
		}

		text = text[:start] + content + text[end:]
	}
	return text
}
