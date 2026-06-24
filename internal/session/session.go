package session

import (
	"strings"
	"time"

	"github.com/malisev/midnight-director/internal/tmux"
)

const StaleThreshold = 5 * time.Second
const SummaryDebounce = 4 * time.Second

type State int

const (
	StateIdle State = iota
	StateRunning
	StateDone
	StateWaiting
)

func (s State) String() string {
	switch s {
	case StateIdle:
		return "idle"
	case StateRunning:
		return "running"
	case StateDone:
		return "done"
	case StateWaiting:
		return "waiting"
	}
	return "unknown"
}

type Session struct {
	Name           string
	State          State
	PrevState      State
	Command        string
	Title          string // set by the process via OSC 2 escape sequence, read from pane_title
	Hint           string
	StreamChunk    string
	Summary          string // AI-generated one-sentence summary of last completed run
	IsSummarizing    bool   // true while AI is processing
	StableStateSince time.Time // when current done/waiting state was first entered
	Note           string
	IsClaude       bool
	LastOutput     string
	LastChangeTime time.Time
}

var shellCommands = map[string]bool{
	"bash": true, "zsh": true, "sh": true, "fish": true, "dash": true,
	"csh": true, "tcsh": true, "ksh": true,
}

func isShell(cmd string) bool {
	return shellCommands[strings.TrimLeft(cmd, "-")]
}

func Refresh(s *Session) error {
	s.PrevState = s.State

	cmd, err := tmux.CurrentCommand(s.Name)
	if err != nil {
		return err
	}
	s.Command = cmd
	s.IsClaude = cmd == "claude"

	// read pane title set via OSC 2 (\033]2;...\007) — works for any process
	if title, err := tmux.PaneTitle(s.Name); err == nil {
		s.Title = title
	}

	if isShell(cmd) {
		if s.PrevState != StateIdle {
			s.StableStateSince = time.Now()
		}
		s.State = StateIdle
		s.Hint = ""
		s.StreamChunk = ""
		return nil
	}

	pane, err := tmux.CapturePanePlain(s.Name)
	if err != nil {
		return err
	}

	// track output changes for running/done detection
	if pane != s.LastOutput {
		s.LastOutput = pane
		s.LastChangeTime = time.Now()
		s.StreamChunk = lastNonEmpty(tailLines(pane, 3))
	}


	// check if foreground process is blocked on terminal input
	shellPID, pidErr := tmux.PanePID(s.Name)
	if pidErr == nil && shellPID > 0 {
		if isWaitingForInput(shellPID) {
			if s.PrevState != StateWaiting {
				s.StableStateSince = time.Now()
			}
			s.State = StateWaiting
			s.Hint = extractHint(pane)
			s.StreamChunk = ""
			return nil
		}
	}

	// output unchanged for longer than StaleThreshold → done
	if !s.LastChangeTime.IsZero() && time.Since(s.LastChangeTime) > StaleThreshold {
		if s.PrevState != StateDone {
			s.StableStateSince = time.Now()
		}
		s.State = StateDone
		s.Hint = extractHint(pane)
		s.StreamChunk = ""
		return nil
	}

	s.State = StateRunning
	s.StableStateSince = time.Time{} // reset debounce — state is active again

	// clear stale summary when a new run begins
	if s.PrevState != StateRunning {
		s.Summary = ""
		s.IsSummarizing = false
	}

	return nil
}

func tailLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

func lastNonEmpty(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n "), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		l := strings.TrimSpace(lines[i])
		if l != "" {
			return l
		}
	}
	return ""
}

func extractHint(pane string) string {
	lines := strings.Split(strings.TrimRight(pane, "\n"), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		l := strings.TrimSpace(lines[i])
		if l == "" || l == ">" {
			continue
		}
		if len(l) > 60 {
			l = l[:60] + "…"
		}
		return l
	}
	return ""
}
