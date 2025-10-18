# Configuración del Agente LLM Autónomo

## Archivo de configuración
Usa `sikma_code.json` para parámetros generales.

Ejemplo (referencial):
```json
{
  "model": "<NOMBRE_DEL_MODELO>",
  "temperature": 0.3,
  "max_tokens": 4096,
  "log_level": "info"
}
```

## Variables de entorno
- `API_KEY`: clave del proveedor.
- `MODEL`: nombre del modelo por defecto.
- `LOG_LEVEL`: `debug|info|warn|error`.

## Buenas prácticas
- No versionar claves; usar `.gitignore` y variables de entorno.
- Validar parámetros en `internal/config` y registrar en `internal/log`.
- Mantener compatibilidad en `internal/llm` y permitir cambio de proveedor.

## Flujo de arranque
1. Cargar `sikma_code.json`.
2. Leer variables de entorno (sobre-escritura si aplica).
3. Inicializar logs.
4. Arrancar TUI (`cmd/sikmacode/main.go`).

## Diagnóstico
- Revisar `outputs/*.txt` y `AGENT_AUDIT.md`.
- Aumentar logs con `LOG_LEVEL=debug`.