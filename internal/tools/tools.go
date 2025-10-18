package tools

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/djS1km4/sikmacode/internal/config"
	"github.com/djS1km4/sikmacode/internal/log"
	"github.com/sourcegraph/go-diff/diff"
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

	case "file:patch":
		path, ok := call.Arguments["path"].(string)
		if !ok {
			return "", fmt.Errorf("argumento 'path' para file:patch es inválido o no existe")
		}
		diffText, ok := call.Arguments["diff"].(string)
		if !ok {
			return "", fmt.Errorf("argumento 'diff' para file:patch es inválido o no existe")
		}
		if err := applyUnifiedPatch(path, diffText); err != nil {
			return "", err
		}
		return fmt.Sprintf("Parche aplicado correctamente a %s", path), nil

	case "search:web":
		query, ok := call.Arguments["query"].(string)
		if !ok {
			return "", fmt.Errorf("argumento 'query' para search:web es inválido o no existe")
		}
		results, err := webSearchBing(query)
		if err != nil {
			return "", err
		}
		return results, nil

	case "memory:write":
		text, ok := call.Arguments["text"].(string)
		if !ok {
			return "", fmt.Errorf("argumento 'text' para memory:write es inválido o no existe")
		}
		if err := appendToFile("AGENT_MEMORY.log", time.Now().Format(time.RFC3339)+" | "+text+"\n"); err != nil {
			return "", err
		}
		return "Memoria persistida.", nil

	case "todo:write":
		desc, ok := call.Arguments["text"].(string)
		if !ok {
			return "", fmt.Errorf("argumento 'text' para todo:write es inválido o no existe")
		}
		id, err := appendTodo(desc)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("TODO agregado (id: %d).", id), nil

	case "todo:update":
		idFloat, ok := call.Arguments["id"].(float64)
		if !ok {
			return "", fmt.Errorf("argumento 'id' para todo:update es inválido o no existe")
		}
		status, ok := call.Arguments["status"].(string)
		if !ok {
			return "", fmt.Errorf("argumento 'status' para todo:update es inválido o no existe")
		}
		if err := updateTodo(int(idFloat), status); err != nil {
			return "", err
		}
		return fmt.Sprintf("TODO %d actualizado a '%s'.", int(idFloat), status), nil

	case "ask_user_confirmation":
		// Este tool es gestionado por la TUI para solicitar confirmación humana.
		prompt, _ := call.Arguments["prompt"].(string)
		return fmt.Sprintf("Solicitud de confirmación: %s", prompt), nil

	case "bash:execute":
		command, ok := call.Arguments["command"].(string)
		if !ok {
			return "", fmt.Errorf("argumento 'command' para bash:execute es inválido o no existe")
		}
		if blocked := isDangerousCommand(command); blocked {
			return "", fmt.Errorf("comando bloqueado por denylist de seguridad")
		}
		// Timeout por defecto desde configuración, sobreescribible por argumento
		timeoutSec := 15
		if cfg, err := config.LoadConfig(); err == nil && cfg != nil && cfg.Security.DefaultTimeoutSec > 0 {
			timeoutSec = cfg.Security.DefaultTimeoutSec
		}
		if v, ok := call.Arguments["timeout"].(float64); ok {
			timeoutSec = int(v)
		} else if v2, ok := call.Arguments["timeout"].(int); ok {
			timeoutSec = v2
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "cmd", "/C", command)
		output, err := cmd.CombinedOutput()
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("tiempo excedido al ejecutar comando tras %ds", timeoutSec)
		}
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
		return err // Devuelve el error original de escritura
	}

	// Paso de verificación: intentar leer el archivo inmediatamente después de escribirlo.
	_, err = os.ReadFile(absPath)
	if err != nil {
		log.Printf("VERIFICACIÓN FALLIDA: El archivo no se pudo leer después de escribir: %v", err)
		return fmt.Errorf("la escritura pareció exitosa pero la verificación de lectura falló: %w", err)
	}

	log.Printf("Escritura y verificación exitosa en el archivo %s", absPath)
	return nil // Éxito confirmado
}

