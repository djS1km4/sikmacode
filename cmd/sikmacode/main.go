package main

import (
	"flag"
	"fmt"
	"io"
	stdlog "log"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/djS1km4/sikmacode/internal/agent"
	"github.com/djS1km4/sikmacode/internal/config"
	"github.com/djS1km4/sikmacode/internal/llm"
	siklog "github.com/djS1km4/sikmacode/internal/log"
	"github.com/djS1km4/sikmacode/internal/tui"
)

func main() {
	// Flags
	sessionName := flag.String("session", "", "Nombre de la sesión a cargar o crear.")
	configPath := flag.String("config", "", "Ruta a archivo de configuración para instalar (opcional).")
	initFlag := flag.Bool("init", false, "Inicializa rutas y archivos por defecto en el sistema.")
	providerFlag := flag.String("provider", "", "Selecciona el proveedor activo (ej: gemini).")
	modelFlag := flag.String("model", "", "Selecciona el modelo del proveedor activo.")
	promptFlag := flag.String("prompt", "", "Modo no interactivo: envía un prompt y muestra la respuesta.")
	cwdFlag := flag.String("cwd", "", "Establece el directorio de trabajo antes de iniciar.")
	debugFlag := flag.Bool("debug", false, "Habilita modo debug con logs en stdout.")
	timeoutFlag := flag.Int("timeout", 0, "Actualiza el timeout por defecto (segundos) para bash:execute")
	flag.Parse()

	// --debug: habilitar salida de log por stdout
	if *debugFlag {
		os.Setenv("SIKMACODE_DEBUG", "true")
		siklog.EnableDebug()
	}

	// --cwd: cambiar directorio de trabajo
	if *cwdFlag != "" {
		if err := os.Chdir(*cwdFlag); err != nil {
			siklog.Printf("no se pudo cambiar cwd a %s: %v", *cwdFlag, err)
		} else {
			siklog.Printf("cwd establecido a: %s", *cwdFlag)
		}
	}

	// --init: prepara estructura de configuración
	if *initFlag {
		if err := ensureDefaultConfigInstalled(); err != nil {
			stdlog.Fatalf("error en init: %v", err)
		}
		fmt.Println("Init completado: configuración instalada si faltaba.")
		return
	}

	// --config: instalar archivo de configuración custom si se facilita
	if *configPath != "" {
		if err := installConfigFromPath(*configPath); err != nil {
			stdlog.Fatalf("no se pudo instalar configuracion: %v", err)
		}
		fmt.Println("Configuración instalada desde ruta proporcionada.")
	}

	// provider/model: actualizar configuración activa
	if *providerFlag != "" || *modelFlag != "" {
		cfg, err := config.LoadConfig()
		if err != nil {
			stdlog.Fatalf("no se pudo cargar configuración: %v", err)
		}
		if *providerFlag != "" {
			cfg.ActiveLLM = *providerFlag
			if _, ok := cfg.Providers[*providerFlag]; !ok {
				cfg.Providers[*providerFlag] = config.LLMProvider{
					Name:      *providerFlag,
					Model:     "",
					APIKeyEnv: "GEMINI_API_KEY",
				}
			}
		}
		if *modelFlag != "" {
			p := cfg.Providers[cfg.ActiveLLM]
			p.Model = *modelFlag
			cfg.Providers[cfg.ActiveLLM] = p
		}
		if err := config.SaveConfig(cfg); err != nil {
			stdlog.Fatalf("no se pudo guardar configuración: %v", err)
		}
		fmt.Println("Proveedor/modelo actualizados en configuración.")
	}

	// NUEVO: --timeout para actualizar el timeout por defecto de seguridad
	if *timeoutFlag > 0 {
		cfg, err := config.LoadConfig()
		if err != nil {
			stdlog.Fatalf("no se pudo cargar configuración: %v", err)
		}
		cfg.Security.DefaultTimeoutSec = *timeoutFlag
		if err := config.SaveConfig(cfg); err != nil {
			stdlog.Fatalf("no se pudo guardar configuración: %v", err)
		}
		fmt.Printf("Timeout por defecto actualizado a %d segundos.\n", *timeoutFlag)
	}

	// Modo no interactivo: enviar prompt y salir
	if *promptFlag != "" {
		cfg, err := config.LoadConfig()
		if err != nil {
			stdlog.Fatalf("no se pudo cargar configuracion: %v", err)
		}
		apiKey, err := cfg.GetActiveAPIKey()
		if err != nil {
			stdlog.Fatalf("api key: %v", err)
		}
		provider := cfg.Providers[cfg.ActiveLLM]
		ag := agent.NewAgent(cfg.Agent)
		systemPrompt := ag.BuildSystemPrompt()
		resp, err := llm.GenerateResponse(apiKey, provider.Model, systemPrompt, nil, *promptFlag)
		if err != nil {
			stdlog.Fatalf("error LLM: %v", err)
		}
		fmt.Println(resp)
		return
	}

	// Modo TUI
	p := tea.NewProgram(tui.NewAppModel(*sessionName), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		stdlog.Fatal(err)
	}
}

func ensureDefaultConfigInstalled() error {
	cfgPath, err := config.GetConfigPath()
	if err != nil {
		return err
	}
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		// Intentar copiar desde repo local si existe
		repoCfg := filepath.Join(".", "sikma_code.json")
		if _, err := os.Stat(repoCfg); err == nil {
			if err := copyFile(repoCfg, cfgPath); err != nil {
				return err
			}
			return nil
		}
		// Si no existe, crear por defecto
		_, err := config.LoadConfig()
		return err
	}
	return nil
}

func installConfigFromPath(src string) error {
	cfgPath, err := config.GetConfigPath()
	if err != nil {
		return err
	}
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		return copyFile(src, cfgPath)
	}
	// Si ya existe, no sobrescribir; sólo informar
	fmt.Printf("Configuración ya existe en %s, no se sobrescribe.\n", cfgPath)
	return nil
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	s, err := os.Open(src)
	if err != nil {
		return err
	}
	defer s.Close()
	d, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer d.Close()
	_, err = io.Copy(d, s)
	return err
}
