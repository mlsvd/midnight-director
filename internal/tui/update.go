package tui

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/malisev/midnight-director/internal/actions"
	"github.com/malisev/midnight-director/internal/ai"
	"github.com/malisev/midnight-director/internal/session"
	"github.com/malisev/midnight-director/internal/tmux"
)

type sessionsDiscoveredMsg []*session.Session
type clearRenamedMsg struct{}
type sessionCreatedMsg *session.Session
type liveScreenMsg string
type screenReattachMsg string
type sessionRefsMsg []tmux.SessionRef
type summaryResultMsg struct {
	idx  int
	text string
}
type errMsg error
type clearActionFlashMsg struct{}
type actionResultMsg struct {
	name   string
	output string
	err    error
}

// runAction resolves the session's current tmux session_id and pane PID right before running —
// not when the dropdown was opened — so the script always sees up-to-date values.
func runAction(a actions.Action, sessionName string) tea.Cmd {
	return func() tea.Msg {
		id, _ := tmux.SessionID(sessionName)
		pid, _ := tmux.PanePID(sessionName)
		out, err := a.Run(sessionName, id, pid)
		return actionResultMsg{name: a.Name, output: string(out), err: err}
	}
}

func clearActionFlashAfter() tea.Cmd {
	return func() tea.Msg {
		time.Sleep(4 * time.Second)
		return clearActionFlashMsg{}
	}
}

// firstLine returns a short one-line summary for the action-result flash: the first non-empty
// line of the script's combined output if there is one (most useful when it failed loudly),
// falling back to the exec error itself.
func firstLine(output string, err error) string {
	for _, l := range strings.Split(output, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			return l
		}
	}
	if err != nil {
		return err.Error()
	}
	return ""
}

// discoverSessions fetches every tmux session and hydrates each one's state. Each hydration is
// several separate tmux subprocess round-trips (current command, pane title, capture-pane, pane
// PID, two tmux option reads) — done one session at a time, that serial exec overhead is what
// made the list visibly slow to populate with many sessions. Fanning them out across goroutines
// cuts the wall-clock cost to roughly the slowest single session instead of the sum of all of
// them, matching how the ongoing per-tick poll (pollSession) already runs one Cmd per session.
func discoverSessions() tea.Cmd {
	return func() tea.Msg {
		refs, err := tmux.ListSessionsWithIDs()
		if err != nil {
			return errMsg(err)
		}
		sessions := make([]*session.Session, len(refs))
		var wg sync.WaitGroup
		for i, ref := range refs {
			s := &session.Session{ID: ref.ID, Name: ref.Name}
			sessions[i] = s
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = session.Refresh(s)
				session.LoadMark(s)
				session.LoadEmoji(s)
			}()
		}
		wg.Wait()
		return sessionsDiscoveredMsg(sessions)
	}
}

// listSessionRefs polls tmux directly (bypassing this instance's in-memory session list) so
// reconcileSessions can notice sessions created, killed, or renamed by another midnight-director
// instance — or by a bare `tmux` command — since the last tick.
func listSessionRefs() tea.Cmd {
	return func() tea.Msg {
		refs, err := tmux.ListSessionsWithIDs()
		if err != nil {
			return nil // a transient tmux hiccup here shouldn't surface as an error popup — just try again next tick
		}
		return sessionRefsMsg(refs)
	}
}

