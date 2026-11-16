package tmux

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
)

func ListSessions() ([]string, error) {
	out, err := exec.Command("tmux", "ls", "-F", "#{session_name}").Output()
	if err != nil {
		// tmux returns exit code 1 when no sessions exist
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return nil, nil
		}
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var sessions []string
	for _, l := range lines {
		if l != "" {
			sessions = append(sessions, l)
		}
	}
	return sessions, nil
}

// PaneInfo is one session's live tmux state, as gathered in bulk by ListPanesAll instead of the
// 4 separate per-session subprocess calls (display-message ×3, show-options ×2) this used to
// take. WindowActivity is the unix-seconds timestamp tmux itself tracks for "last time this
// pane produced output" — session.RefreshFast uses it to skip the much more expensive
// capture-pane call entirely for sessions that haven't changed since the last poll.
type PaneInfo struct {
	SessionID      string
	SessionName    string
	Command        string
	PID            int
	Title          string
	WindowActivity int64
	Mark           string
	Emoji          string
}

// panesFieldCount is how many "\t"-joined tokens ListPanesAll's format string emits. Keep this
// in sync with the field list below — it's also exactly how many elements SplitN is told to
// produce, so a short line (fewer than panesFieldCount-1 tabs) still parses as best-effort
// instead of being silently discarded.
const panesFieldCount = 10

// ListPanesAll fetches live state for every session on the server in a single tmux subprocess
// call, rather than one call per session per field. A session may have more than one window or
// pane if something outside midnight-director split it — in that case this picks the active
// pane (window_active && pane_active), matching what a plain `-t session` reference already
// resolves to elsewhere in this package, so callers don't need to care about the distinction.
//
// The separator between fields matters more than it looks: an earlier version used "\x01" on
// the theory that a raw control byte could never collide with real pane content, but tmux
// itself doesn't treat that assumption uniformly — some versions pass embedded control bytes
// through a -F format string untouched, others (confirmed on tmux 3.4) escape them to the
// literal 4-character sequence "\001", which then never matches split("\x01") at all. Every
// line fails to parse, ListPanesAll silently returns an empty map with no error, and the
// session list looks identical to "there really are zero sessions" — this is exactly the "md
// shows no sessions but tmux list-sessions has them" bug. "\t" doesn't have this problem (tmux
// passes it through as a real tab on every version tested), but it can legitimately appear in
// free-text fields a user controls (a session name, a pane title) — so those go last and are
// parsed with SplitN, meaning a stray tab can only ever bleed into the final field's own
// content, never shift or corrupt the structured fields that come before it.
func ListPanesAll() (map[string]PaneInfo, error) {
	const sep = "\t"
	format := strings.Join([]string{
		"#{session_id}", "#{pane_pid}", "#{window_activity}", "#{window_active}",
		"#{pane_active}", "#{@md-mark}", "#{@md-emoji}",
		"#{session_name}", "#{pane_current_command}", "#{pane_title}",
	}, sep)

	out, err := exec.Command("tmux", "list-panes", "-a", "-F", format).Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return map[string]PaneInfo{}, nil // no sessions
		}
		return nil, err
	}

	return parsePanesOutput(string(out))
}

// parsePanesOutput is split out from ListPanesAll purely so the format-mismatch detection below
// can be exercised directly in tests against synthetic input, without needing a tmux binary
// whose exact field-escaping behavior varies by version to reproduce the failure.
func parsePanesOutput(out string) (map[string]PaneInfo, error) {
	const sep = "\t"
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	result := make(map[string]PaneInfo)
	totalLines, parsedRows := 0, 0
	var sampleBadLine string

	for _, line := range lines {
		if line == "" {
			continue
		}
		totalLines++
		f := strings.SplitN(line, sep, panesFieldCount)
		if len(f) < panesFieldCount {
			if sampleBadLine == "" {
				sampleBadLine = line
			}
			continue
		}
		parsedRows++

		id := f[0]
		isActivePane := f[3] == "1" && f[4] == "1"
		if _, seen := result[id]; seen && !isActivePane {
			continue // already have a row for this session; this one isn't its active pane
		}
		pid, _ := strconv.Atoi(f[1])
		activity, _ := strconv.ParseInt(f[2], 10, 64)
		result[id] = PaneInfo{
			SessionID:      id,
			PID:            pid,
			WindowActivity: activity,
			Mark:           f[5],
			Emoji:          f[6],
			SessionName:    f[7],
			Command:        f[8],
			Title:          f[9],
		}
	}

	// tmux printed real output but not one line of it matched the expected shape — that's a
	// format incompatibility, not "no sessions," and must not look like one (see the long
	// comment above). Surfacing it as an error is what makes it show up in the UI instead of
	// silently rendering an empty list.
	if totalLines > 0 && parsedRows == 0 {
		return nil, fmt.Errorf("list-panes output didn't match the expected %d-field format (got e.g. %q)", panesFieldCount, sampleBadLine)
	}

	return result, nil
}

