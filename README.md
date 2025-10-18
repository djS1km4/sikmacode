# ✨ Sikma Code ✨

**Tu compañero de programación IA directamente en tu terminal.**

Sikma Code es un agente de codificación autónomo construido en Go con una TUI elegante (Bubble Tea + Lip Gloss). Ofrece planificación, ejecución asistida y trazabilidad para acelerar tu flujo de trabajo.

---

## 🚀 Características Principales
- 🤖 Agente autónomo: entiende tareas, planifica, usa herramientas y se auto-ajusta.
- 💅 TUI moderna: interfaz fluida y personalizable con atajos de teclado.
- 🔌 Multi-proveedor LLM: configurable y conmutación sencilla entre proveedores.
- 🧰 Herramientas: lectura/escritura de archivos, ejecución de comandos y más.
- 🧠 Sesiones y trazabilidad: auditoría en `AGENT_AUDIT.md` y logs en `outputs/`.

## 📦 Requisitos
- `Go` 1.21+ (recomendado)
- Terminal con soporte UTF-8 (Windows Terminal/PowerShell 7+, macOS, Linux)

## 🛠️ Instalación y Ejecución
- Ejecución directa:
  ```powershell
  go run cmd\sikmacode\main.go
  ```
- Compilación:
  ```powershell
  go build -o sikmacode.exe .\cmd\sikmacode
  .\sikmacode.exe
  ```
- Instalar (próximamente):
  ```bash
  go install github.com/djS1km4/sikmacode@latest
  ```

## ⚙️ Configuración
- Archivo: `sikma_code.json` (parámetros generales: modelo, temperatura, tokens, logs).
- Variables de entorno (según proveedor):
  - `API_KEY` o `GEMINI_API_KEY` (si usas Google AI Studio)
  - `MODEL` (modelo por defecto)
  - `LOG_LEVEL` (`debug|info|warn|error`)

## 🖥️ Uso Rápido (TUI)
- Navegación con flechas, `Enter`, `Esc`, `Tab`.
- Cambia modelo y parámetros desde el menú.
- Gestiona sesiones (archivos en `sessions/`) y reanuda tu trabajo.
- Observa auditoría y registros en `AGENT_AUDIT.md` y `outputs/*.txt`.

## 📚 Documentación y Roadmap
- Documentación: `docs/README.md`
- Hoja de ruta: `docs/roadmap.md`
- Guía de configuración: `docs/configuracion_agente_llm_autonomo.md`
- Análisis y plan: `docs/analisis_y_plan_desarrollo_sikmacode.md`
- Informe CLI/TUI: `docs/informe_cli_sikma.md`

## 🤝 Contribuir
- Usa ramas `docs/*` para documentación y abre PRs.
- Desarrollo en `feature/*`; versiones en `master` con tags.

## 📌 Estado Actual
- Proyecto unificado en la carpeta raíz.
- Build verificado (`go build ./...`).
- TUI estable y lista para pruebas intensivas.
- Próximamente: capturas de pantalla y videos.

## 📜 Licencia
Este proyecto está licenciado bajo **GNU Affero General Public License v3.0**. Ver `LICENSE`.

---
⭐ Si este proyecto te es útil, dale una estrella.

Desarrollado con ❤️ usando **Go**, **Bubble Tea** 🍵 y **Lip Gloss** 💄.