func tickEvery(d time.Duration) tea.Cmd {
	return tea.Every(d, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// screenRefreshInterval is decoupled from tickEvery so the screen-overlay preview can refresh faster without also multiplying the list's per-session poll.
const screenRefreshInterval = 200 * time.Millisecond

func screenTickEvery() tea.Cmd {
	return tea.Every(screenRefreshInterval, func(t time.Time) tea.Msg {
		return screenTickMsg(t)
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
		content, err := tmux.CapturePaneRaw(name)
		if err != nil {
			content = "(error capturing screen)"
		}
		return screenCaptureMsg{idx: idx, content: content}
	}
}

// summarizeConcurrency caps how many AI summarize calls run at once. Each one is a real `claude
// -p`/equivalent subprocess making an LLM call; when several sessions go idle around the same
// time (common right after enabling auto-summarize, or right at startup if sessions were
// already idle), they'd otherwise all fire simultaneously and compete for the same CPU/network/
// API rate limits, making every one of them slower rather than actually running in parallel.
// Extra eligible sessions just wait — pollMsg re-checks them every tick until a slot frees up.
const summarizeConcurrency = 2

var summarizeSem = make(chan struct{}, summarizeConcurrency)

func summarizeSession(idx int, paneText, aiCmd string) tea.Cmd {
	return func() tea.Msg {
		summarizeSem <- struct{}{}
		defer func() { <-summarizeSem }()
		text, err := ai.Summarize(aiCmd, paneText)
		if err != nil {
			return summaryResultMsg{idx: idx, text: ""}
		}
		return summaryResultMsg{idx: idx, text: text}
	}
}

func refreshScreen(name string) tea.Cmd {
	return func() tea.Msg {
		content, err := tmux.CapturePaneRaw(name)
		if err != nil {
			return nil
		}
		return liveScreenMsg(content)
	}
}

func (m Model) screenViewportSize() (width, height int) {
	height = m.height - 4
	if height < 1 {
		height = 1
	}
	width = m.width
	if width < 1 {
		width = 1
	}
	return width, height
}

func (m *Model) setScreenContent(content string) {
	w, h := m.screenViewportSize()
	m.screenViewport.Width = w
	m.screenViewport.Height = h

	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lines[i] = ansi.Truncate(line, w, "")
	}

	for len(lines) > 0 && strings.TrimSpace(ansi.Strip(lines[len(lines)-1])) == "" {
		lines = lines[:len(lines)-1]
	}

	atBottom := m.screenViewport.AtBottom()
	m.screenViewport.SetContent(strings.Join(lines, "\n"))
	if atBottom {
		m.screenViewport.GotoBottom()
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

func indexOfSession(sessions []*session.Session, target *session.Session) int {
	for i, s := range sessions {
		if s == target {
			return i
		}
	}
	return -1
}

// sessionVisible reports whether s passes the active name filter (always true when no filter
// is set). Case-insensitive substring match on the session name.
func (m Model) sessionVisible(s *session.Session) bool {
	if m.filterQuery == "" {
		return true
	}
	return strings.Contains(strings.ToLower(s.Name), strings.ToLower(m.filterQuery))
}

// nextVisible returns the nearest index to from (moving by dir, which must be +1 or -1) whose
// session passes the filter, or from unchanged if there isn't one — so up/down navigation skips
// over sessions hidden by an active filter instead of landing on them.
func (m Model) nextVisible(from, dir int) int {
	i := from
	for {
		i += dir
		if i < 0 || i >= len(m.sessions) {
			return from
		}
		if m.sessionVisible(m.sessions[i]) {
			return i
		}
	}
}

// clampFocusedToFilter nudges m.focused to the nearest visible session when the currently
// focused one has been hidden by the filter (e.g. right after the query changes). If nothing
// matches at all, it leaves m.focused as-is — buildListContent shows a "no matches" message.
func (m *Model) clampFocusedToFilter() {
	if len(m.sessions) == 0 || m.filterQuery == "" || m.sessionVisible(m.sessions[m.focused]) {
		return
	}
	for d := 1; d < len(m.sessions); d++ {
		if i := m.focused + d; i < len(m.sessions) && m.sessionVisible(m.sessions[i]) {
			m.focused = i
			return
		}
		if i := m.focused - d; i >= 0 && m.sessionVisible(m.sessions[i]) {
			m.focused = i
			return
		}
	}
}

// sessionHydratedMsg is a no-op completion ping for hydrateSession — buildListContent reads the
// *session.Session fields live off the shared pointer, so the message only needs to exist to
// wake Update() up for a re-render; it carries nothing itself.
type sessionHydratedMsg struct{}

// hydrateSession fills in a newly-discovered session's state and tmux-persisted tags in the
// background. reconcileSessions runs synchronously on the main update goroutine (it's not a
// tea.Cmd) — doing this work there instead would block all key handling and rendering for as
// long as the tmux subprocess round-trips take, once per newly-seen session.
func hydrateSession(s *session.Session) tea.Cmd {
	return func() tea.Msg {
		_ = session.Refresh(s)
		session.LoadMark(s)
		session.LoadEmoji(s)
		return sessionHydratedMsg{}
	}
}

// reconcileSessions merges a fresh tmux session list into m.sessions, so sessions created,
// killed, or renamed by another midnight-director instance (or a bare tmux command) show up
// here without a restart. Matching is by tmux's stable session_id first — that's what lets a
// rename be recognized as a rename (preserving Note/Summary/Mark/in-flight state) rather than
// read as "killed + new session created". Falls back to matching by name for sessions
// discovered before IDs were tracked. Returns whether any session was renamed (so the caller
// can trigger the same "moved" flash a local rename gets) and any commands needed to hydrate
// genuinely new sessions in the background — see hydrateSession.
func (m *Model) reconcileSessions(refs []tmux.SessionRef) (bool, []tea.Cmd) {
	byID := make(map[string]*session.Session, len(m.sessions))
	byName := make(map[string]*session.Session, len(m.sessions))
	for _, s := range m.sessions {
		if s.ID != "" {
			byID[s.ID] = s
		}
		byName[s.Name] = s
	}

	var focusedSession *session.Session
	if m.focused < len(m.sessions) {
		focusedSession = m.sessions[m.focused]
	}

	renamed := false
	var newCmds []tea.Cmd
	seen := make(map[*session.Session]bool, len(refs))
	merged := make([]*session.Session, 0, len(refs))
	for _, ref := range refs {
		if s, ok := byID[ref.ID]; ok && !seen[s] {
			if s.Name != ref.Name {
				s.Name = ref.Name
				m.renamedSession = ref.Name
				renamed = true
			}
			merged = append(merged, s)
			seen[s] = true
			continue
		}
		if s, ok := byName[ref.Name]; ok && !seen[s] {
			s.ID = ref.ID // backfill — discovered before ID tracking existed
			merged = append(merged, s)
			seen[s] = true
			continue
		}
		// genuinely new — created directly via tmux, or by another midnight-director instance.
		// Appears in the list immediately with blank state; hydrateSession fills it in async.
		ns := &session.Session{ID: ref.ID, Name: ref.Name}
		merged = append(merged, ns)
		newCmds = append(newCmds, hydrateSession(ns))
	}

	m.sessions = merged
	sortSessions(m.sessions)

	if focusedSession != nil {
		if idx := indexOfSession(m.sessions, focusedSession); idx >= 0 {
			m.focused = idx
		}
	}
	m.clampFocused()

	return renamed, newCmds
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

// headerHeight returns how many rows the top-of-screen header occupies: just the title, or
// title + the active-filter banner — see renderFilterBanner — so the viewport shrinks to make
// room for it instead of overflowing the terminal by a line while a filter is set.
func (m Model) headerHeight() int {
	if m.filterQuery != "" {
		return 2
	}
	return 1
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m, cmd := m.innerUpdate(msg)

	if m.width > 0 {
		vpH := m.height - m.headerHeight() - m.commandBarHeight()
		if vpH < 1 {
			vpH = 1
		}
		if m.viewport.Height != vpH {
			m.viewport.Height = vpH
		}
		m.clampFocused()
		m.clampFocusedToFilter()
		content, extraLines := m.buildListContent()
		m.viewport.SetContent(content)
		if m.mode == modeFilter || m.mode == modeNewSession || m.mode == modeCommandInput {
			// these three render their input as a trailing line after every session row,
			// not tied to the focused row's position — always scroll to the true bottom
			// rather than relying on ensureFocusedVisible's focused-row heuristic.
			m.viewport.GotoBottom()
		} else {
			m.ensureFocusedVisible(extraLines)
		}
	}

	return m, cmd
}

func (m Model) innerUpdate(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		vpHeight := m.height - m.headerHeight() - m.commandBarHeight()
		if vpHeight < 1 {
			vpHeight = 1
		}
		m.viewport = viewport.New(m.width-2, vpHeight)
		m.help.Width = m.width - 6
		m.screenInput.SetWidth(m.width)
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
		m.setScreenContent(msg.content)
		m.mode = modeScreenView
		return m, screenTickEvery()

	case liveScreenMsg:
		m.setScreenContent(string(msg))
		return m, nil

	case screenReattachMsg:
		if len(m.sessions) > 0 {
			return m, refreshScreen(string(msg))
		}
		return m, nil

	case screenTickMsg:
		if (m.mode != modeScreenView && m.mode != modeScreenInput) || len(m.sessions) == 0 {
			return m, nil // overlay closed since this was scheduled — let the ticker die
		}
		return m, tea.Batch(refreshScreen(m.sessions[m.focused].Name), screenTickEvery())

	case tickMsg:
		var cmds []tea.Cmd
		for i, s := range m.sessions {
			cmds = append(cmds, pollSession(i, s))
		}
		cmds = append(cmds, listSessionRefs())
		cmds = append(cmds, tickEvery(5*time.Second))
		cmds = append(cmds, tea.SetWindowTitle(m.windowTitle()))
		return m, tea.Batch(cmds...)

	case sessionRefsMsg:
		renamed, newCmds := m.reconcileSessions(msg)
		if renamed {
			newCmds = append(newCmds, clearRenamedAfter())
		}
		return m, tea.Batch(newCmds...)

	case sessionHydratedMsg:
		return m, nil

	case errMsg:
		m.err = msg
		return m, nil

	case clearRenamedMsg:
		m.renamedSession = ""
		return m, nil

	case actionResultMsg:
		if msg.err != nil {
			detail := firstLine(msg.output, msg.err)
			m.actionFlash = "✗ " + msg.name + ": " + detail
		} else {
			m.actionFlash = "✓ " + msg.name
		}
		return m, clearActionFlashAfter()

	case clearActionFlashMsg:
		m.actionFlash = ""
		return m, nil

	case pickerSentMsg:
		fromScreen := m.pickerFromScreen
		m.pickerFromScreen = false
		if msg == "" {
			return m, nil // picker was cancelled — nothing sent
		}
		if fromScreen {
			m.mode = modeScreenView
			return m, refreshScreen(string(msg))
		}
		return m, connectToSession(string(msg), m.backHint())

	case tea.MouseMsg:
		if m.mode == modeScreenView {
			var cmd tea.Cmd
			m.screenViewport, cmd = m.screenViewport.Update(msg)
			return m, cmd
		}
		return m, nil

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
	case modeCommandInput, modeQuickInput:
		return m.handleInputKey(msg)
	case modeScreenInput:
		return m.handleScreenInputKey(msg)
	case modeScreenView:
		return m.handleScreenKey(msg)
	case modeKillConfirm:
		return m.handleKillKey(msg)
	case modeRenameInput:
		return m.handleRenameKey(msg)
	case modeAnnotateInput:
		return m.handleAnnotateKey(msg)
	case modeMarkSelect:
		return m.handleMarkSelectKey(msg)
	case modeActionSelect:
		return m.handleActionSelectKey(msg)
	case modeEmojiSelect:
		return m.handleEmojiSelectKey(msg)
	case modeFilter:
		return m.handleFilterKey(msg)
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
		m.focused = m.nextVisible(m.focused, -1)

	case "down", "j":
		m.focused = m.nextVisible(m.focused, 1)

	case "pgdown", "ctrl+d":
		for i := 0; i < m.viewportPageSize(); i++ {
			m.focused = m.nextVisible(m.focused, 1)
		}

	case "pgup", "ctrl+u":
		for i := 0; i < m.viewportPageSize(); i++ {
			m.focused = m.nextVisible(m.focused, -1)
		}

	case "/":
		m.mode = modeFilter
		m.input.SetValue(m.filterQuery)
		m.input.Placeholder = "filter sessions…"
		m.input.Focus()
		return m, textinput.Blink

	case "esc":
		m.filterQuery = ""

	case "o":
		if len(m.sessions) > 0 {
			return m, createSession(m.nextChildName())
		}

	case "x":
		if len(m.sessions) > 0 {
			m.mode = modeKillConfirm
		}

	case "m":
		if len(m.sessions) > 0 {
			m.mode = modeMarkSelect
			m.markCursor = markIndex(m.sessions[m.focused].Mark)
		}

	case "E":
		if len(m.sessions) > 0 {
			m.mode = modeEmojiSelect
			m.emojiCursor = emojiIndex(m.sessions[m.focused].Emoji)
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

	case "left", "h":
		if len(m.sessions) > 0 {
			opts, _ := actions.Load(actions.DefaultDir())
			if len(opts) > 0 {
				m.actionOptions = opts
				m.actionCursor = 0
				m.mode = modeActionSelect
			}
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
			return m, connectToSession(m.sessions[m.focused].Name, m.backHint())
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
	entries := m.menuEntries()
	switch msg.String() {
	case "up", "k":
		if m.menuCursor > 0 {
			m.menuCursor--
		}
	case "down", "j":
		if m.menuCursor < len(entries)-1 {
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

	switch m.menuEntries()[m.menuCursor].item {
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

	case menuMark:
		m.mode = modeMarkSelect
		m.markCursor = markIndex(s.Mark)

	case menuResetMark:
		_ = session.SetMark(s, "")
		m.mode = modeList

	case menuEmoji:
		m.mode = modeEmojiSelect
		m.emojiCursor = emojiIndex(s.Emoji)

	case menuResetEmoji:
		_ = session.SetEmoji(s, "")
		m.mode = modeList

	case menuKill:
		m.mode = modeKillConfirm
	}
	return m, nil
}

func (m Model) handleMarkSelectKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.markCursor > 0 {
			m.markCursor--
		}
	case "down", "j":
		if m.markCursor < len(markOptions)-1 {
			m.markCursor++
		}
	case "left", "esc":
		m.mode = modeList
	case "enter":
		if len(m.sessions) > 0 {
			_ = session.SetMark(m.sessions[m.focused], markOptions[m.markCursor].key)
		}
		m.mode = modeList
	}
	return m, nil
}

func (m Model) handleEmojiSelectKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.emojiCursor > 0 {
			m.emojiCursor--
		}
	case "down", "j":
		if m.emojiCursor < len(emojiOptions)-1 {
			m.emojiCursor++
		}
	case "left", "esc":
		m.mode = modeList
	case "enter":
		if len(m.sessions) > 0 {
			_ = session.SetEmoji(m.sessions[m.focused], emojiOptions[m.emojiCursor].emoji)
		}
		m.mode = modeList
	}
	return m, nil
}

func (m Model) handleActionSelectKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.actionCursor > 0 {
			m.actionCursor--
		}
	case "down", "j":
		if m.actionCursor < len(m.actionOptions)-1 {
			m.actionCursor++
		}
	case "right", "esc":
		m.mode = modeList
	case "enter":
		m.mode = modeList
		if len(m.sessions) > 0 && m.actionCursor < len(m.actionOptions) {
			a := m.actionOptions[m.actionCursor]
			s := m.sessions[m.focused]
			return m, runAction(a, s.Name)
		}
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
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// handleFilterKey drives the "/" session-name filter. It filters live as you type — every
// keystroke updates m.filterQuery immediately — rather than waiting for Enter. Enter just
// commits (returns to the list with the filter still applied); Esc clears it entirely.
func (m Model) handleFilterKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.filterQuery = ""
		m.input.SetValue("")
		m.input.Blur()
		m.mode = modeList
		return m, nil

	case "enter", "ctrl+j":
		m.filterQuery = strings.TrimSpace(m.input.Value())
		m.input.Blur()
		m.mode = modeList
		return m, nil
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.filterQuery = m.input.Value()
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
		return m, nil
	case "i", "enter":
		if len(m.sessions) > 0 {
			s := m.sessions[m.focused]
			m.mode = modeScreenInput
			m.screenInput.Reset()
			m.screenInput.Placeholder = "send to " + s.Name + "… (ctrl+j/alt+enter for a newline)"
			m.screenInput.SetWidth(m.width)
			return m, m.screenInput.Focus()
		}
		return m, nil
	case "p":
		if len(m.sessions) > 0 {
			m.pickerFromScreen = true // stay on the overlay after sending instead of attaching — see pickerSentMsg
			return m, openPicker(m.sessions[m.focused].Name, m.darkMode)
		}
		return m, nil
	case "c":
		if len(m.sessions) > 0 {
			return m, connectFromScreen(m.sessions[m.focused].Name, m.backHint())
		}
		return m, nil
	case "G", "end":
		m.screenViewport.GotoBottom()
		return m, nil
	case "g", "home":
		m.screenViewport.GotoTop()
		return m, nil
	}

	// anything else (up/down/k/j/pgup/pgdown/ctrl+u/ctrl+d, and " " when not otherwise bound) scrolls the pane history
	var cmd tea.Cmd
	m.screenViewport, cmd = m.screenViewport.Update(msg)
	return m, cmd
}

// handleScreenInputKey drives the multi-line composer opened from the preview overlay. "enter"
// (and its "ctrl+m" alias) sends, since that's the convention every other input in this app
// already uses; ctrl+j and alt+enter are bound as the textarea's own InsertNewline keys (see
// model.go) so typing a real newline needs a deliberate combo instead of colliding with send —
// they fall through to the m.screenInput.Update(msg) below rather than being handled here.
func (m Model) handleScreenInputKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeScreenView
		m.screenInput.Blur()
		return m, nil

	case "enter", "ctrl+m":
		val := strings.TrimSpace(m.screenInput.Value())
		m.screenInput.Blur()
		m.mode = modeScreenView
		if len(m.sessions) > 0 {
			s := m.sessions[m.focused]
			_ = tmux.SendKeys(s.Name, resolveFromRefs(val, s.Name))
			return m, refreshScreen(s.Name)
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.screenInput, cmd = m.screenInput.Update(msg)
	return m, cmd
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
				sortSessions(m.sessions)
				m.focused = indexByName(m.sessions, val)
				m.renamedSession = val
				m.mode = modeList
				return m, clearRenamedAfter()
			}
		}
		m.mode = modeList
		return m, nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func clearRenamedAfter() tea.Cmd {
	return func() tea.Msg {
		time.Sleep(1500 * time.Millisecond)
		return clearRenamedMsg{}
	}
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
		id, _ := tmux.SessionID(name) // best-effort — reconcileSessions backfills by name if this is empty
		s := &session.Session{ID: id, Name: name}
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

// ensureFocusedVisible scrolls the viewport so the focused row is visible. extraLines is the
// height of any popup (menu, mark/emoji/action select, inline input, kill confirm) currently
// open below that row — without it, opening a menu near the bottom of a long list would leave
// its items scrolled out of view instead of pulling them on screen. See buildListContent.
func (m *Model) ensureFocusedVisible(extraLines int) {
	if len(m.sessions) == 0 || m.viewport.Height == 0 {
		return
	}
	top := 0
	for i, s := range m.sessions {
		if i == m.focused {
			break
		}
		if !m.sessionVisible(s) {
			continue // not actually rendered — see buildListContent
		}
		top += sessionRows(s)
	}
	bottom := top + sessionRows(m.sessions[m.focused]) - 1 + extraLines
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

// attachSession blocks the TUI's event loop and hands the terminal to a real tmux client until the user detaches, then reports completion via onDone.
func attachSession(name, backHint string, onDone func() tea.Msg) tea.Cmd {
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
			return onDone()
		},
	)
}

func connectToSession(name, backHint string) tea.Cmd {
	return attachSession(name, backHint, func() tea.Msg { return nil })
}

// connectFromScreen attaches directly from the preview overlay; unlike connectToSession it returns to the (refreshed) overlay instead of the session list once the user detaches.
func connectFromScreen(name, backHint string) tea.Cmd {
	return attachSession(name, backHint, func() tea.Msg { return screenReattachMsg(name) })
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
			return pickerSentMsg("") // cancelled — still deliver a msg so pickerFromScreen gets reset
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
