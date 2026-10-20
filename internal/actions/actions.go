package actions

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Action is a user-configured bash script that can be run against the focused session, e.g. to
// open a PR in a browser based on a convention encoded in the session name.
type Action struct {
	Name string // derived from the file name, dashes/underscores turned into spaces
	Path string
}

// DefaultDir is where custom action scripts live — one *.sh file per action.
func DefaultDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "midnight-director", "actions")
}

const scriptExt = ".sh"

// Load scans dir for *.sh scripts and returns one Action per script, sorted by file name. A
// missing directory yields no actions rather than an error, since most users won't have any.
func Load(dir string) ([]Action, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var names []string
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != scriptExt {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)

	var out []Action
	for _, name := range names {
		label := strings.NewReplacer("-", " ", "_", " ").Replace(strings.TrimSuffix(name, scriptExt))
		out = append(out, Action{Name: label, Path: filepath.Join(dir, name)})
	}
	return out, nil
}

// Run executes the action's script via bash, passing the session's name, tmux session_id, and
// pane PID both as positional args ($1 $2 $3) and as MD_SESSION_NAME/MD_SESSION_ID/MD_SESSION_PID
// env vars, so scripts can use whichever convention is more convenient.
func (a Action) Run(sessionName, sessionID string, pid int) ([]byte, error) {
	pidStr := strconv.Itoa(pid)
	cmd := exec.Command("bash", a.Path, sessionName, sessionID, pidStr)
	cmd.Env = append(os.Environ(),
		"MD_SESSION_NAME="+sessionName,
		"MD_SESSION_ID="+sessionID,
		"MD_SESSION_PID="+pidStr,
	)
	return cmd.CombinedOutput()
}
