# Informe CLI Sikma — Uso y comportamiento

## Ejecución
- Comando: `go run cmd\sikmacode\main.go`
- Verificar inicio de TUI, menús y paneles.

## Flujos validados
- Sesión: crear, operar, cerrar; export/import si aplica.
- Modelos: cambio y ajustes (temperatura/tokens).
- Temas: alternar y confirmar cambios visuales.
- Generación CLI: plantilla mínima, prompts guiados y resultados.

## Observabilidad
- Auditar eventos en `AGENT_AUDIT.md`.
- Logs en `outputs/*.txt`.

## Errores conocidos
- Ajustes de layout pueden requerir revisión según terminal.
- Advertencias de CRLF/LF en Windows (sin impacto funcional).

## Próximos pasos
- Capturar capturas y trazas de sesiones representativas.
- Completar métricas de rendimiento (latencia, tokens, errores).