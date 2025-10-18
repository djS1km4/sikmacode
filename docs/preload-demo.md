# Demo: Precarga Alfanumérica en la TUI

Este documento acompaña el PR `feature/tui-alphanumeric-preload` y describe cómo grabar y visualizar el GIF demostrativo de la precarga alfanumérica.

## ¿Qué verás?
- Al pulsar `Enter` tras escribir un prompt, aparece un bloque “🤖 Agente” con **16 caracteres alfanuméricos** que cambian cada ~150 ms.
- Al llegar el primer chunk de la respuesta del modelo, la precarga desaparece y se muestra el contenido en streaming.
- Se mantiene la **barra de carga** durante el streaming.

## Grabación del GIF (Windows)
1. Instala [ScreenToGif](https://www.screentogif.com/).
2. Abre `./sikmacode.exe` en tu terminal.
3. Activa el área de grabación sobre la ventana de la TUI.
4. Escribe un prompt y pulsa `Enter`.
5. Para cuando empiece a llegar el texto, detén la grabación.
6. Guarda como `preload-demo.gif` y coloca el archivo en `sikmacode_v1.0/docs/preload-demo.gif`.

> Alternativas: **ShareX**, **OBS Studio**.

## Incluir el GIF en el PR
- Añade el archivo `sikmacode_v1.0/docs/preload-demo.gif` al commit del PR.
- Referéncialo desde el README o este documento:

```markdown
![Precarga Alfanumérica](./preload-demo.gif)
```

## Configuración rápida
- Color/estilo del texto: `internal/tui/styles.go` → `LoadingStyle`.
- Velocidad de cambio: `internal/tui/model.go` → `preloadTickCmd()` intervalo (por defecto `150*time.Millisecond`).
- Longitud del texto: `internal/tui/model.go` → `randomAlphaNum(16)`.