// applyUnifiedPatch intenta aplicar un parche unified diff al archivo dado.
func applyUnifiedPatch(path string, diffText string) error {
	orig, err := ReadFile(path)
	if err != nil {
		return err
	}
	fDiffs, err := diff.ParseMultiFileDiff([]byte(diffText))
	if err != nil {
		return fmt.Errorf("no se pudo parsear el diff: %w", err)
	}
	// Si el diff corresponde a múltiples archivos, aplicamos el primero que afecta al path
	newContent := orig
	for _, fd := range fDiffs {
		// Intentar aplicar sólo si el archivo objetivo concuerda (cuando sea posible)
		if fd.NewName != path && fd.OrigName != path && !strings.HasSuffix(fd.NewName, filepath.Base(path)) && !strings.HasSuffix(fd.OrigName, filepath.Base(path)) {
			continue
		}
		var builder strings.Builder
		origLines := strings.Split(newContent, "\n")
		origIdx := 0
		for _, h := range fd.Hunks {
			// Escribir contenido sin cambios hasta el inicio del hunk
			start := int(h.OrigStartLine - 1)
			for origIdx < start && origIdx < len(origLines) {
				builder.WriteString(origLines[origIdx])
				builder.WriteString("\n")
				origIdx++
			}
			// Aplicar el cuerpo del hunk
			body := string(h.Body)
			for _, line := range strings.Split(body, "\n") {
				if line == "" {
					continue
				}
				switch line[0] {
				case ' ':
					// línea sin cambio: copiar de original y avanzar
					if origIdx < len(origLines) {
						builder.WriteString(origLines[origIdx])
						builder.WriteString("\n")
						origIdx++
					}
				case '-':
					// eliminar: avanzar en original sin escribir
					if origIdx < len(origLines) {
						origIdx++
					}
				case '+':
					// agregar: escribir el contenido nuevo (sin '+')
					builder.WriteString(line[1:])
					builder.WriteString("\n")
				}
			}
		}
		// Escribir el resto del original
		for origIdx < len(origLines) {
			builder.WriteString(origLines[origIdx])
			builder.WriteString("\n")
			origIdx++
		}
		newContent = builder.String()
	}
	return WriteFile(path, newContent)
}

// webSearchBing obtiene resultados de búsqueda simples de Bing y devuelve texto legible.
func webSearchBing(query string) (string, error) {
	endpoint := "https://www.bing.com/search?q=" + url.QueryEscape(query)
	resp, err := http.Get(endpoint)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d al consultar búsqueda web", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	html := string(body)
	// Extraer títulos y URLs con heurística sencilla
	re := regexp.MustCompile(`<li class="b_algo"[\s\S]*?<h2>[\s\S]*?<a href="([^"]+)"[\s\S]*?>([\s\S]*?)</a>`)
	matches := re.FindAllStringSubmatch(html, 5)
	if len(matches) == 0 {
		return "Sin resultados parseables.", nil
	}
	var b strings.Builder
	b.WriteString("Resultados web (top 5):\n")
	for i, m := range matches {
		url := strings.TrimSpace(m[1])
		title := strings.TrimSpace(stripTags(m[2]))
		b.WriteString(fmt.Sprintf("%d. %s\n   %s\n", i+1, title, url))
	}
	return b.String(), nil
}

func stripTags(s string) string {
	re := regexp.MustCompile("<[^>]+>")
	return re.ReplaceAllString(s, "")
}

// Seguridad básica: detectar comandos peligrosos
func isDangerousCommand(cmd string) bool {
    // Usar denylist desde configuración si está disponible
    cfg, _ := config.LoadConfig()
    patterns := []string{"rm -rf", "rd /s /q", "del /q", "format ", "mkfs", "shutdown", "reboot", "mkpartition", "bcdedit", "reg delete"}
    if cfg != nil && len(cfg.Security.Denylist) > 0 {
        patterns = cfg.Security.Denylist
    }
    low := strings.ToLower(cmd)
    for _, p := range patterns {
        if strings.Contains(low, strings.ToLower(p)) {
            return true
        }
    }
    return false
}

// Detectar comandos críticos que requieren doble confirmación (configurable)
func IsCriticalCommand(cmd string) bool {
    cfg, _ := config.LoadConfig()
    patterns := []string{"git push", "reset --hard", "terraform apply", "docker rmi", "npm publish", "choco install", "pip install -U", "setx ", "reg add"}
    if cfg != nil && len(cfg.Security.Critical) > 0 {
        patterns = cfg.Security.Critical
    }
    low := strings.ToLower(cmd)
    for _, p := range patterns {
        if strings.Contains(low, strings.ToLower(p)) {
            return true
        }
    }
    return false
}

// Helpers para archivos simples
func appendToFile(path string, text string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(text)
	return err
}

func appendTodo(desc string) (int, error) {
	path := "AGENT_TODOS.md"
	// calcular próximo id
	content := ""
	if b, err := os.ReadFile(path); err == nil {
		content = string(b)
	}
	lines := strings.Split(content, "\n")
	maxID := 0
	idRe := regexp.MustCompile(`\(id: (\d+)\)$`)
	for _, ln := range lines {
		m := idRe.FindStringSubmatch(ln)
		if len(m) == 2 {
			if v := strings.TrimSpace(m[1]); v != "" {
				if n, err := strconv.Atoi(v); err == nil && n > maxID {
					maxID = n
				}
			}
		}
	}
	newID := maxID + 1
	entry := fmt.Sprintf("- [ ] %s (id: %d)\n", desc, newID)
	if err := appendToFile(path, entry); err != nil {
		return 0, err
	}
	return newID, nil
}

func updateTodo(id int, status string) error {
	path := "AGENT_TODOS.md"
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(b), "\n")
	idRe := regexp.MustCompile(fmt.Sprintf(`\(id: %d\)$`, id))
	for i, ln := range lines {
		if idRe.MatchString(ln) {
			if strings.Contains(ln, "- [ ]") {
				ln = strings.Replace(ln, "- [ ]", "- [x]", 1)
			} else if strings.Contains(ln, "- [x]") && status == "pending" {
				ln = strings.Replace(ln, "- [x]", "- [ ]", 1)
			}
			lines[i] = ln
			break
		}
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644)
}
