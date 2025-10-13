package tui

import "github.com/charmbracelet/lipgloss"

// Styles contiene las definiciones de estilo para la TUI.
type Styles struct {
	AppStyle    lipgloss.Style
	InputField  lipgloss.Style
	ErrorStyle  lipgloss.Style
}

// DefaultStyles devuelve un conjunto de estilos predeterminados.
func DefaultStyles() Styles {
	return Styles{
		AppStyle: lipgloss.NewStyle().
			Padding(1, 2).
			Border(lipgloss.RoundedBorder(), true).
			BorderForeground(lipgloss.Color("62")), // Un color verde/amarillo
		InputField: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), true, false, false, false).
			BorderForeground(lipgloss.Color("240")), // Gris oscuro
		ErrorStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("9")).    // Rojo
			Background(lipgloss.Color("235")). // Gris oscuro
			Padding(0, 1),
	}
}
