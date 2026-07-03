package tmux

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
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

func NewSession(name string) error {
	return exec.Command("tmux", "new-session", "-d", "-s", name).Run()
}

func KillSession(name string) error {
	return exec.Command("tmux", "kill-session", "-t", name).Run()
}

func RenameSession(old, newName string) error {
	return exec.Command("tmux", "rename-session", "-t", old, newName).Run()
}

func SendKeys(session, keys string) error {
	return exec.Command("tmux", "send-keys", "-t", session, keys, "Enter").Run()
}

func CapturePaneRaw(session string) (string, error) {
	out, err := exec.Command("tmux", "capture-pane", "-t", session, "-p", "-e").Output()
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

// RegisterPickerBinding installs a global tmux key binding (M-p) that opens
// the prompt picker popup targeting whichever session is currently active.
func RegisterPickerBinding(execPath string) error {
	// Single-quote execPath so spaces in the path don't break shell parsing.
	// #{session_name} is expanded by tmux at keypress time before shell sees it.
	cmd := fmt.Sprintf("'%s' --picker #{session_name}", execPath)
	return exec.Command("tmux", "bind-key", "-n", "M-r",
		"display-popup", "-E", "-w", "80%", "-h", "80%", cmd,
	).Run()
}
