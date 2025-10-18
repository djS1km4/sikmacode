# Hoja de Ruta — SikmaCode

Objetivo: consolidar un agente TUI para generación de proyectos CLI y flujos asistidos.

## Hitos
- v1.0.x: Estabilizar TUI, sesiones y configuración; documentación completa.
- v1.1: Plugins de herramientas y plantillas ampliadas; mejoras de UX TUI.
- v1.2: Integración extendida con servicios externos; auditoría y trazabilidad avanzada.

## Funcionalidades priorizadas
- Sesiones persistentes, con export/import.
- Selector de modelos y ajustes de temperatura/tokens.
- Temas y accesibilidad TUI.
- Flujo end-to-end para generar una app CLI (plantillas + prompts guiados).
- Registro y auditoría (`outputs/`, `AGENT_AUDIT.md`).

## Plan de ramas
- Desarrollo: `feature/*` por componente.
- Documentación: `docs/*` con PR obligatorio.
- Releases: `master` con tags.

## Criterios de entrega
- Build estable (`go build ./...`).
- Pruebas manuales guiadas (TUI: navegación, temas, sesiones).
- Documentación actualizada en `docs/`.
- Changelog y auditoría actualizados.