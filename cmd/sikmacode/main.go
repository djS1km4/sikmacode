package main

import (
	"flag" // Importar el paquete flag
	"log"

	"github.com/djS1km4/sikmacode/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	// Definir el flag --session
	sessionName := flag.String("session", "", "Nombre de la sesión a cargar o crear.")
	flag.Parse()

	// Pasar el nombre de la sesión al modelo
	p := tea.NewProgram(tui.NewAppModel(*sessionName), tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}
