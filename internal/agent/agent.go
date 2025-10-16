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
	sb.WriteString("### REGLAS DE EJECUCIÓN ###\n")
	sb.WriteString("1. Para cualquier operación destructiva o que modifique el sistema de archivos (ej. borrar o sobreescribir un archivo), DEBES usar la herramienta `ask_user_confirmation` PRIMERO para pedir permiso.\n")
	sb.WriteString("2. Una vez que el usuario responde 'CONFIRMADO', tu siguiente acción debe ser ejecutar la operación destructiva de inmediato, sin volver a preguntar.\n")
	sb.WriteString("3. Describe la acción peligrosa en el `prompt` de la confirmación.\n")
	sb.WriteString("4. Estás operando en un sistema de archivos de Windows. Usa los comandos de shell de Windows (ej. `dir` en lugar de `ls`, `del` en lugar de `rm`).\n\n")
	sb.WriteString("### FORMATO DE RESPUESTA ###\n")
	sb.WriteString("Cuando necesites usar una herramienta, responde ÚNICAMENTE con un array de objetos JSON que siga este formato exacto:\n")
	sb.WriteString("```json\n")
	sb.WriteString(`[
  {
    "tool": "<nombre_de_la_herramienta>",
    "kwargs": {
      "<argumento_1>": "<valor_1>"
    }
  }
]`)
	sb.WriteString("\n```\n\n")
	sb.WriteString("Responde en formato JSON cuando necesites usar una herramienta.\n")
	return sb.String()
}
