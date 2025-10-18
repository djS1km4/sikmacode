package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"

	"github.com/djS1km4/sikmacode/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

type appConfig struct {
	Model string `json:"model"`
}

func main() {
	// Flags simples para override de modelo y ruta de configuración
	modelFlag := flag.String("model", "", "Nombre de modelo LLM a usar (override)")
	configPath := flag.String("config", "sikma_code.json", "Ruta de archivo de configuración")
	flag.Parse()

	// Si se especifica --model, override directo del entorno
	if *modelFlag != "" {
		_ = os.Setenv("MODEL", *modelFlag)
	} else if os.Getenv("MODEL") == "" {
		// Si no hay env MODEL, intenta cargar desde JSON
		if data, err := os.ReadFile(*configPath); err == nil {
			var cfg appConfig
			if jsonErr := json.Unmarshal(data, &cfg); jsonErr == nil {
				if cfg.Model != "" {
					_ = os.Setenv("MODEL", cfg.Model)
				}
			}
		}
	}

	p := tea.NewProgram(tui.NewModel())

	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}
