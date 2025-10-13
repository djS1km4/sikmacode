package tui

import "github.com/charmbracelet/lipgloss"

// Styles es un struct para mantener los estilos de la TUI.
type Styles struct {
	BorderColor lipgloss.Color
	InputField  lipgloss.Style
}

// DefaultStyles retorna una configuración de estilos por defecto.
func DefaultStyles() Styles {
	return Styles{
		BorderColor: lipgloss.Color("#874BFD"),
		InputField:  lipgloss.NewStyle().BorderForeground(lipgloss.Color("#874BFD")).BorderStyle(lipgloss.RoundedBorder()).Padding(1),
	}
}
