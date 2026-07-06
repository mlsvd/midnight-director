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

func (m Model) buildListContent() string {
	if len(m.sessions) == 0 {
		msg := m.theme.Empty.Render("No sessions. Press  n  to create one.")
		return lipgloss.Place(m.viewport.Width, m.viewport.Height, lipgloss.Center, lipgloss.Center, msg)
	}

	var b strings.Builder
	{
		for i, s := range m.sessions {
			b.WriteString(m.renderSessionRow(i, s))
			b.WriteString("\n")
			if s.Note != "" {
				b.WriteString(m.renderNoteLine(s))
				b.WriteString("\n")
			}
			if i == m.focused {
				switch m.mode {
				case modeMenu:
					b.WriteString(m.viewMenu())
					b.WriteString("\n")
				case modeQuickInput:
					b.WriteString(m.renderInlineInput("  "))
					b.WriteString("\n")
				case modeKillConfirm:
					b.WriteString(m.theme.Confirm.Render(fmt.Sprintf(`  Kill "%s"? [y/N]`, s.Name)))
					b.WriteString("\n")
				case modeAnnotateInput, modeRenameInput:
					b.WriteString(m.renderInlineInput("  "))
					b.WriteString("\n")
				}
			}
		}
	}

	switch m.mode {
	case modeNewSession:
		b.WriteString(m.renderInlineInput("  new session: "))
		b.WriteString("\n")
	case modeCommandInput:
		b.WriteString(m.renderInlineInput("  command: "))
		b.WriteString("\n")
	}

	return b.String()
}

func (m Model) viewSessionList() string {
	var b strings.Builder
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, m.viewport.View(), m.renderScrollbar()))
	b.WriteString("\n")
	b.WriteString(m.viewCommandBar())
	return b.String()
}

func (m Model) renderScrollbar() string {
	total := m.viewport.TotalLineCount()
	h := m.viewport.Height
	if h <= 0 {
		return ""
	}

	trackStyle := lipgloss.NewStyle().Background(lipgloss.Color("236"))
	thumbStyle := lipgloss.NewStyle().Background(lipgloss.Color("243"))

	track := trackStyle.Render(" ") + " "
	thumb := thumbStyle.Render(" ") + " "

	// no scrollbar needed when all content fits
	if total <= h {
		return strings.Repeat(track+"\n", h-1) + track
	}

	thumbH := h * h / total
	if thumbH < 1 {
		thumbH = 1
	}
	maxTop := h - thumbH
	thumbTop := int(m.viewport.ScrollPercent() * float64(maxTop))

	var b strings.Builder
	for i := 0; i < h; i++ {
		if i >= thumbTop && i < thumbTop+thumbH {
			b.WriteString(thumb)
		} else {
			b.WriteString(track)
		}
		if i < h-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func (m Model) renderSessionRow(i int, s *session.Session) string {
	focused := i == m.focused
	rowWidth := m.width - 2 // subtract scrollbar column

	depth := strings.Count(s.Name, "/")
	treeIndent := strings.Repeat("  ", depth)

	var indicator string
	switch {
	case focused:
		indicator = m.theme.SessionNameFocused.Render("▌")
	case depth > 0:
		indicator = m.theme.Hint.Render("└")
	default:
		indicator = " "
	}
	bar := treeIndent + indicator + " "

	// claude icon
	icon := "  "
	if s.IsClaude {
		icon = m.theme.ClaudeIcon.Render("◆ ")
	}

	// name
	var nameStyled string
	if focused {
		nameStyled = m.theme.SessionNameFocused.Render(s.Name)
	} else {
		nameStyled = m.theme.SessionName.Render(s.Name)
	}

	// status badge
	var badge string
	switch s.State {
	case session.StateIdle:
		badge = m.theme.Idle.Render("idle")
	case session.StateRunning:
		badge = m.theme.Running.Render(m.spinner.View() + " running")
	case session.StateDone:
		badge = m.theme.Done.Render("✓ done")
	case session.StateWaiting:
		badge = m.theme.Waiting.Render("waiting")
	}

	fixed := lipgloss.Width(bar) + lipgloss.Width(icon) + lipgloss.Width(nameStyled) +
		2 + lipgloss.Width(badge) + 2
	detailAvail := rowWidth - fixed
	if detailAvail < 0 {
		detailAvail = 0
	}
	detail := m.renderDetail(s, detailAvail)

	row := bar + icon + nameStyled + "  " + badge + "  " + detail
	if focused {
		row = m.theme.Focused.Width(rowWidth).Render(row)
	}
	return row
}

func (m Model) renderNoteLine(s *session.Session) string {
	depth := strings.Count(s.Name, "/")
	indent := strings.Repeat("  ", depth) + "          "
	available := m.width - 2 - len([]rune(indent)) - len("note: ")
	note := []rune(s.Note)
	if len(note) > available && available > 1 {
		note = append(note[:available-1], '…')
	}
	return m.theme.Note.Render(indent + "note: " + string(note))
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
	return lipgloss.NewStyle().MarginLeft(2).Render(m.theme.Menu.Render(strings.Join(rows, "\n")))
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
	h := m.help
	h.Width = m.width - 6
	h.ShowAll = true

	promptsPath := "~/.config/midnight-director/prompts.json"
	tips := m.theme.Hint.Render(
		"prompts: " + promptsPath + "  ·  placeholders: {{variable}} {{from:parent}} {{from:name:10l}}  ·  c=child session",
	)

	return m.theme.CommandBar.Width(m.width - 2).Render(h.View(m.helpKeys()) + "\n" + tips)
}
