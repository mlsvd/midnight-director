package tmux

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
