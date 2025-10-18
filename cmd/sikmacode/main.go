package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/djS1km4/sikmacode/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

type appConfig struct {
	Model           string   `json:"model"`
	Temperature     *float32 `json:"temperature"`
	MaxOutputTokens *int32   `json:"max_output_tokens"`
}

func main() {
	// Flags simples para override de modelo y ruta de configuración
	modelFlag := flag.String("model", "", "Nombre de modelo LLM a usar (override)")
	configPath := flag.String("config", "sikma_code.json", "Ruta de archivo de configuración")
	temperatureFlag := flag.Float64("temperature", -1, "Temperatura de muestreo (0.0-1.0)")
	maxTokensFlag := flag.Int("max-tokens", -1, "Máximo de tokens de salida")
	flag.Parse()

	// Overrides por flags
	if *modelFlag != "" {
		_ = os.Setenv("MODEL", *modelFlag)
	}
	if *temperatureFlag >= 0 {
		_ = os.Setenv("TEMPERATURE", fmt.Sprintf("%g", *temperatureFlag))
	}
	if *maxTokensFlag >= 0 {
		_ = os.Setenv("MAX_OUTPUT_TOKENS", fmt.Sprintf("%d", *maxTokensFlag))
	}

	// Si faltan valores en entorno, intenta cargar desde JSON
	if data, err := os.ReadFile(*configPath); err == nil {
		var cfg appConfig
		if jsonErr := json.Unmarshal(data, &cfg); jsonErr == nil {
			if os.Getenv("MODEL") == "" && cfg.Model != "" {
				_ = os.Setenv("MODEL", cfg.Model)
			}
			if os.Getenv("TEMPERATURE") == "" && cfg.Temperature != nil {
				_ = os.Setenv("TEMPERATURE", fmt.Sprintf("%g", *cfg.Temperature))
			}
			if os.Getenv("MAX_OUTPUT_TOKENS") == "" && cfg.MaxOutputTokens != nil {
				_ = os.Setenv("MAX_OUTPUT_TOKENS", fmt.Sprintf("%d", *cfg.MaxOutputTokens))
			}
		}
	}

	p := tea.NewProgram(tui.NewModel())

	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}
