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
- Variables de entorno:
  - `GEMINI_API_KEY`: clave de Google AI Studio (obligatoria).
  - `MODEL`: nombre del modelo LLM (por defecto `gemini-2.5-pro`).
  - `GLAMOUR_STYLE`: tema para Markdown en la TUI (`dark`, `light`, `dracula`, etc.).
  - `LOG_LEVEL`: `debug|info|warn|error`.
  - `TEMPERATURE`: valor `float` para control de creatividad (ej. `0.2`).
  - `MAX_OUTPUT_TOKENS`: límite de tokens de salida (ej. `2048`).
- Flags CLI:
  - `--model <nombre>`: fuerza el modelo para la sesión actual.
  - `--config <ruta>`: carga `sikma_code.json` (por defecto `./sikma_code.json`).
  - `--temperature <float>`: establece `TEMPERATURE`.
  - `--max-tokens <int>`: establece `MAX_OUTPUT_TOKENS`.
- Prioridades de configuración (de mayor a menor): `flags` > `entorno` > `JSON` > `por defecto`.
- Archivo de configuración (`sikma_code.json`):
  ```json
  {
    "model": "gemini-2.5-pro",
    "temperature": 0.2,
    "max_output_tokens": 2048
  }
  ```
- Ejemplos (PowerShell):
  ```powershell
  # Mínimo para correr (API key obligatoria)
  $env:GEMINI_API_KEY = "<tu_api_key>"
  
  # Forzar modelo y parámetros vía flags (prioridad máxima)
  .\sikmacode.exe --model "gemini-2.5-flash" --temperature 0.2 --max-tokens 2048
  
  # Usar archivo JSON (por defecto ./sikma_code.json o con --config)
  .\sikmacode.exe --config ".\sikma_code.json"
  
  # Fallback vía entorno si no hay flags
  $env:TEMPERATURE = "0.3"; $env:MAX_OUTPUT_TOKENS = "1024"
  .\sikmacode.exe
  ```

## 🖥️ Uso Rápido (TUI)
- Navegación con flechas, `Enter`, `Esc`, `Tab`.
- Respuestas se muestran con Markdown estilizado en el `viewport` (Glamour).
- Cambia modelo y parámetros desde el menú.
- Gestiona sesiones (archivos en `sessions/`) y reanuda tu trabajo.
- Observa auditoría y registros en `AGENT_AUDIT.md` y `outputs/*.txt`.

## ⌨️ Atajos de Teclado
- `Enter`: envía el mensaje y limpia el campo (sin salto de línea).
- `Esc` o `Ctrl+C`: sale de la aplicación.
- `↑`/`↓`: desplaza la conversación.
- `PageUp`/`PageDown`: desplazamiento rápido.
- `Home`/`End`: ir al inicio/fin del historial.
- Nota: el `textarea` y el `viewport` heredan atajos estándar de Bubble Tea (flechas y paginación). No se admite salto de línea en el área de entrada por ahora.

## 📚 Documentación y Roadmap
- Auditoría y plan: `docs/auditoria_avance_sikmacode.md`
- Hoja de ruta: `docs/hoja de ruta SikmaCode.md`
- Guía de configuración: `docs/Configuración Agente LLM Autónomo.md`
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
