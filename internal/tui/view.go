package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/malisev/midnight-director/internal/session"
)

const minWidth = 30
const minHeight = 5

func (m Model) View() string {
	if m.width == 0 {
		return ""
	}
	if m.width < minWidth || m.height < minHeight {
		return m.theme.Empty.Render("terminal too small")
	}

	var b strings.Builder
	b.WriteString(m.theme.Title.Render("midnight director"))
	b.WriteString("\n")

	switch m.mode {
	case modeScreenView, modeScreenInput:
		b.WriteString(m.viewScreen())
	default:
		b.WriteString(m.viewSessionList())
	}

	return b.String()
}

func (m Model) viewSessionList() string {
	var b strings.Builder

	if len(m.sessions) == 0 {
		padding := (m.height - 4) / 2
		for i := 0; i < padding; i++ {
			b.WriteString("\n")
		}
		b.WriteString(m.theme.Empty.Render("  No sessions. Press  n  to create one."))
		b.WriteString("\n")
	} else {
		b.WriteString("\n")
		for i, s := range m.sessions {
			b.WriteString(m.renderSessionRow(i, s))
			b.WriteString("\n")
			if i == m.focused && m.mode == modeQuickInput {
				b.WriteString(m.renderInlineInput("  "))
				b.WriteString("\n")
			}
			if i == m.focused && m.mode == modeKillConfirm {
				b.WriteString(m.theme.Confirm.Render(fmt.Sprintf(`  Kill "%s"? [y/N]`, s.Name)))
				b.WriteString("\n")
			}
			if i == m.focused && (m.mode == modeAnnotateInput || m.mode == modeRenameInput) {
				b.WriteString(m.renderInlineInput("  "))
				b.WriteString("\n")
			}
		}
		b.WriteString("\n")
	}

	if m.mode == modeMenu && len(m.sessions) > 0 {
		b.WriteString(m.viewMenu())
		b.WriteString("\n")
	}
	if m.mode == modeNewSession {
		b.WriteString(m.renderInlineInput("  new session: "))
		b.WriteString("\n")
	}
	if m.mode == modeCommandInput {
		b.WriteString(m.renderInlineInput("  command: "))
		b.WriteString("\n")
	}

	b.WriteString(m.viewCommandBar())
	return b.String()
}

func (m Model) renderSessionRow(i int, s *session.Session) string {
	focused := i == m.focused

	icon := "  "
	if s.IsClaude {
		icon = m.theme.ClaudeIcon.Render("◆ ")
	}

	var name string
	if focused {
		name = m.theme.SessionNameFocused.Render("[" + s.Name + "]")
	} else {
		name = m.theme.SessionName.Render("[" + s.Name + "]")
	}

	var stateBadge string
	switch s.State {
	case session.StateIdle:
		stateBadge = m.theme.Idle.Render("idle")
	case session.StateRunning:
		spinner := spinnerFrames[m.spinnerTick%len(spinnerFrames)]
		stateBadge = m.theme.Running.Render(spinner + " running")
	case session.StateDone:
		stateBadge = m.theme.Done.Render("✓ done")
	case session.StateWaiting:
		stateBadge = m.theme.Waiting.Render("waiting")
	}

	var shortcut string
	if focused && s.State == session.StateWaiting {
		shortcut = m.theme.Shortcut.Render(" [↵ i]")
	}

	var note string
	if s.Note != "" && focused {
		note = m.theme.Hint.Render("  ✎ " + s.Note)
	}

	fixedWidth := lipgloss.Width(icon) + lipgloss.Width(name) + 2 +
		lipgloss.Width(stateBadge) + 2 +
		lipgloss.Width(shortcut) + lipgloss.Width(note)
	available := m.width - 2 - fixedWidth - 2
	if available < 0 {
		available = 0
	}

	detail := m.renderDetail(s, available)

	row := fmt.Sprintf("%s%s  %s  %s%s%s", icon, name, stateBadge, detail, shortcut, note)
	if focused {
		row = m.theme.Focused.Width(m.width - 2).Render(row)
	}
	return row
}

func (m Model) renderDetail(s *session.Session, available int) string {
	if available < 4 {
		return ""
	}

	var text string
	var style lipgloss.Style

	switch {
	case s.IsSummarizing:
		text = "⟳ summarizing…"
		style = m.theme.Hint
	case s.Summary != "":
		text = "✦ " + s.Summary
		style = m.theme.Summary
	case s.Title != "":
		text = s.Title
		style = m.theme.Hint
	case s.State == session.StateRunning && s.StreamChunk != "":
		text = s.StreamChunk
		style = m.theme.StreamChunk
	case s.Hint != "":
		text = s.Hint
		style = m.theme.Hint
	default:
		return ""
	}

	runes := []rune(text)
	if len(runes) > available {
		runes = runes[:available-1]
		text = string(runes) + "…"
	}
	return style.Render(text)
}

func (m Model) renderInlineInput(prefix string) string {
	return m.theme.InputPrompt.Render(prefix + m.input.View())
}

func (m Model) viewMenu() string {
	var rows []string
	for i, item := range menuItems {
		if i == m.menuCursor {
			rows = append(rows, m.theme.MenuSelected.Render(item.label))
		} else {
			rows = append(rows, m.theme.MenuNormal.Render(item.label))
		}
	}
	return "  " + m.theme.Menu.Render(strings.Join(rows, "\n"))
}

func (m Model) viewScreen() string {
	var b strings.Builder

	contentHeight := m.height - 4
	if contentHeight < 1 {
		contentHeight = 1
	}
	innerWidth := m.width - 4
	if innerWidth < 1 {
		innerWidth = 1
	}

	lines := strings.Split(m.screenText, "\n")
	for i, line := range lines {
		runes := []rune(line)
		if len(runes) > innerWidth {
			lines[i] = string(runes[:innerWidth])
		}
	}
	if len(lines) > contentHeight {
		lines = lines[len(lines)-contentHeight:]
	}

	screenBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.Shortcut.GetForeground()).
		Width(m.width - 2).
		Render(strings.Join(lines, "\n"))

	b.WriteString(screenBox)
	b.WriteString("\n")

	if m.mode == modeScreenInput {
		b.WriteString(m.renderInlineInput("  "))
	} else {
		b.WriteString(m.theme.Shortcut.Render("  [i] send input   [esc] close"))
	}
	b.WriteString("\n")

	return b.String()
}

func (m Model) viewCommandBar() string {
	themeLabel := "light"
	if m.darkMode {
		themeLabel = "dark"
	}

	var aiHint string
	if m.aiCmd == "" {
		aiHint = "s no-ai"
	} else if m.autoSummarize {
		aiHint = "s sum:on"
	} else {
		aiHint = "s sum:off"
	}

	left := "> "
	right := m.theme.Shortcut.Render(
		fmt.Sprintf("n new   i input   e rename   a note   t %s   %s   q quit", themeLabel, aiHint),
	)

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right) - 4
	if gap < 1 {
		return m.theme.CommandBar.Width(m.width - 2).Render(left)
	}

	bar := left + strings.Repeat(" ", gap) + right
	return m.theme.CommandBar.Width(m.width - 2).Render(bar)
}
