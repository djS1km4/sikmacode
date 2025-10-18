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

	// Cargar seguridad dinámica desde la configuración del sistema
	cfg, _ := config.LoadConfig()
	deny := []string{"rm -rf", "rd /s /q", "del /q", "format ", "mkfs", "shutdown", "reboot", "mkpartition", "bcdedit", "reg delete"}
	critical := []string{"git push", "reset --hard", "terraform apply", "docker rmi", "npm publish", "choco install", "pip install -U", "setx ", "reg add"}
	timeoutSec := 15
	if cfg != nil {
		if len(cfg.Security.Denylist) > 0 { deny = cfg.Security.Denylist }
		if len(cfg.Security.Critical) > 0 { critical = cfg.Security.Critical }
		if cfg.Security.DefaultTimeoutSec > 0 { timeoutSec = cfg.Security.DefaultTimeoutSec }
	}

	// ROL / OBJETIVO / TRASFONDO
	sb.WriteString("### ROL ###\n")
	sb.WriteString(fmt.Sprintf("Eres un %s.\n\n", a.Role))
	sb.WriteString("### OBJETIVO ###\n")
	sb.WriteString(fmt.Sprintf("%s\n\n", a.Goal))
	sb.WriteString("### TRASFONDO ###\n")
	sb.WriteString(fmt.Sprintf("%s\n\n", a.Backstory))

	// ENTORNO
	sb.WriteString("### ENTORNO OPERATIVO ###\n")
	sb.WriteString("Sistema Operativo: Windows\n")
	sb.WriteString("Shell para comandos: cmd.exe /C\n\n")

	// HERRAMIENTAS
	sb.WriteString("### HERRAMIENTAS DISPONIBLES ###\n")
	sb.WriteString("Puedes usar las siguientes herramientas cuando sea necesario:\n")
	toolSet := make(map[string]struct{})
	for _, t := range a.Tools { toolSet[t] = struct{}{} }
	// Asegurar presencia de confirmación humana en el catálogo
	if _, ok := toolSet["ask_user_confirmation"]; !ok { sb.WriteString("- `ask_user_confirmation`\n") }
	for _, tool := range a.Tools {
		sb.WriteString(fmt.Sprintf("- `%s`\n", tool))
	}
	sb.WriteString("\n")

	// POLÍTICAS DE AUTONOMÍA Y SEGURIDAD
	sb.WriteString("### POLÍTICAS DE AUTONOMÍA Y SEGURIDAD ###\n")
	sb.WriteString("- Antes de ejecutar acciones, elabora un plan breve y claro.\n")
	sb.WriteString("- Usa `ask_user_confirmation` para acciones sensibles o potencialmente disruptivas.\n")
	sb.WriteString("- No intentes eludir confirmaciones: la TUI las gestiona (incluye doble confirmación).\n")
	sb.WriteString(fmt.Sprintf("- Denylist activa: %s\n", strings.Join(deny, ", ")))
	sb.WriteString(fmt.Sprintf("- Comandos críticos (doble confirmación): %s\n", strings.Join(critical, ", ")))
	sb.WriteString(fmt.Sprintf("- Timeout por defecto para `bash:execute`: %ds (configurable).\n\n", timeoutSec))

	// PROTOCOLO DE TRABAJO
	sb.WriteString("### PROTOCOLO DE TRABAJO ###\n")
	sb.WriteString("1) Planificación: genera un 'Plan de Alto Nivel' (texto) con tareas y dependencias.\n")
	sb.WriteString("2) Ejecución: cuando necesites una herramienta, emite ÚNICAMENTE JSON con el llamado.\n")
	sb.WriteString("3) Auto-corrección: si una operación falla, diagnostica, ajusta y vuelve a intentar.\n")
	sb.WriteString("4) Memoria/Tareas: si está disponible, usa `memory:write` y `todo:write`/`todo:update` para persistencia.\n\n")

	// FORMATO DE RESPUESTA
	sb.WriteString("### FORMATO DE RESPUESTA ###\n")
	sb.WriteString("- Respuesta normal (sin herramientas):\n")
	sb.WriteString("  Resumen breve\n  Plan de acción\n  Siguiente paso\n")
	sb.WriteString("- Llamada a herramientas: responde ÚNICAMENTE con un array JSON de objetos:\n")
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
	sb.WriteString("Nunca mezcles texto con el JSON cuando llames herramientas.\n")

	return sb.String()
}
