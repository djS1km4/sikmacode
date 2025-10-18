package tui

import (
	"github.com/charmbracelet/bubbles/key"
)

// KeyMap representa los atajos del TUI.
type KeyMap struct {
	Quit    key.Binding
	Save    key.Binding
	Help    key.Binding
	Theme   key.Binding
	Models  key.Binding // Nuevo: abre el menú MODELOS
	Sessions key.Binding // Nuevo: abre el menú SESIONES
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		Quit: key.NewBinding(
			key.WithKeys("ctrl+c"),
			key.WithHelp("ctrl+c", "salir"),
		),
		Save: key.NewBinding(
			key.WithKeys("ctrl+s"),
			key.WithHelp("ctrl+s", "guardar sesión"),
		),
		Help: key.NewBinding(
			key.WithKeys("f1"),
			key.WithHelp("f1", "ayuda"),
		),
		Theme: key.NewBinding(
			key.WithKeys("f2"),
			key.WithHelp("f2", "cambiar tema"),
		),
		Models: key.NewBinding(
			key.WithKeys("f3"),
			key.WithHelp("f3", "abrir menú MODELOS"),
		),
		Sessions: key.NewBinding(
			key.WithKeys("f4"),
			key.WithHelp("f4", "abrir menú SESIONES"),
		),
	}
}