# Auditoría de Avance y Plan de Cierre – Sikma Code

**Fecha:** 18 de octubre de 2025  
**Rama:** `docs/recovery-plan`  
**Contexto:** Proyecto unificado en raíz, documentación base creada y README actualizado; build verificado.

---

## Resumen Ejecutivo
- Estado actual: MVP conversacional funcional sobre TUI (Bubble Tea) con conexión a Gemini (`GEMINI_API_KEY`), historial en memoria y cierre con `Esc/Ctrl+C`.
- Documentación: ramas `docs/*` activas; creados informes y guía de configuración; README enriquecido con sección de atajos.
- Build: `go build ./...` exitoso. Estructura clara (`cmd/`, `internal/`, `sessions/`).
- Gap principal: no hay configuración externa (`sikma_code.json`), persistencia de sesiones integrada, CLI/flags, renderizado Markdown, ni el núcleo del agente autónomo.

---

## Alcance de la Auditoría
- Documentos revisados: `docs/hoja de ruta SikmaCode.md`, `docs/analisis_y_plan_desarrollo_sikmacode.md`, `docs/Configuración Agente LLM Autónomo.md`, `docs/informe_cli_sikma.md`.
- Código analizado: `cmd/sikmacode/main.go`, `internal/tui/model.go`, `internal/tui/styles.go`, `internal/llm/client.go`, `sessions/default.json`.
- Resultado: mapeo completo del estado actual contra la hoja de ruta (6 fases) y plan propuesto v1.1.

---

## Estado Actual vs Hoja de Ruta

### Fase 1: MVP – Núcleo TUI y LLM
- Implementado:
  - TUI básica con `textarea` y `viewport`, ciclo `Init/Update/View`.
  - Conexión a LLM (Gemini 2.5), lectura de `GEMINI_API_KEY`, envío con `Enter`.
  - Historial de conversación acumulado en memoria (`cs.History`).
- Pendiente:
  - Renderizado de Markdown con `glamour`.
  - Mejora de estilos y tema con Lip Gloss.

### Fase 2: Configuración, Sesiones y Persistencia
- Pendiente:
  - Lectura de `sikma_code.json` (proveedor/modelo/parámetros).  
  - Gestión segura de API keys y validación de configuración.  
  - Persistencia real de sesiones: `new/save/load/list/switch` y almacenamiento en `sessions/`.
  - CLI/flags (Cobra) para interacción fuera de TUI.

### Fase 3: Agente Autónomo (Núcleo)
- Pendiente:
  - `Agent` (Role, Goal, Backstory, Constraints).  
  - Bucle “observar-pensar-actuar” con contexto del System Prompt.  
  - Herramientas mínimas: `file:read`, `file:write`, `ask_user_confirmation`.  
  - Parser de salida del LLM (JSON → comandos).

### Fase 4: Herramientas Avanzadas y Resiliencia
- Pendiente:
  - `bash:execute` con confirmación obligatoria.  
  - `search:web`.  
  - Planificación de tareas y presentación en TUI.  
  - Auto-corrección y diagnóstico de errores.

### Fase 5: Memoria y Extensibilidad
- Pendiente:
  - `AGENT_MEMORY.log` (JSONL) y lectura de últimas N entradas.  
  - `AGENT_TODOS.md` y comandos `todo:*`.  
  - Integración inicial MCP/LSP (mínimo viable).  
  - Auto-optimización del System Prompt.

### Fase 6: Pulido y Distribución
- Pendiente:
  - Multilenguaje (carga de `es.json`/`en.json`).  
  - Tematización avanzada con Lip Gloss.  
  - Distribución multiplataforma (`goreleaser`).  
  - Documentación de instalación y uso avanzado.

---

## Atajos y Comportamiento Actual de la TUI
- `Enter`: envía y limpia el campo de entrada (sin salto de línea).
- `Esc` o `Ctrl+C`: salir de la aplicación.
- `↑`/`↓`: desplazamiento del historial; `PageUp/PageDown` y `Home/End` soportados por `viewport`.
- Nota: `textarea` tiene `InsertNewline` deshabilitado; el salto de línea no está disponible por ahora.

---

## Riesgos y Bloqueos
- Dependencia exclusiva de `GEMINI_API_KEY` sin selección de proveedor/modelo.
- Sin configuración persistente ni CLI → menor reproducibilidad y automatización.
- Ausencia de persistencia de sesiones y memoria del agente → no se conserva progreso entre ejecuciones.
- Renderizado de Markdown ausente → respuesta del LLM menos legible.

---

## Recomendaciones Inmediatas (Plan 1–2 semanas)
1. Renderizado Markdown en `viewport` con `glamour` (impacto directo en UX).  
2. `sikma_code.json`: esquema y lectura (proveedor, modelo, temperatura, tokens, idioma, tema).  
3. Persistencia de sesiones (`sessions/`): guardar al cerrar y cargar al iniciar; comandos TUI mínimos.  
4. CLI con `cobra`: `--session`, `--model`, `--config`, `new/save/load/list`.  
5. Ajustes de estilo base con Lip Gloss (tema claro/oscuro inicial).

---

## Plan de Ramas Propuesto
- Crear `feature/mvp-phase2` para implementar Configuración + Sesiones + Markdown.
- Mantener documentación en `docs/recovery-plan` y abrir PRs para revisión.

---

## Checklist de Cierre (v1.1)
- [ ] Integrar `glamour` para Markdown en `viewport`.  
- [ ] Implementar `sikma_code.json` y lector de configuración.  
- [ ] Persistencia de sesiones: `new/save/load/list/switch`.  
- [ ] CLI con `cobra` y flags esenciales.  
- [ ] `Agent` mínimo + `file:read/write` + confirmación humana.  
- [ ] Corregir referencias de auditoría (`AGENT_AUDIT.md`/`outputs/*`) o implementar registro.  
- [ ] Documentar instalación y flujo avanzado; actualizar README según avance.  
- [ ] Preparar PR de cierre para v1.1.

---

## Evidencias y Referencias
- Build verificado: `go build ./...` sin errores.  
- README actualizado con atajos.  
- Documentación base creada en `docs/`.
- Referencias clave: Charmbracelet (`bubbletea`, `lipgloss`, `bubbles`, `glamour`), Qwen-Code, prácticas de “configuración como código”.

---

## Próximo Paso Sugerido
Aprobar la creación de la rama `feature/mvp-phase2` y comenzar con: `glamour` (Markdown), `sikma_code.json` (config), persistencia de sesiones y CLI básica. Al finalizar, abrir PR para revisión y pruebas.