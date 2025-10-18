package main

import (
	"flag"
	"fmt"
	"io"
	stdlog "log"
	"os"
	"path/filepath"
	"strings"
	"time"
	"encoding/json"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/djS1km4/sikmacode/internal/agent"
	"github.com/djS1km4/sikmacode/internal/config"
	"github.com/djS1km4/sikmacode/internal/llm"
	siklog "github.com/djS1km4/sikmacode/internal/log"
	"github.com/djS1km4/sikmacode/internal/tui"
	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/iterator"
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
	streamFlag := flag.Bool("stream", false, "Muestra respuesta en streaming (solo con --prompt)")
	outFlag := flag.String("out", "", "Guardar salida del prompt en archivo (por defecto ./outputs/<timestamp>.txt)")
	logLevelFlag := flag.String("log-level", "", "Nivel de log: error|warn|info|debug")
	doctorFlag := flag.Bool("doctor", false, "Valida el entorno (configuración, API key, provider/modelo, permisos de salida)")
	doctorJSONFlag := flag.Bool("doctor-json", false, "Salida del doctor en JSON")
	flag.Parse()

	// Doctor: validación temprana del entorno
	if *doctorFlag || *doctorJSONFlag {
		if err := runDoctor(*doctorJSONFlag); err != nil {
			stdlog.Fatalf("doctor reporta errores: %v", err)
		}
		return
	}

	// --debug / --log-level: habilitar salida de log por stdout
	if *debugFlag || strings.ToLower(*logLevelFlag) == "debug" {
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

		// Preparar ruta de salida
		var outPath string
		if *outFlag == "" {
			outDir := filepath.Join(".", "outputs")
			_ = os.MkdirAll(outDir, 0o755)
			stamp := time.Now().Format("20060102-150405")
			outPath = filepath.Join(outDir, fmt.Sprintf("prompt-%s.txt", stamp))
		} else {
			outPath = *outFlag
			_ = os.MkdirAll(filepath.Dir(outPath), 0o755)
		}

		if *streamFlag {
			// Validación temprana del modelo activo
			if provider.Model == "" {
				stdlog.Fatalf("modelo activo vacío; configure --model o use --provider junto con --model")
			}

			client, iter, err := llm.StartStream(apiKey, provider.Model, systemPrompt, nil, *promptFlag)
			if err != nil {
				stdlog.Fatalf("error LLM streaming con modelo '%s': %v", provider.Model, err)
			}
			defer client.Close()

			// Indicador simple de streaming
			fmt.Print(">>> ")

			var outBuf strings.Builder
			for {
				resp, err := iter.Next()
				if err == iterator.Done {
					break
				}
				if err != nil {
					stdlog.Fatalf("error en stream: %v", err)
				}
				if resp == nil || len(resp.Candidates) == 0 {
					continue
				}
				cand := resp.Candidates[0]
				if cand.Content == nil || len(cand.Content.Parts) == 0 {
					continue
				}
				for _, p := range cand.Content.Parts {
					if t, ok := p.(genai.Text); ok {
						text := string(t)
						fmt.Print(text)
						outBuf.WriteString(text)
					}
				}
			}
			fmt.Println()
			// Guardar salida en archivo
			if err := os.WriteFile(outPath, []byte(outBuf.String()), 0o644); err == nil {
				fmt.Printf("Salida guardada en: %s\n", outPath)
			} else {
				stdlog.Printf("no se pudo guardar salida en %s: %v", outPath, err)
			}
			return
		}

		resp, err := llm.GenerateResponse(apiKey, provider.Model, systemPrompt, nil, *promptFlag)
		if err != nil {
			stdlog.Fatalf("error LLM: %v", err)
		}
		fmt.Println(resp)
		// Guardar salida en archivo
		if err := os.WriteFile(outPath, []byte(resp), 0o644); err == nil {
			fmt.Printf("Salida guardada en: %s\n", outPath)
		} else {
			stdlog.Printf("no se pudo guardar salida en %s: %v", outPath, err)
		}
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

func runDoctor(asJSON bool) error {
	var problems []string
	var oks []string

	if !asJSON {
		fmt.Println("== Sikma Code Doctor ==")
	}
	// Config
	cfg, err := config.LoadConfig()
	if err != nil {
		problems = append(problems, fmt.Sprintf("configuración: no se pudo cargar (%v)", err))
	} else {
		oks = append(oks, "Configuración cargada")
		if !asJSON {
			fmt.Println("[OK] Configuración cargada")
		}
	}
	// API Key
	if err == nil {
		if _, err := cfg.GetActiveAPIKey(); err != nil {
			problems = append(problems, fmt.Sprintf("api key: faltante o inválida (%v)", err))
		} else {
			oks = append(oks, "API key activa disponible")
			if !asJSON {
				fmt.Println("[OK] API key activa disponible")
			}
		}
	}
	// Provider / Model
	if err == nil {
		p := cfg.Providers[cfg.ActiveLLM]
		if cfg.ActiveLLM == "" {
			problems = append(problems, "provider: no configurado (use --provider)")
		} else {
			oks = append(oks, fmt.Sprintf("Provider activo: %s", cfg.ActiveLLM))
			if !asJSON {
				fmt.Printf("[OK] Provider activo: %s\n", cfg.ActiveLLM)
			}
		}
		if strings.TrimSpace(p.Model) == "" {
			problems = append(problems, "modelo: vacío (use --model)")
		} else {
			oks = append(oks, fmt.Sprintf("Modelo activo: %s", p.Model))
			if !asJSON {
				fmt.Printf("[OK] Modelo activo: %s\n", p.Model)
			}
		}
	}
	// Salida (permisos)
	outDir := filepath.Join(".", "outputs")
	_ = os.MkdirAll(outDir, 0o755)
	probe := filepath.Join(outDir, "doctor-probe.txt")
	if err := os.WriteFile(probe, []byte("ok"), 0o644); err != nil {
		problems = append(problems, fmt.Sprintf("outputs: no se pudo escribir en %s (%v)", outDir, err))
	} else {
		oks = append(oks, fmt.Sprintf("Permisos de escritura en %s", outDir))
		if !asJSON {
			fmt.Printf("[OK] Permisos de escritura en %s\n", outDir)
		}
		_ = os.Remove(probe)
	}

	// Resumen
	if asJSON {
		type DoctorReport struct {
			OK       []string `json:"ok"`
			Problems []string `json:"problems"`
		}
		report := DoctorReport{OK: oks, Problems: problems}
		b, _ := json.MarshalIndent(report, "", "  ")
		fmt.Println(string(b))
	} else {
		if len(problems) > 0 {
			fmt.Println("\n== Problemas detectados ==")
			for _, p := range problems {
				fmt.Printf("- %s\n", p)
			}
		} else {
			fmt.Println("\nTodo se ve correcto. ¡Listo para usar!")
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("%d problema(s) detectado(s)", len(problems))
	}
	return nil
}
