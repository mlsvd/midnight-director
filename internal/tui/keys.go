package tui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Up       key.Binding
	Down     key.Binding
	PageDown key.Binding
	PageUp   key.Binding
	Input    key.Binding
	Screen   key.Binding
	Picker   key.Binding
	Child    key.Binding
	Menu     key.Binding
	New      key.Binding
	Rename   key.Binding
	Annotate key.Binding
	Generate key.Binding
	Theme    key.Binding
	AutoSum  key.Binding
	Suspend  key.Binding
	Quit     key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.PageDown, k.Input, k.Screen, k.Picker, k.Menu, k.New, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.PageDown, k.PageUp},
		{k.Input, k.Screen, k.Picker, k.Child, k.Menu, k.New},
		{k.Rename, k.Annotate, k.Generate, k.AutoSum},
		{k.Theme, k.Suspend, k.Quit},
	}
}

func (m Model) helpKeys() keyMap {
	themeLabel := "→dark"
	if m.darkMode {
		themeLabel = "→light"
	}
	var sumLabel string
	switch {
	case m.aiCmd == "":
		sumLabel = "auto:n/a"
	case m.autoSummarize:
		sumLabel = "auto:on"
	default:
		sumLabel = "auto:off"
	}
	genLabel := "summarize"
	if m.aiCmd == "" {
		genLabel = "sum:n/a"
	}
	return keyMap{
		Up:       key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:     key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		PageDown: key.NewBinding(key.WithKeys("pgdown", "ctrl+d"), key.WithHelp("pgdn", "page down")),
		PageUp:   key.NewBinding(key.WithKeys("pgup", "ctrl+u"), key.WithHelp("pgup", "page up")),
		Input:    key.NewBinding(key.WithKeys("i", "enter"), key.WithHelp("i", "input")),
		Screen:   key.NewBinding(key.WithKeys(" "), key.WithHelp("spc", "screen")),
		Picker:   key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "prompts")),
		Child:    key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "child")),
		Menu:     key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→", "menu")),
		New:      key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new")),
		Rename:   key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "rename")),
		Annotate: key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "note")),
		Generate: key.NewBinding(key.WithKeys("g"), key.WithHelp("g", genLabel)),
		AutoSum:  key.NewBinding(key.WithKeys("s"), key.WithHelp("s", sumLabel)),
		Theme:    key.NewBinding(key.WithKeys("t"), key.WithHelp("t", themeLabel)),
		Suspend:  key.NewBinding(key.WithKeys("ctrl+z"), key.WithHelp("C-z", "suspend")),
		Quit:     key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	}
}
