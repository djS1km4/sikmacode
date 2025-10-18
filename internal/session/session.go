package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/generative-ai-go/genai"
)

// SerializableMessage es una estructura simplificada para poder guardar y cargar mensajes en formato JSON.
type SerializableMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// getSessionsDir devuelve la ruta completa al directorio de sesiones.
func getSessionsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "sikmacode", "sessions"), nil
}

// SaveSession guarda el historial de una conversación en un archivo JSON.
func SaveSession(name string, history []*genai.Content) error {
	sessionsDir, err := getSessionsDir()
	if err != nil {
		return err
	}

	// Crear el directorio de sesiones si no existe.
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		return err
	}

	// Convertir el historial de genai.Content a un formato serializable.
	serializableHistory := make([]SerializableMessage, 0, len(history))
	for _, msg := range history {
		if len(msg.Parts) > 0 {
			if txt, ok := msg.Parts[0].(genai.Text); ok {
				serializableHistory = append(serializableHistory, SerializableMessage{
					Role:    msg.Role,
					Content: string(txt),
				})
			}
		}
	}

	// Convertir a JSON.
	data, err := json.MarshalIndent(serializableHistory, "", "  ")
	if err != nil {
		return err
	}

	// Guardar el archivo.
	filePath := filepath.Join(sessionsDir, fmt.Sprintf("%s.json", name))
	return os.WriteFile(filePath, data, 0644)
}

// LoadSession carga el historial de una conversación desde un archivo JSON.
func LoadSession(name string) ([]*genai.Content, error) {
	sessionsDir, err := getSessionsDir()
	if err != nil {
		return nil, err
	}

	filePath := filepath.Join(sessionsDir, fmt.Sprintf("%s.json", name))

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var serializableHistory []SerializableMessage
	err = json.Unmarshal(data, &serializableHistory)
	if err != nil {
		return nil, err
	}

	// Convertir el historial serializable de nuevo a []*genai.Content.
	history := make([]*genai.Content, 0, len(serializableHistory))
	for _, sMsg := range serializableHistory {
		history = append(history, &genai.Content{
			Parts: []genai.Part{genai.Text(sMsg.Content)},
			Role:  sMsg.Role,
		})
	}

	return history, nil
}

// ListSessions devuelve una lista de nombres de sesiones disponibles (sin la extensión .json).
func ListSessions() ([]string, error) {
	sessionsDir, err := getSessionsDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		// Si el directorio no existe aún, devolver lista vacía
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if filepath.Ext(name) == ".json" {
			base := name[:len(name)-len(".json")]
			names = append(names, base)
		}
	}
	return names, nil
}

// DeleteSession elimina la sesión indicada (archivo .json del historial).
func DeleteSession(name string) error {
	sessionsDir, err := getSessionsDir()
	if err != nil {
		return err
	}
	path := filepath.Join(sessionsDir, fmt.Sprintf("%s.json", name))
	if _, err := os.Stat(path); err != nil {
		return err
	}
	return os.Remove(path)
}

// RenameSession renombra el archivo de sesión de oldName a newName.
func RenameSession(oldName, newName string) error {
	if newName == "" {
		return fmt.Errorf("nuevo nombre de sesión vacío")
	}
	sessionsDir, err := getSessionsDir()
	if err != nil {
		return err
	}
	oldPath := filepath.Join(sessionsDir, fmt.Sprintf("%s.json", oldName))
	newPath := filepath.Join(sessionsDir, fmt.Sprintf("%s.json", newName))
	if _, err := os.Stat(oldPath); err != nil {
		return err
	}
	if _, err := os.Stat(newPath); err == nil {
		return fmt.Errorf("ya existe una sesión con el nombre '%s'", newName)
	}
	return os.Rename(oldPath, newPath)
}