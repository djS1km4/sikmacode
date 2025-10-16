package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// Config es la estructura principal de configuración para Sikma Code.
type Config struct {
	ActiveLLM string               `json:"active_llm"`
	Providers map[string]LLMProvider `json:"providers"`
	Agent     AgentConfig          `json:"agent"`
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
			Tools:     []string{"file:read", "file:write", "bash:execute"},
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

// GetActiveAPIKey devuelve la API key del proveedor de LLM activo.
func (c *Config) GetActiveAPIKey() (string, error) {
	provider, ok := c.Providers[c.ActiveLLM]
	if !ok {
		return "", errors.New("proveedor de LLM activo no encontrado en la configuración")
	}

	apiKey := os.Getenv(provider.APIKeyEnv)
	if apiKey == "" {
		return "", errors.New("la variable de entorno para la API key no está configurada: " + provider.APIKeyEnv)
	}

	return apiKey, nil
}