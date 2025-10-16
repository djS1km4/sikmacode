package tools

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/djS1km4/sikmacode/internal/log"
)

// ToolCall representa una llamada a una herramienta con nombre y argumentos.
// Esta definición debe ser consistente con la que espera el paquete TUI.
type ToolCall struct {
	Name      string                 `json:"tool"`
	Arguments map[string]interface{} `json:"kwargs"`
}

// Execute es un despachador que ejecuta la herramienta correcta basada en el nombre.
func Execute(call ToolCall) (string, error) {
	switch call.Name {
	case "file:write":
		path, ok := call.Arguments["path"].(string)
		if !ok {
			return "", fmt.Errorf("argumento 'path' para file:write es inválido o no existe")
		}
		content, ok := call.Arguments["content"].(string)
		if !ok {
			return "", fmt.Errorf("argumento 'content' para file:write es inválido o no existe")
		}
		err := WriteFile(path, content)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Archivo escrito exitosamente en %s", path), nil

	case "file:read":
		path, ok := call.Arguments["path"].(string)
		if !ok {
			return "", fmt.Errorf("argumento 'path' para file:read es inválido o no existe")
		}
		content, err := ReadFile(path)
		if err != nil {
			return "", err
		}
		return content, nil

	case "bash:execute":
		command, ok := call.Arguments["command"].(string)
		if !ok {
			return "", fmt.Errorf("argumento 'command' para bash:execute es inválido o no existe")
		}
		// NOTA: Por ahora se ejecuta directamente. La confirmación se añadirá después.
		cmd := exec.Command("cmd", "/C", command) // Usar cmd /C para compatibilidad con Windows
		output, err := cmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("error al ejecutar comando: %s, salida: %s", err, string(output))
		}
		return fmt.Sprintf("Acción completada.\n%s", string(output)), nil

	default:
		return "", fmt.Errorf("herramienta desconocida: %s", call.Name)
	}
}

// ReadFile lee el contenido de un archivo y lo devuelve como un string.
func ReadFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// WriteFile escribe contenido en un archivo.
func WriteFile(path string, content string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		log.Printf("Error al obtener la ruta absoluta para %s: %v", path, err)
		return err
	}

	log.Printf("Intentando escribir en el archivo: %s", absPath)

	err = os.WriteFile(absPath, []byte(content), 0644)
	if err != nil {
		log.Printf("Falló la escritura en el archivo %s: %v", absPath, err)
	} else {
		log.Printf("Escritura exitosa en el archivo %s", absPath)
	}
	return err
}
