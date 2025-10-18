package tui

import "github.com/charmbracelet/lipgloss"

// Styles contiene las definiciones de estilo para la TUI.
type Styles struct {
	AppStyle      lipgloss.Style
	InputField    lipgloss.Style
	ErrorStyle    lipgloss.Style
	UserRole      lipgloss.Style
	AgentRole     lipgloss.Style
	FooterStyle   lipgloss.Style
	DividerStyle  lipgloss.Style
	LoadingStyle  lipgloss.Style
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
			Foreground(lipgloss.Color("9")). // Rojo
			Background(lipgloss.Color("235")). // Gris oscuro
			Padding(0, 1),
		UserRole: lipgloss.NewStyle().
			Foreground(lipgloss.Color("81")). // Azul claro
			Bold(true),
		AgentRole: lipgloss.NewStyle().
			Foreground(lipgloss.Color("178")). // Amarillo
			Bold(true),
		FooterStyle: lipgloss.NewStyle().
			Background(lipgloss.Color("236")).
			Foreground(lipgloss.Color("248")).
			Padding(0, 1),
		DividerStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")),
		LoadingStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("248")).
			Italic(true),
	}
}