func NewSession(name string) error {
	// Ensure the server is running before creating a session. On macOS over SSH,
	// new-session hangs when it has to start the daemon itself; start-server avoids that.
	exec.Command("tmux", "start-server").Run() //nolint:errcheck
	return exec.Command("tmux", "new-session", "-d", "-s", name).Run()
}

func KillSession(name string) error {
	return exec.Command("tmux", "kill-session", "-t", name).Run()
}

func RenameSession(old, newName string) error {
	return exec.Command("tmux", "rename-session", "-t", old, newName).Run()
}

var sendKeysSeq int64

// SendKeys pastes text via tmux's paste buffer (-r/-p) rather than send-keys directly, so embedded newlines don't fire as premature Enter submissions, then sends a real Enter to submit.
func SendKeys(session, keys string) error {
	if keys == "" {
		return exec.Command("tmux", "send-keys", "-t", session, "Enter").Run()
	}

	buf := fmt.Sprintf("md-%d-%d", os.Getpid(), atomic.AddInt64(&sendKeysSeq, 1))
	if err := exec.Command("tmux", "set-buffer", "-b", buf, "--", keys).Run(); err != nil {
		return err
	}
	if err := exec.Command("tmux", "paste-buffer", "-b", buf, "-d", "-r", "-p", "-t", session).Run(); err != nil {
		return err
	}
	return exec.Command("tmux", "send-keys", "-t", session, "Enter").Run()
}

// SetSessionOption sets a tmux user option scoped to session, so it survives
// a midnight-director restart even though the process itself is stateless.
func SetSessionOption(session, option, value string) error {
	return exec.Command("tmux", "set-option", "-t", session, option, value).Run()
}

// UnsetSessionOption removes a previously set session-scoped user option.
func UnsetSessionOption(session, option string) error {
	return exec.Command("tmux", "set-option", "-u", "-t", session, option).Run()
}

// GetSessionOption reads a session-scoped user option, returning "" if unset.
func GetSessionOption(session, option string) (string, error) {
	out, err := exec.Command("tmux", "show-options", "-t", session, "-v", option).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

const screenHistoryLines = 2000

func CapturePaneRaw(session string) (string, error) {
	out, err := exec.Command("tmux", "capture-pane", "-t", session, "-p", "-e", "-S", fmt.Sprintf("-%d", screenHistoryLines)).Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func CapturePanePlain(session string) (string, error) {
	out, err := exec.Command("tmux", "capture-pane", "-t", session, "-p").Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func CurrentCommand(session string) (string, error) {
	out, err := exec.Command("tmux", "display-message", "-t", session, "-p", "#{pane_current_command}").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func PaneTitle(session string) (string, error) {
	out, err := exec.Command("tmux", "display-message", "-t", session, "-p", "#{pane_title}").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// SessionID returns a session's stable tmux session_id (e.g. "$3"), unaffected by renames.
func SessionID(session string) (string, error) {
	out, err := exec.Command("tmux", "display-message", "-t", session, "-p", "#{session_id}").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func PanePID(session string) (int, error) {
	out, err := exec.Command("tmux", "display-message", "-t", session, "-p", "#{pane_pid}").Output()
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(out)))
}

func AttachSession(session string) error {
	cmd := exec.Command("tmux", "attach-session", "-t", session)
	cmd.Stdin = nil
	return fmt.Errorf("use ExecAttach for interactive attach: %w", cmd.Run())
}

func AttachArgs(session string) []string {
	return []string{"tmux", "attach-session", "-t", session}
}

// RegisterPickerBinding writes a stable wrapper script and registers a tmux
// key binding pointing to it. The script is rewritten on every launch so it
// always exec's the current binary, while the tmux binding path never changes.
// CurrentSession returns the name of the tmux session midnight-director is
// running in, or "" if not inside tmux.
func CurrentSession() string {
	if os.Getenv("TMUX") == "" {
		return ""
	}
	out, err := exec.Command("tmux", "display-message", "-p", "#{session_name}").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// RegisterBackBinding registers M-b as a global key that switches back to
// the midnight-director session from any other session.
func RegisterBackBinding(ourSession string) error {
	if ourSession == "" {
		return nil
	}
	return exec.Command("tmux", "bind-key", "-n", "M-b",
		"switch-client", "-t", ourSession).Run()
}

func RegisterPickerBinding(execPath string) error {
	configDir := filepath.Join(os.Getenv("HOME"), ".config", "midnight-director")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	scriptPath := filepath.Join(configDir, "picker.sh")
	script := fmt.Sprintf("#!/bin/sh\nexec '%s' --picker \"$1\"\n", execPath)
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		return err
	}

	cmd := fmt.Sprintf("'%s' #{session_name}", scriptPath)
	return exec.Command("tmux", "bind-key", "-n", "M-r",
		"display-popup", "-E", "-w", "80%", "-h", "80%", cmd,
	).Run()
}
