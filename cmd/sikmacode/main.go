package main

import (
	"log"

	"github.com/djS1km4/sikmacode/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	p := tea.NewProgram(tui.NewModel())

	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}
