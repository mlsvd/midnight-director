package picker

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) View() string {
	if m.width == 0 {
		return ""
	}

	var body string
	var title string

	switch m.state {
	case stateList:
		title = "prompts"
		body = m.viewList()
	case stateFill:
		title = m.selected.Name
		body = m.viewFill()
	case stateEdit:
		title = m.selected.Name
		body = m.viewEdit()
	}

	inner := lipgloss.NewStyle().
		Width(m.width - 4).
		Render(styleTitle.Render(title) + "\n\n" + body)

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("99")).
		Width(m.width - 2).
		Render(inner)
}

func (m Model) viewList() string {
	hint := styleHint.Render("/ filter   ↑↓ navigate   ↵ select   esc quit")
	return m.list.View() + "\n" + hint
}

func (m Model) viewFill() string {
	var b strings.Builder

	preview := m.selected.Fill(m.fillValues)
	b.WriteString(stylePreview.Render(truncatePreview(preview, m.width-8, 4)))
	b.WriteString("\n\n")

	total := len(m.placeholders)
	cur := m.placeholders[m.fillIdx]
	b.WriteString(styleLabel.Render(fmt.Sprintf("placeholder %d/%d: ", m.fillIdx+1, total)))
	b.WriteString(styleHighlight.Render("{{" + cur + "}}"))
	b.WriteString("\n")
	b.WriteString(m.fillInput.View())
	b.WriteString("\n\n")
	b.WriteString(styleHint.Render("↵ next   esc back"))

	return b.String()
}

func (m Model) viewEdit() string {
	var b strings.Builder

	b.WriteString(styleLabel.Render("edit before sending:"))
	b.WriteString("\n")
	b.WriteString(m.editor.View())
	b.WriteString("\n\n")
	b.WriteString(styleHint.Render("ctrl+s send   esc back"))

	return b.String()
}

func truncatePreview(text string, width, maxLines int) string {
	lines := strings.Split(text, "\n")
	if len(lines) > maxLines {
		lines = lines[:maxLines]
		lines = append(lines, "…")
	}
	for i, l := range lines {
		runes := []rune(l)
		if len(runes) > width {
			lines[i] = string(runes[:width-1]) + "…"
		}
	}
	return strings.Join(lines, "\n")
}
