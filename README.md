# ✨ Sikma Code ✨

**Tu compañero de programación IA de última generación, directamente en tu terminal.**

Sikma Code es un agente de codificación IA autónomo y glamuroso. Construido con Go y el ecosistema de Charm, combina una TUI (Interfaz de Usuario de Texto) elegante y fluida con potentes capacidades de planificación, ejecución y aprendizaje para potenciar tu flujo de trabajo de desarrollo.

---

## 🚀 Características Principales

*   **🤖 Agente Autónomo Inteligente:** Capaz de entender tareas complejas, planificar su ejecución, usar herramientas y auto-corregirse.
*   **💅 TUI Glamurosa:** ¡Adiós a las interfaces aburridas! Gracias a `Bubble Tea` y `Lip Gloss`, la experiencia es moderna, responsiva y personalizable.
*   **🔌 Multi-Modelo:** Configura y cambia fácilmente entre diferentes proveedores de LLM (como los modelos Gemini de Google).
*   **🧰 Acceso a Herramientas:** El agente puede leer y escribir archivos, ejecutar comandos de terminal y buscar en la web para resolver problemas.
*   **🧠 Memoria Persistente:** Sikma Code aprende de sus interacciones para mejorar continuamente (¡próximamente!).
*   **✅ Gestión de Tareas:** Mantiene un registro de los objetivos y tareas pendientes para proyectos a largo plazo.

## 🛠️ Instalación (¡Próximamente!)

El objetivo es que la instalación sea tan simple como:

```bash
go install github.com/djS1km4/sikmacode@latest
```

## ⚙️ Configuración

Sikma Code se configura mediante variables de entorno. Para empezar, solo necesitas tu clave de API de Google AI Studio:

```bash
export GEMINI_API_KEY="tu_clave_de_api_aqui"
```

## 📜 Licencia

Este proyecto está licenciado bajo la **GNU Affero General Public License v3.0**. Puedes ver el texto completo en el archivo `LICENSE`.

---

⭐ Si este proyecto te es útil, considera darle una estrella ⭐

Desarrollado con ❤️ usando **Go**, **Bubble Tea** Bubble Tea 🍵, **Lip Gloss** 💄 y **HardTechno** 🎵

---

## 🧩 Precarga Alfanumérica (TUI)

A partir de la rama `feature/tui-alphanumeric-preload`, la TUI muestra una precarga visual antes de que el agente responda: una cadena aleatoria de 16 caracteres alfanuméricos que cambia dinámicamente hasta que llegan los primeros chunks del stream.

- **Cuándo aparece:** inmediatamente tras pulsar `Enter` con tu prompt.
- **Cuándo desaparece:** al llegar el primer chunk del streaming o al finalizar la respuesta.
- **Qué se mantiene:** la barra de carga existente sigue animándose durante el streaming.

### Uso rápido

1. Compila: `go build ./cmd/sikmacode`
2. Ejecuta TUI: `./sikmacode.exe`
3. Escribe tu prompt y pulsa `Enter`.

Verás el bloque “🤖 Agente” con 16 caracteres alfanuméricos que cambian, seguido por el texto del agente en streaming.

### Personalización

- **Estilo visual (color, negrita, etc.)**
  Edita `internal/tui/styles.go` en el estilo `LoadingStyle`:

  ```go
  LoadingStyle: lipgloss.NewStyle().
      Foreground(lipgloss.Color("81")). // Azul
      Bold(true),
  ```

- **Velocidad del tick**
  Ajusta el intervalo en `internal/tui/model.go` dentro de `preloadTickCmd()`:

  ```go
  return tea.Tick(150*time.Millisecond, func(t time.Time) tea.Msg {
      return preloadTickMsg{ text: randomAlphaNum(16) }
  })
  ```

- **Longitud de la cadena**
  Cambia el `16` en `randomAlphaNum(16)` por el número de caracteres que prefieras.

- **Desactivar temporalmente**
  Si deseas desactivar la precarga por ahora, comenta la activación en la pulsación de `Enter` dentro de `Update`:

  ```go
  // m.isPreloading = true
  // m.preloadText = randomAlphaNum(16)
  // return m, tea.Batch(m.startStreamCmd(userInput), m.preloadTickCmd())
  return m, m.startStreamCmd(userInput)
  ```

> Nota: en el futuro se añadirá un flag/config para poder activar/desactivar sin tocar código.

### Demo

Consulta `sikmacode_v1.0/docs/preload-demo.md` para ver el GIF demostrativo y pasos de grabación.
