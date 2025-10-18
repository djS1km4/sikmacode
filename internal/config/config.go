package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// SecurityConfig define opciones de seguridad para herramientas destructivas.
type SecurityConfig struct {
	Denylist          []string `json:"denylist"`
	Critical          []string `json:"critical"`
	DefaultTimeoutSec int      `json:"default_timeout_sec"`
}

// Config es la estructura principal de configuración para Sikma Code.
type Config struct {
	ActiveLLM string                 `json:"active_llm"`
	Providers map[string]LLMProvider `json:"providers"`
	Agent     AgentConfig            `json:"agent"`
	Security  SecurityConfig         `json:"security"`
}

// LLMProvider define la configuración para un proveedor de LLM específico.
type LLMProvider struct {
	Name      string `json:"name"`
	Model     string `json:"model"`
	APIKeyEnv string `json:"api_key_env"` // Variable de entorno que contiene la API key
}

// AgentConfig define la configuración para el agente autónomo.
type AgentConfig struct {
	Role      string   `json:"role"`
	Goal      string   `json:"goal"`
	Backstory string   `json:"backstory"`
	Tools     []string `json:"tools"`
}

// GetConfigPath devuelve la ruta completa al archivo de configuración.
func GetConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "sikmacode", "sikma_code.json"), nil
}

// LoadConfig carga la configuración desde el archivo sikma_code.json.
// Si el archivo no existe, crea uno por defecto.
func LoadConfig() (*Config, error) {
	path, err := GetConfigPath()
	if err != nil {
		return nil, err
	}

	file, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return createDefaultConfig(path)
		}
		return nil, err
	}

	var cfg Config
	err = json.Unmarshal(file, &cfg)
	if err != nil {
		return nil, err
	}

	return &cfg, nil
}

// createDefaultConfig crea un archivo de configuración por defecto.
func createDefaultConfig(path string) (*Config, error) {
	cfg := Config{
		ActiveLLM: "gemini",
		Providers: map[string]LLMProvider{
			"gemini": {
				Name:      "Gemini",
				Model:     "gemini-2.5-flash",
				APIKeyEnv: "GEMINI_API_KEY",
			},
		},
		Agent: AgentConfig{
			Role:      "Ingeniero de Software Senior Autónomo",
			Goal:      "Asistir en el ciclo completo de desarrollo de software.",
			Backstory: "Eres un agente de IA de última generación.",
			Tools:     []string{"file:read", "file:write", "file:patch", "bash:execute", "search:web", "ask_user_confirmation", "memory:write", "todo:write", "todo:update"},
		},
		Security: SecurityConfig{
			Denylist: []string{"rm -rf", "rd /s /q", "del /q", "format ", "mkfs", "shutdown", "reboot", "mkpartition", "bcdedit", "reg delete"},
			Critical: []string{"git push", "reset --hard", "terraform apply", "docker rmi", "npm publish", "choco install", "pip install -U", "setx ", "reg add"},
			DefaultTimeoutSec: 15,
		},
	}

	file, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, err
	}

	// Asegurarse de que el directorio exista
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	if err := os.WriteFile(path, file, 0644); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// getEnvStorePath devuelve la ruta del archivo donde se almacenan las API keys de forma persistente.
func getEnvStorePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "sikmacode", "envvars.json"), nil
}

// SaveEnvVar guarda de forma persistente un par ENV->valor.
func SaveEnvVar(env, value string) error {
	path, err := getEnvStorePath()
	if err != nil {
		return err
	}
	m := map[string]string{}
	if b, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(b, &m)
	}
	m[env] = value
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// LoadEnvVar carga el valor almacenado para una ENV si existe.
func LoadEnvVar(env string) (string, error) {
	path, err := getEnvStorePath()
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	m := map[string]string{}
	if err := json.Unmarshal(b, &m); err != nil {
		return "", err
	}
	return m[env], nil
}

// GetActiveAPIKey devuelve la API key del proveedor de LLM activo.
func (c *Config) GetActiveAPIKey() (string, error) {
	provider, ok := c.Providers[c.ActiveLLM]
	if !ok {
		return "", errors.New("proveedor de LLM activo no encontrado en la configuración")
	}

	apiKey := os.Getenv(provider.APIKeyEnv)
	if apiKey == "" {
		// Fallback: leer del almacén persistente
		if v, err := LoadEnvVar(provider.APIKeyEnv); err == nil && v != "" {
			apiKey = v
		}
	}
	if apiKey == "" {
		return "", errors.New("la variable de entorno para la API key no está configurada: " + provider.APIKeyEnv)
	}
	return apiKey, nil
}

func SaveConfig(cfg *Config) error {
    path, err := GetConfigPath()
    if err != nil {
        return err
    }
    data, err := json.MarshalIndent(cfg, "", "  ")
    if err != nil {
        return err
    }
    if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
        return err
    }
    return os.WriteFile(path, data, 0o644)
}