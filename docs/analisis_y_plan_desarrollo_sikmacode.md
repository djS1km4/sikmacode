# Análisis y Plan de Desarrollo — SikmaCode

## Contexto y objetivo
SikmaCode es un agente con TUI para asistir en generación de proyectos CLI y tareas de desarrollo, con enfoque en trazabilidad, sesiones y configuración controlada.

## Alcance
- Interfaz TUI: navegación, temas, accesibilidad.
- Gestión de sesiones: inicio/cierre, persistencia.
- Configuración del agente: modelos, parámetros, claves, logging.
- Herramientas/utilidades: operaciones auxiliares reproducibles.

## Arquitectura (módulos)
- `internal/agent`: orquestación del flujo y decisiones.
- `internal/llm`: cliente y adaptadores a proveedores.
- `internal/tui`: modelos de vista, estilos, teclas.
- `internal/session`: estado y persistencia.
- `internal/config`: lectura y validación de configuración.
- `internal/tools`: utilidades y plugins.
- `internal/log`: registro y niveles.
- `cmd/sikmacode`: punto de entrada CLI/TUI.

## Backlog principal
- TUI: mejoras de layout y accesibilidad.
- Sesiones: export/import; reanudación segura.
- Modelos: selector y validación; límites de tokens/temperatura.
- Auditoría: aumentar granularidad en `AGENT_AUDIT.md` y `outputs/`.
- Plantillas: generación de CLI reproducible.

## Plan de pruebas
- Preflight: lectores de config y hooks de logs.
- TUI: navegación, cambios de tema, atajos.
- Sesiones: crear/cerrar, reanudar, export/import.
- Flujo CLI: plantilla mínima end-to-end.

## Riesgos y mitigación
- Archivos no versionados: centralizar en `docs/` y PRs.
- Configuración sensible: `.gitignore` para claves; uso de variables de entorno.
- Dependencia de proveedor LLM: desacoplar en `internal/llm` y permitir fallback.

## Indicadores de éxito
- Build estable, pruebas manuales pasan.
- Documentación y changelog al día.
- Flujo end-to-end reproducible.