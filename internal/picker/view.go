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
	case stateWarn:
		title = m.selected.Name
		body = m.viewWarn()
	}

	inner := lipgloss.NewStyle().
		Width(m.width - 4).
		Render(m.styles.title.Render(title) + "\n\n" + body)

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.styles.border).
		Width(m.width - 2).
		Render(inner)
}

func (m Model) viewList() string {
	hint := m.styles.hint.Render("/ filter   ↑↓ navigate   ↵ select   esc quit")

	if len(m.list.Items()) == 0 {
		path := "~/.config/midnight-director/prompts.json"
		body := m.styles.label.Render("No prompts yet. Create "+path+":") +
			"\n\n" +
			m.styles.preview.Render(
				`[`+"\n"+
					`  {`+"\n"+
					`    "name": "short label shown in picker",`+"\n"+
					`    "text": "full prompt text, can span multiple lines"`+"\n"+
					`  }`+"\n"+
					`]`,
			) +
			"\n\n" +
			m.styles.label.Render("Placeholders inside \"text\" (filled interactively when prompt is selected):") +
			"\n" +
			m.styles.hint.Render("  {{placeholder}}    prompted to enter a value before sending")
		return body + "\n\n" + hint
	}

	return m.list.View() + "\n" + hint
}

func (m Model) viewFill() string {
	var b strings.Builder

	preview := m.selected.Fill(m.fillValues)
	b.WriteString(m.styles.preview.Render(truncatePreview(preview, m.width-8, 4)))
	b.WriteString("\n\n")

	total := len(m.placeholders)
	cur := m.placeholders[m.fillIdx]
	b.WriteString(m.styles.label.Render(fmt.Sprintf("placeholder %d/%d: ", m.fillIdx+1, total)))
	b.WriteString(m.styles.highlight.Render("{{" + cur + "}}"))
	b.WriteString("\n")
	b.WriteString(m.fillInput.View())
	b.WriteString("\n\n")
	b.WriteString(m.styles.hint.Render("↵ next   esc back"))

	return b.String()
}

func (m Model) viewEdit() string {
	var b strings.Builder

	b.WriteString(m.styles.label.Render("edit before sending:"))
	b.WriteString("\n")
	b.WriteString(m.editor.View())
	b.WriteString("\n\n")
	b.WriteString(m.styles.hint.Render("ctrl+s send   esc back"))

	return b.String()
}

func (m Model) viewWarn() string {
	var b strings.Builder
	b.WriteString(m.styles.highlight.Render("⚠  Session is not waiting for input."))
	b.WriteString("\n")
	b.WriteString(m.styles.hint.Render("   Sending now may paste text into a shell or running process unexpectedly."))
	b.WriteString("\n\n")
	b.WriteString(m.styles.label.Render("   [s] send anyway   [c] copy to clipboard   [esc] back to edit"))
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
