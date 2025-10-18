# Auditoría de Agente Autónomo (Sikma Code)

Objetivo: Verificar que el agente autónomo cumple con los requisitos del documento "Configuración Agente LLM Autónomo.md", evitando conflictos con la lógica existente (especialmente confirmaciones) y asegurando una integración correcta antes de pruebas.

## Resumen de Cumplimiento
- Prompt Maestro: Incluye rol, objetivo y trasfondo.
- Herramientas disponibles: file:read, file:write, file:patch, bash:execute, search:web, ask_user_confirmation, memory:write, todo:write, todo:update.
- Políticas de autonomía y seguridad: denylist, comandos críticos (doble confirmación), timeout por defecto, confirmaciones gestionadas por la TUI.
- Protocolo operativo: planificación, ejecución (JSON para herramientas), auto-corrección, memoria/tareas.
- Formato de respuesta: texto estructurado para respuestas normales; JSON (array de objetos) cuando se llamen herramientas, sin mezclar texto.

## Detalles de Implementación
- Archivo: `internal/agent/agent.go`
  - Función `BuildSystemPrompt()` actualizada para:
    - Incluir la configuración dinámica de seguridad (`denylist`, `critical`, `timeout`).
    - Integrar políticas de autonomía (plan previo, confirmaciones, doble confirmación). 
    - Explicar el protocolo (planificación -> ejecución -> auto-corrección -> memoria/tareas).
    - Definir con precisión el formato de respuesta.
- TUI (confirmaciones y ejecución):
  - Archivo: `internal/tui/model.go`
    - Confirmaciones: `isConfirming`, `confirmationType` (S/N/A), `awaitingDoubleConfirm`.
    - Doble confirmación: activada para `bash:execute` si el comando coincide con `tools.IsCriticalCommand`.
    - Herramienta `ask_user_confirmation`: interceptada por la TUI y devuelta al modelo con "CONFIRMATION: yes/no".
    - Sanitización: se elimina JSON de tool-calls del texto, evitando colisiones visuales.
- Tools:
  - Archivo: `internal/tools/tools.go`
    - Implementa: `file:read`, `file:write`, `file:patch`, `search:web`, `memory:write`, `todo:write`, `todo:update`, `bash:execute`, `ask_user_confirmation`.
    - Seguridad: `isDangerousCommand` (denylist); `IsCriticalCommand` (comandos críticos con doble confirmación); `bash:execute` bajo timeout configurable.
- Configuración del repositorio:
  - Archivo: `sikma_code.json` (del repo) ampliado con el set completo de herramientas.

## Verificaciones Clave
- Confirmaciones: El agente está instruido a usar `ask_user_confirmation`. La TUI gestiona las confirmaciones (incluye doble confirmación), evitando prompts propios del agente en texto.
- Seguridad: Denylist y críticos se inyectan en el prompt a partir de la configuración real del sistema (`~/.config/sikmacode/sikma_code.json`).
- Formato: Cuando el agente llame herramientas, debe responder solo con JSON (array). No mezclar texto.
- Persistencia: `AGENT_MEMORY.log` y `AGENT_TODOS.md` están en `.gitignore` y herramientas de memoria/tareas operan sobre estos archivos.

## Limitaciones y Consideraciones
- Ruta de configuración activa: la ejecución usa `~/.config/sikmacode/sikma_code.json`. El archivo del repositorio se usa solo con `--init` si no existe, o `--config` si se instala uno nuevo.
- Modelo/Proveedor: deben estar configurados correctamente (ver `cmd/sikmacode --doctor` y `--doctor-network`).

## Pasos de Validación Recomendados (previos a pruebas interactivas)
1. Compilación: `go build ./cmd/sikmacode`.
2. Doctor de entorno: `go run ./cmd/sikmacode --doctor`.
3. Doctor de red: `go run ./cmd/sikmacode --doctor-network`.
4. Opcional: ajustar seguridad con banderas `--timeout`, `--deny-add`, `--critical-add`.

## Plan de Pruebas Posteriores
- Prueba TUI: `go run ./cmd/sikmacode`.
  - Enviar una tarea que requiera `file:write` (debería pedir S/N/A si no está permitido siempre).
  - Enviar un `bash:execute` crítico (ej. "git push origin main") para verificar doble confirmación.
  - Validar que el agente describe un plan breve, y cuando ejecuta herramientas responde solo con JSON.

## Estado: Listo para prueba
La configuración y el prompt del agente están alineados con el documento de requisitos. Las confirmaciones y la seguridad están coordinadas con la TUI. Se recomienda ejecutar los pasos de validación antes de pruebas funcionales completas.