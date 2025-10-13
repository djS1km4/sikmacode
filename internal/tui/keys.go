package tui

import "github.com/charmbracelet/bubbles/key"

// KeyMap define los atajos de teclado para la aplicación.
type KeyMap struct {
	Save key.Binding
	Quit key.Binding
}

// DefaultKeyMap devuelve un conjunto de atajos de teclado predeterminados.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Save: key.NewBinding(
			key.WithKeys("ctrl+s"),
			key.WithHelp("ctrl+s", "guardar sesión"),
		),
		Quit: key.NewBinding(
			key.WithKeys("ctrl+c", "esc"),
			key.WithHelp("ctrl+c/esc", "salir"),
		),
	}
}
