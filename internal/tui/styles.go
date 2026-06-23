package tui

import "github.com/charmbracelet/lipgloss"

type Theme struct {
	Title              lipgloss.Style
	Focused            lipgloss.Style
	SessionName        lipgloss.Style
	SessionNameFocused lipgloss.Style
	Idle               lipgloss.Style
	Running            lipgloss.Style
	Done               lipgloss.Style
	Waiting            lipgloss.Style
	Hint               lipgloss.Style
	StreamChunk        lipgloss.Style
	Summary            lipgloss.Style
	ClaudeIcon         lipgloss.Style
	Menu               lipgloss.Style
	MenuSelected       lipgloss.Style
	MenuNormal         lipgloss.Style
	CommandBar         lipgloss.Style
	Shortcut           lipgloss.Style
	InputPrompt        lipgloss.Style
	Empty              lipgloss.Style
	Confirm            lipgloss.Style
}

func darkTheme() Theme {
	purple := lipgloss.Color("99")
	gray := lipgloss.Color("240")
	green := lipgloss.Color("76")
	yellow := lipgloss.Color("220")
	red := lipgloss.Color("196")
	cyan := lipgloss.Color("39")
	white := lipgloss.Color("255")
	dim := lipgloss.Color("244")

	return Theme{
		Title:              lipgloss.NewStyle().Bold(true).Foreground(purple).Padding(0, 1),
		Focused:            lipgloss.NewStyle().Background(lipgloss.Color("237")).Bold(true),
		SessionName:        lipgloss.NewStyle().Bold(true).Foreground(white),
		SessionNameFocused: lipgloss.NewStyle().Bold(true).Foreground(purple),
		Idle:               lipgloss.NewStyle().Foreground(gray),
		Running:            lipgloss.NewStyle().Foreground(green),
		Done:               lipgloss.NewStyle().Foreground(lipgloss.Color("243")),
		Waiting:            lipgloss.NewStyle().Foreground(yellow),
		Hint:               lipgloss.NewStyle().Foreground(dim).Italic(true),
		StreamChunk:        lipgloss.NewStyle().Foreground(cyan),
		Summary:            lipgloss.NewStyle().Foreground(lipgloss.Color("222")).Bold(true),
		ClaudeIcon:         lipgloss.NewStyle().Foreground(purple),
		Menu:               lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(purple).Padding(0, 1),
		MenuSelected:       lipgloss.NewStyle().Background(purple).Foreground(white).Bold(true).Padding(0, 1),
		MenuNormal:         lipgloss.NewStyle().Foreground(white).Padding(0, 1),
		CommandBar:         lipgloss.NewStyle().BorderTop(true).BorderStyle(lipgloss.NormalBorder()).BorderForeground(gray).Foreground(white).Padding(0, 1),
		Shortcut:           lipgloss.NewStyle().Foreground(gray),
		InputPrompt:        lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(cyan).Padding(0, 1),
		Empty:              lipgloss.NewStyle().Foreground(gray).Italic(true),
		Confirm:            lipgloss.NewStyle().Foreground(red).Bold(true),
	}
}

func lightTheme() Theme {
	purple := lipgloss.Color("55")
	gray := lipgloss.Color("243")
	green := lipgloss.Color("28")
	amber := lipgloss.Color("130")
	red := lipgloss.Color("124")
	teal := lipgloss.Color("30")
	black := lipgloss.Color("235")
	dim := lipgloss.Color("245")

	return Theme{
		Title:              lipgloss.NewStyle().Bold(true).Foreground(purple).Padding(0, 1),
		Focused:            lipgloss.NewStyle().Background(lipgloss.Color("253")).Bold(true),
		SessionName:        lipgloss.NewStyle().Bold(true).Foreground(black),
		SessionNameFocused: lipgloss.NewStyle().Bold(true).Foreground(purple),
		Idle:               lipgloss.NewStyle().Foreground(gray),
		Running:            lipgloss.NewStyle().Foreground(green),
		Done:               lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
		Waiting:            lipgloss.NewStyle().Foreground(amber),
		Hint:               lipgloss.NewStyle().Foreground(dim).Italic(true),
		StreamChunk:        lipgloss.NewStyle().Foreground(teal),
		Summary:            lipgloss.NewStyle().Foreground(lipgloss.Color("94")).Bold(true),
		ClaudeIcon:         lipgloss.NewStyle().Foreground(purple),
		Menu:               lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(purple).Padding(0, 1),
		MenuSelected:       lipgloss.NewStyle().Background(purple).Foreground(lipgloss.Color("255")).Bold(true).Padding(0, 1),
		MenuNormal:         lipgloss.NewStyle().Foreground(black).Padding(0, 1),
		CommandBar:         lipgloss.NewStyle().BorderTop(true).BorderStyle(lipgloss.NormalBorder()).BorderForeground(gray).Foreground(black).Padding(0, 1),
		Shortcut:           lipgloss.NewStyle().Foreground(gray),
		InputPrompt:        lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(teal).Padding(0, 1),
		Empty:              lipgloss.NewStyle().Foreground(gray).Italic(true),
		Confirm:            lipgloss.NewStyle().Foreground(red).Bold(true),
	}
}

var spinnerFrames = []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"}
