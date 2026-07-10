package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/malisev/midnight-director/internal/picker"
	"github.com/malisev/midnight-director/internal/prompts"
	"github.com/malisev/midnight-director/internal/tmux"
	"github.com/malisev/midnight-director/internal/tui"
)

func main() {
	if len(os.Args) >= 3 && os.Args[1] == "--picker" {
		darkMode := len(os.Args) < 4 || os.Args[3] != "light"
		runPicker(os.Args[2], darkMode)
		return
	}

	execPath, _ := os.Executable()
	_ = tmux.RegisterPickerBinding(execPath)
	_ = tmux.RegisterBackBinding(tmux.CurrentSession())

	p := tea.NewProgram(
		tui.New(),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func runPicker(session string, darkMode bool) {
	ps, err := prompts.Load(prompts.DefaultPath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading prompts: %v\n", err)
		os.Exit(1)
	}

	m := picker.New(ps, session, darkMode, tmux.SendKeys)
	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
