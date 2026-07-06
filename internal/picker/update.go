package picker

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/malisev/midnight-director/internal/tmux"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.editor.SetWidth(m.width - 6)
		m.editor.SetHeight(m.height - 10)
		// 4 = outer border (2) + title + blank line; 2 = hint + margin
		m.list.SetSize(m.width-4, m.height-6)
		return m, nil
	case tea.KeyMsg:
		switch m.state {
		case stateList:
			return m.handleListKey(msg)
		case stateFill:
			return m.handleFillKey(msg)
		case stateEdit:
			return m.handleEditKey(msg)
		}
	}

	return m.forwardToActive(msg)
}

func (m Model) forwardToActive(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.state {
	case stateList:
		m.list, cmd = m.list.Update(msg)
	case stateFill:
		m.fillInput, cmd = m.fillInput.Update(msg)
	case stateEdit:
		m.editor, cmd = m.editor.Update(msg)
	}
	return m, cmd
}

func (m Model) handleListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// let the list handle esc to clear an active filter; only quit when not filtering
	if msg.String() == "esc" || msg.String() == "ctrl+c" {
		if m.list.FilterState() == list.Filtering {
			var cmd tea.Cmd
			m.list, cmd = m.list.Update(msg)
			return m, cmd
		}
		m.done = true
		return m, tea.Quit
	}

	// intercept enter when not filtering to select the current item
	if msg.String() == "enter" && m.list.FilterState() != list.Filtering {
		item, ok := m.list.SelectedItem().(promptItem)
		if !ok {
			return m, nil
		}
		m.selected = item.Prompt
		m.fillValues = map[string]string{}

		// pre-resolve from: placeholders without showing an input field
		allPH := m.selected.Placeholders()
		var manual []string
		for _, ph := range allPH {
			if strings.HasPrefix(ph, "from:") {
				if val, err := resolveFrom(ph, m.session); err == nil {
					m.fillValues[ph] = val
				} else {
					m.fillValues[ph] = "" // failed gracefully
				}
			} else {
				manual = append(manual, ph)
			}
		}
		m.placeholders = manual
		m.fillIdx = 0
		if len(m.placeholders) == 0 {
			return m.enterEdit(m.selected.Fill(m.fillValues))
		}
		m.state = stateFill
		m.fillInput.SetValue("")
		m.fillInput.Placeholder = m.placeholders[0]
		return m, m.fillInput.Focus()
	}

	// forward everything else (navigation, filter activation, typing) to the list
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m Model) handleFillKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.state = stateList
		return m, nil

	case "enter":
		m.fillValues[m.placeholders[m.fillIdx]] = m.fillInput.Value()
		m.fillIdx++
		if m.fillIdx >= len(m.placeholders) {
			return m.enterEdit(m.selected.Fill(m.fillValues))
		}
		m.fillInput.SetValue("")
		m.fillInput.Placeholder = m.placeholders[m.fillIdx]
		return m, m.fillInput.Focus()
	}

	return m.forwardToActive(msg)
}

func (m Model) handleEditKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		if len(m.placeholders) > 0 {
			m.state = stateFill
			m.fillIdx = len(m.placeholders) - 1
			m.fillInput.SetValue(m.fillValues[m.placeholders[m.fillIdx]])
			return m, m.fillInput.Focus()
		}
		m.state = stateList
		return m, nil

	case "ctrl+s":
		text := strings.TrimSpace(m.editor.Value())
		if text != "" && m.sendFn != nil {
			_ = m.sendFn(m.session, text)
		}
		return m, tea.Quit
	}

	return m.forwardToActive(msg)
}

func (m Model) enterEdit(text string) (Model, tea.Cmd) {
	m.state = stateEdit
	m.editor.SetValue(text)
	cmd := m.editor.Focus()
	m.editor.CursorEnd()
	return m, cmd
}

// resolveFrom resolves a "from:target" or "from:target:Nl" placeholder
// by capturing the target session's pane content.
// "parent" keyword resolves to the parent of currentSession via name prefix.
func resolveFrom(ph, currentSession string) (string, error) {
	inner := strings.TrimPrefix(ph, "from:")
	target, lineSpec, _ := strings.Cut(inner, ":")

	if target == "parent" {
		if idx := strings.LastIndex(currentSession, "/"); idx >= 0 {
			target = currentSession[:idx]
		} else {
			return "", nil // no parent
		}
	}

	content, err := tmux.CapturePanePlain(target)
	if err != nil {
		return "", err
	}
	content = strings.TrimRight(content, "\n")

	if lineSpec != "" && strings.HasSuffix(lineSpec, "l") {
		if n, err := strconv.Atoi(strings.TrimSuffix(lineSpec, "l")); err == nil && n > 0 {
			lines := strings.Split(content, "\n")
			if len(lines) > n {
				lines = lines[len(lines)-n:]
			}
			content = strings.Join(lines, "\n")
		}
	}

	return content, nil
}
