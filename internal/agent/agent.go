package agent

import (
	"fmt"
	"strings"

	"github.com/djS1km4/sikmacode/internal/config"
)

// Agent representa al agente autónomo y su configuración.
type Agent struct {
	Role      string
	Goal      string
	Backstory string
	Tools     []string
}

// NewAgent crea una nueva instancia del agente a partir de la configuración.
func NewAgent(cfg config.AgentConfig) *Agent {
	return &Agent{
		Role:      cfg.Role,
		Goal:      cfg.Goal,
		Backstory: cfg.Backstory,
		Tools:     cfg.Tools,
	}
}

// BuildSystemPrompt construye el "System Prompt" completo para el LLM.
func (a *Agent) BuildSystemPrompt() string {
	var sb strings.Builder
	sb.WriteString("### ROL ###\n")
	sb.WriteString(fmt.Sprintf("Eres un %s.\n\n", a.Role))
	sb.WriteString("### OBJETIVO ###\n")
	sb.WriteString(fmt.Sprintf("%s\n\n", a.Goal))
	sb.WriteString("### TRASFONDO ###\n")
	sb.WriteString(fmt.Sprintf("%s\n\n", a.Backstory))
	sb.WriteString("### HERRAMIENTAS DISPONIBLES ###\n")
	sb.WriteString("Puedes usar las siguientes herramientas:\n")
	for _, tool := range a.Tools {
		sb.WriteString(fmt.Sprintf("- `%s`\n", tool))
	}
	sb.WriteString("\n")
	sb.WriteString("Responde en formato JSON cuando necesites usar una herramienta.\n")
	return sb.String()
}
