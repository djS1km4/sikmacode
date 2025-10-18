# Changelog

## v1.0.1 — Kaomoji: parpadeo natural y doble ocasional

- Ajuste de parpadeo del kaomoji en el header para verse más suave y humano.
- Intervalo aleatorio ampliado: entre ~6 y ~11 segundos entre parpadeos.
- Cierre de ojos más lento/suave: ~180–300 ms con ojos cerrados.
- Doble parpadeo ocasional: ~1 de cada 10 ciclos, segundo cierre tras ~220–320 ms.
- Sin cambios funcionales de negocio, solo UI/UX de animación.

### Archivos afectados
- `internal/tui/model.go`: nueva lógica de temporizadores y estado `kaoDoDouble`.
- `internal/tui/styles.go`: añadidos `ThinkingStyle` y `ChipStyle` para coherencia visual.

### Detalles técnicos
- `kaomojiIdleTickCmd`: intervalos con `rand.Intn(5000)+6000` ms.
- `kaomojiResetTickCmd`: cierre más largo para sensación suave.
- `kaomojiSecondBlinkTickCmd`: segundo parpadeo breve en eventos aleatorios.
- `appModel`: nuevo campo `kaoDoDouble` inicializado en `NewAppModel`.

### Cómo probar
1. Ejecuta el TUI como de costumbre.
2. Observa el encabezado: parpadeos más pausados y ocasionales dobles.
3. Si deseas ajustar frecuencia o suavidad, avísame y lo adapto rápido.