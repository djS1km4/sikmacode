package tui

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"time"
	// nuevos imports
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/djS1km4/sikmacode/internal/agent"
	"github.com/djS1km4/sikmacode/internal/config"
	"github.com/djS1km4/sikmacode/internal/llm"
	"github.com/djS1km4/sikmacode/internal/session"
	"github.com/djS1km4/sikmacode/internal/tools"
	"github.com/google/generative-ai-go/genai"
	"errors"
	"io"
)

const logo = `
░██████╗██╗██╗░░██╗███╗░░░███╗░█████╗░░█████╗░░█████╗░██████╗░███████╗
██╔════╝██║██║░██╔╝████╗░████║██╔══██╗██╔══██╗██╔══██╗██╔══██╗██╔════╝
╚█████╗░██║█████═╝░██╔████╔██║███████║██║░░╚═╝██║░░██║██║░░██║█████╗░░
░╚═══██╗██║██╔═██╗░██║╚██╔╝██║██╔══██║██║░░██╗██║░░██║██║░░██║██╔══╝░░
██████╔╝██║██║░╚██╗██║░╚═╝░██║██║░░██║╚█████╔╝╚█████╔╝██████╔╝███████╗
╚═════╝╚╝╚═╝░░╚═╝╚═╝░░░░░╚═╝╚═╝░░╚═╝░╚════╝░░╚════╝░╚══════╝
`

const smallLogo = "≧◉◡◉≦"

type completionMsg struct {
	content string
}

type errorMsg struct {
	err error
}

// Mensajes para streaming
type streamChunkMsg struct{ chunk string }
type streamDoneMsg struct{}

type loadingTickMsg struct{ frame string }

// Nuevo: mensaje de frase de pensamiento
type thinkingTickMsg struct{ phrase string }

// Nuevo: animación de chips
type chipAnimTickMsg struct{}
// Nuevo: parpadeo de kaomoji
type kaomojiBlinkMsg struct{}
type kaomojiResetMsg struct{}

func (e errorMsg) Error() string { return e.err.Error() }

type appModel struct {
	viewport         viewport.Model
	textarea         textarea.Model
	messages         []*genai.Content
	styles           Styles
	config           *config.Config
	apiKey           string
	sessionName      string
	keyMap           KeyMap
	isReady          bool
	isSplashVisible  bool
	isConfirming     bool
	confirmPrompt    string
	confirmationType string
	pendingCall      *tools.ToolCall
	allowedTools     map[string]bool
	agent            *agent.Agent
	err              error
	width, height    int
	// Streaming
	isStreaming  bool
	streamBuffer strings.Builder
	streamClient llm.StreamClient
	streamIter   llm.StreamIterator
	// Barra de carga
	loadingBar string
	// Precarga (antes de responder)
	isPreloading bool
	preloadText  string
	// Texto motivacional en cabecera durante pensamiento
	thinkingPhrase string
	streamStart    time.Time
	// Metadatos última respuesta para chips
	lastResponseDuration    time.Duration
	lastResponseTokenApprox int
	lastResponseProvider    string
	lastResponseModel       string
	// Ayuda
	showHelp bool
	// Seguridad UX
	awaitingDoubleConfirm bool
	// Spinner visual
	spin spinner.Model
	// Tema/acento actual
	themeAccentIdx int
	// Progreso animado (Harmonica via Bubbles)
	prog progress.Model
	// Nuevo: offset de animación para chips
	chipAnimOffset int
	// Nuevo: kaomoji actual (para parpadeo sutil)
	currentKao string
	kaoDoDouble bool
	// Nuevo: menú MODELOS
	showModels       bool
	modelsLastOutput string
	// Nuevo: catálogo aplanado de modelos
	modelsFlat         []modelEntry
	modelsSelectedIdx  int
	modelsInput        textarea.Model
	modelsCaptureInput bool
	// Nuevo: menú SESIONES interactivo
	showSessions         bool
	sessions             []string
	sessionsSelectedIdx  int
	sessionsInput        textarea.Model
	sessionsCaptureInput bool
	// Viewports para menús con scroll
	modelsViewport   viewport.Model
	sessionsViewport viewport.Model
	// Scrollbar config y métricas
	scrollbarWidth            int
	viewportContentLineCount int
}

func NewAppModel(sessionName string) *appModel {
	styles := DefaultStyles()
	ta := textarea.New()
	ta.Placeholder = "Escribe tu mensaje aquí..."
	ta.Focus()
	ta.Prompt = "> "
	ta.ShowLineNumbers = false
	ta.SetHeight(3)
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()

	vp := viewport.New(80, 20)
	ta.KeyMap.InsertNewline.SetEnabled(false)

	// Inicializar spinner visual
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("178"))
	// Progreso con gradiente y resorte
	pr := progress.New(
		progress.WithDefaultScaledGradient(),
		progress.WithSpringOptions(6.0, 0.5),
	)

	cfg, _ := config.LoadConfig()
	var apiKey string
	if cfg != nil {
		apiKey, _ = cfg.GetActiveAPIKey()
	}

	var messages []*genai.Content
	if sessionName != "" {
		loadedMessages, err := session.LoadSession(sessionName)
		if err == nil {
			messages = loadedMessages
		}
	} else {
		sessionName = fmt.Sprintf("session-%d", time.Now().Unix())
	}

	var agentInstance *agent.Agent
	if cfg != nil {
		agentInstance = agent.NewAgent(cfg.Agent)
	}
	// Sembrar aleatoriedad para barras y precarga
	rand.Seed(time.Now().UnixNano())

	m := &appModel{
		textarea:        ta,
		viewport:        vp,
		styles:          styles,
		config:          cfg,
		apiKey:          apiKey,
		sessionName:     sessionName,
		messages:        messages,
		keyMap:          DefaultKeyMap(),
		isReady:         false,
		isSplashVisible: true,
		allowedTools:    make(map[string]bool),
		agent:           agentInstance,
		spin:            sp,
		themeAccentIdx:  0,
		prog:            pr,
		chipAnimOffset:  0,
		currentKao:      "≧◉◡◉≦",
		kaoDoDouble:     false,
		showModels:      false,
		modelsLastOutput: "",
	}
	// Inicialización de menús interactivos (MODELOS y SESIONES)
	catalog := loadProviderCatalog()
	var flat []modelEntry
	for id, prov := range catalog.Providers {
		for _, model := range prov.Models {
			flat = append(flat, modelEntry{ProviderID: id, ProviderName: prov.Name, EnvVar: prov.EnvVar, ModelID: model})
		}
	}
	sort.SliceStable(flat, func(i, j int) bool {
		if strings.ToLower(flat[i].ProviderName) == strings.ToLower(flat[j].ProviderName) {
			return flat[i].ModelID < flat[j].ModelID
		}
		return strings.ToLower(flat[i].ProviderName) < strings.ToLower(flat[j].ProviderName)
	})
	m.modelsFlat = flat
	m.modelsSelectedIdx = 0
	m.modelsInput = textarea.New()
	m.modelsInput.Prompt = ""
	m.modelsInput.Placeholder = "Ingrese API key y Enter"
	m.modelsInput.ShowLineNumbers = false
	m.modelsInput.SetHeight(1)
	m.modelsCaptureInput = false
	m.showSessions = false
	if names, err := session.ListSessions(); err == nil {
		m.sessions = names
	} else {
		m.sessions = nil
	}
	m.sessionsSelectedIdx = 0
	m.sessionsInput = textarea.New()
	m.sessionsInput.Prompt = ""
	m.sessionsInput.Placeholder = "Renombrar o escribir 'DEL' para borrar"
	m.sessionsInput.ShowLineNumbers = false
	m.sessionsInput.SetHeight(1)
	m.sessionsCaptureInput = false

	// Viewports para listas de menús con scrollbar
	m.modelsViewport = viewport.New(80, 10)

	m.sessionsViewport = viewport.New(80, 10)

	// Configuración de scrollbar visual
	m.scrollbarWidth = 1

	return m
}

func (m *appModel) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, m.kaomojiIdleTickCmd())
}

// Comando para iniciar streaming
func (m *appModel) startStreamCmd(userInput string) tea.Cmd {
	return func() tea.Msg {
		if m.config == nil {
			return errorMsg{fmt.Errorf("configuración no cargada")}
		}
		provider, ok := m.config.Providers[m.config.ActiveLLM]
		if !ok {
			return errorMsg{fmt.Errorf("proveedor de LLM activo no encontrado")}
		}
		// Guardar: Gemini y OpenAI soportados actualmente
		active := strings.ToLower(m.config.ActiveLLM)
		if active != "gemini" && active != "google-gemini" && active != "openai" {
			return errorMsg{fmt.Errorf("Proveedor '%s' no soportado aún en el cliente LLM. Soportados: Google Gemini y OpenAI. Usa F3 → MODELOS para activar uno de ellos.", m.config.ActiveLLM)}
		}
		modelName := provider.Model
		systemPrompt := m.agent.BuildSystemPrompt()
		client, iter, err := llm.StartStream(m.config.ActiveLLM, m.apiKey, modelName, systemPrompt, m.messages, userInput)
		if err != nil {
			return errorMsg{err}
		}
		m.streamClient = client
		m.streamIter = iter
		m.isStreaming = true
		m.streamBuffer.Reset()
		m.loadingBar = randomBar(15)
		m.streamStart = time.Now()
		// Frase inicial mientras piensa
		m.thinkingPhrase = randomThinkingPhrase()
		return m.readNextStreamChunkMsg()
	}
}

func (m *appModel) readNextStreamChunkMsg() tea.Msg {
	if m.streamIter == nil {
		return streamDoneMsg{}
	}
	chunk, err := m.streamIter.Next()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return streamDoneMsg{}
		}
		return errorMsg{err}
	}
	var b strings.Builder
	b.WriteString(chunk)
	return streamChunkMsg{chunk: b.String()}
}

func (m *appModel) preloadTickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return preloadTickMsg{text: randomAlphaNum(16)}
	})
}

// nextStreamChunkCmd moved to bottom of file to group with tick utilities

// sanitizeAgentText elimina bloques JSON de tool-calls del texto del agente.
func sanitizeAgentText(s string) string {
	// Remover bloques con fences ```json ... ```
	reFenceArray := regexp.MustCompile("(?s)```json\\s*\\[.*?\\]\\s*```")
	s = reFenceArray.ReplaceAllString(s, "")
	reFenceObject := regexp.MustCompile("(?s)```json\\s*\\{[\\s\\S]*?\\}")
	s = reFenceObject.ReplaceAllString(s, "")
	// Remover JSON plano que contenga la clave \"tool\"
	rePlainArray := regexp.MustCompile(`(?s)\[\s*\{[\s\S]*?"tool"[\s\S]*?\}\s*\]`)
	s = rePlainArray.ReplaceAllString(s, "")
	rePlainObject := regexp.MustCompile(`(?s)\{[\s\S]*?"tool"[\s\S]*?\}`)
	s = rePlainObject.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}

// executeAndLog es una función helper que ejecuta una herramienta y loguea el resultado.
func (m *appModel) executeAndLog(call tools.ToolCall) {
	result, err := tools.Execute(call)
	var resultMsg string
	if err != nil {
		resultMsg = fmt.Sprintf("Error al ejecutar la herramienta %s: %v", call.Name, err)
	} else {
		resultMsg = result
	}
	m.messages = append(m.messages, &genai.Content{
		Parts: []genai.Part{genai.Text(resultMsg)},
		Role:  "model",
	})
	m.updateViewport()
}

func (m *appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
		cmds  []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		headerHeight := lipgloss.Height(m.headerView())
		footerHeight := lipgloss.Height(m.footerView())

		mainContentHeight := m.height - headerHeight - footerHeight
		availableWidth := m.width - m.styles.AppStyle.GetHorizontalFrameSize()
		availableHeight := mainContentHeight - m.styles.AppStyle.GetVerticalFrameSize()

		dividerHeight := 1
		m.textarea.SetWidth(availableWidth)
		// Usar todo el ancho disponible para el viewport principal
		m.viewport.Width = availableWidth
		m.viewport.Height = availableHeight - dividerHeight - m.textarea.Height()
		// Clamp de seguridad para evitar alturas negativas
		if m.viewport.Height < 3 {
			m.viewport.Height = 3
		}

		// Ajustar viewports de menús (MODELOS/SESIONES) para scroll
		menuWidth := availableWidth - 2
		if menuWidth < 10 {
			menuWidth = availableWidth
		}
		menuContentWidth := menuWidth - m.scrollbarWidth
		if menuContentWidth < 10 {
			menuContentWidth = menuWidth
		}
		m.modelsViewport.Width = menuContentWidth
		m.sessionsViewport.Width = menuContentWidth
		menuHeight := availableHeight - 6
		if menuHeight < 3 {
			menuHeight = 3
		}
		m.modelsViewport.Height = menuHeight
		m.sessionsViewport.Height = menuHeight

		// Ajustar ancho de barra de progreso en header (compacta)
		pw := availableWidth / 8
		if pw < 10 {
			pw = 10
		}
		if pw > 20 {
			pw = 20
		}
		m.prog.Width = pw
		if !m.isReady {
			m.updateViewport()
			m.isReady = true
		}

	case completionMsg:
		content := msg.content
		jsonStr := content

		if start := strings.Index(content, "```json"); start != -1 {
			if end := strings.LastIndex(content, "```"); end > start {
				jsonStr = content[start+len("```json") : end]
				jsonStr = strings.TrimSpace(jsonStr)
			}
		}

		var rawToolCalls []map[string]interface{}
		err := json.Unmarshal([]byte(jsonStr), &rawToolCalls)
		if err != nil {
			var singleRawCall map[string]interface{}
			err2 := json.Unmarshal([]byte(jsonStr), &singleRawCall)
			if err2 == nil {
				rawToolCalls = []map[string]interface{}{singleRawCall}
				err = nil
			}
		}

		if err == nil && len(rawToolCalls) > 0 {
			for _, rawCall := range rawToolCalls {
				call := tools.ToolCall{}
				if name, ok := rawCall["tool"].(string); ok {
					call.Name = name
				} else if name, ok := rawCall["tool_name"].(string); ok {
					call.Name = name
				} else if name, ok := rawCall["tool_code"].(string); ok {
					call.Name = name
				}

				if args, ok := rawCall["kwargs"].(map[string]interface{}); ok {
					call.Arguments = args
				} else if args, ok := rawCall["parameters"].(map[string]interface{}); ok {
					call.Arguments = args
				} else {
					call.Arguments = rawCall
				}

				// La TUI decide si se necesita confirmación
				switch call.Name {
				case "file:write", "file:read":
					if m.allowedTools[call.Name] {
						m.executeAndLog(call)
					} else {
						m.pendingCall = &call
						m.isConfirming = true
						m.confirmPrompt = fmt.Sprintf("El agente quiere usar `%s`. Proceder?", call.Name)
						m.confirmationType = "S/N/A"
						return m, nil
					}
				case "bash:execute":
					m.pendingCall = &call
					m.isConfirming = true
					m.confirmPrompt = fmt.Sprintf("El agente quiere ejecutar: `%s`. Proceder?", call.Arguments["command"])
					m.confirmationType = "S/N"
					// Marcar si requiere doble confirmación
					if cmd, ok := call.Arguments["command"].(string); ok {
						m.awaitingDoubleConfirm = tools.IsCriticalCommand(cmd)
					} else {
						m.awaitingDoubleConfirm = false
					}
					return m, nil
				case "ask_user_confirmation":
					m.pendingCall = &call
					m.isConfirming = true
					prompt := "¿Confirmar acción del agente?"
					if p, ok := call.Arguments["prompt"].(string); ok && p != "" {
						prompt = p
					}
					m.confirmPrompt = prompt
					m.confirmationType = "S/N"
					return m, nil
				default:
					m.executeAndLog(call)
				}
			}
		} else {
			clean := sanitizeAgentText(msg.content)
			if strings.TrimSpace(clean) != "" {
				m.messages = append(m.messages, &genai.Content{Parts: []genai.Part{genai.Text(clean)}, Role: "model"})
			}
			m.updateViewport()
		}

	case streamChunkMsg:
		if msg.chunk != "" {
			m.streamBuffer.WriteString(msg.chunk)
			m.isPreloading = false
			m.preloadText = ""
			m.updateViewport()
		}
		return m, tea.Batch(m.nextStreamChunkCmd(), m.loadingTickCmd(), m.spin.Tick)

	case streamDoneMsg:
		if m.streamClient != nil {
			m.streamClient.Close()
		}
		finalText := m.streamBuffer.String()
		if finalText != "" {
			// Intentar parsear tool-calls como en completionMsg
			jsonStr := finalText
			if start := strings.Index(finalText, "```json"); start != -1 {
				if end := strings.LastIndex(finalText, "```"); end > start {
					jsonStr = finalText[start+len("```json") : end]
					jsonStr = strings.TrimSpace(jsonStr)
				}
			}
			var rawToolCalls []map[string]interface{}
			err := json.Unmarshal([]byte(jsonStr), &rawToolCalls)
			if err != nil {
				var singleRawCall map[string]interface{}
				err2 := json.Unmarshal([]byte(jsonStr), &singleRawCall)
				if err2 == nil {
					rawToolCalls = []map[string]interface{}{singleRawCall}
					err = nil
				}
			}
			if err == nil && len(rawToolCalls) > 0 {
				for _, rawCall := range rawToolCalls {
					call := tools.ToolCall{}
					if name, ok := rawCall["tool"].(string); ok {
						call.Name = name
					} else if name, ok := rawCall["tool_name"].(string); ok {
						call.Name = name
					} else if name, ok := rawCall["tool_code"].(string); ok {
						call.Name = name
					}
					if args, ok := rawCall["kwargs"].(map[string]interface{}); ok {
						call.Arguments = args
					} else if args, ok := rawCall["parameters"].(map[string]interface{}); ok {
						call.Arguments = args
					} else {
						call.Arguments = rawCall
					}
					switch call.Name {
					case "file:write", "file:read":
						if m.allowedTools[call.Name] {
							m.executeAndLog(call)
						} else {
							m.pendingCall = &call
							m.isConfirming = true
							m.confirmPrompt = fmt.Sprintf("El agente quiere usar `%s`. Proceder?", call.Name)
							m.confirmationType = "S/N/A"
							m.updateViewport()
							return m, nil
						}
					case "bash:execute":
						m.pendingCall = &call
						m.isConfirming = true
						m.confirmPrompt = fmt.Sprintf("El agente quiere ejecutar: `%s`. Proceder?", call.Arguments["command"])
						m.confirmationType = "S/N"
						// Marcar si requiere doble confirmación
						if cmd, ok := call.Arguments["command"].(string); ok {
							m.awaitingDoubleConfirm = tools.IsCriticalCommand(cmd)
						} else {
							m.awaitingDoubleConfirm = false
						}
						m.updateViewport()
						return m, nil
					case "ask_user_confirmation":
						m.pendingCall = &call
						m.isConfirming = true
						prompt := "¿Confirmar acción del agente?"
						if p, ok := call.Arguments["prompt"].(string); ok && p != "" {
							prompt = p
						}
						m.confirmPrompt = prompt
						m.confirmationType = "S/N"
						m.updateViewport()
						return m, nil
					default:
						m.executeAndLog(call)
					}
				}
			} else {
				clean := sanitizeAgentText(finalText)
				if strings.TrimSpace(clean) != "" {
					m.messages = append(m.messages, &genai.Content{Parts: []genai.Part{genai.Text(clean)}, Role: "model"})
				}
			}
		}
		// Registrar metadatos para chips
		m.lastResponseDuration = time.Since(m.streamStart)
		if m.config != nil {
			if p, ok := m.config.Providers[m.config.ActiveLLM]; ok {
				m.lastResponseProvider = p.Name
				m.lastResponseModel = p.Model
			}
		}
		// Aproximación simple de tokens
		runes := []rune(finalText)
		m.lastResponseTokenApprox = len(runes) / 4
		m.isStreaming = false
		m.streamIter = nil
		m.loadingBar = ""
		m.isPreloading = false
		m.preloadText = ""
		m.thinkingPhrase = ""
		// Iniciar animación de chips al finalizar streaming
		m.chipAnimOffset = 10
		// Animar progreso al 100%
		pcmd := m.prog.SetPercent(1.0)
		m.updateViewport()
		return m, tea.Batch(m.chipAnimTickCmd(), pcmd)

	case thinkingTickMsg:
		// Actualizar frase solo durante el pensamiento (precarga)
		if m.isPreloading {
			m.thinkingPhrase = msg.phrase
			m.updateViewport()
			return m, m.thinkingTickCmd()
		}
		return m, nil

	case preloadTickMsg:
		if m.isPreloading {
			m.preloadText = msg.text
			m.updateViewport()
			return m, m.preloadTickCmd()
		}
		return m, nil

	case loadingTickMsg:
		m.loadingBar = msg.frame
		if m.isStreaming {
			var pcmd tea.Cmd
			if m.prog.Percent() >= 0.95 {
				pcmd = m.prog.SetPercent(0)
			} else {
				pcmd = m.prog.IncrPercent(0.06)
			}
			m.updateViewport()
			return m, tea.Batch(m.loadingTickCmd(), pcmd, m.spin.Tick)
		}
		return m, nil

	case chipAnimTickMsg:
		if m.chipAnimOffset > 0 {
			m.chipAnimOffset--
			m.updateViewport()
			return m, m.chipAnimTickCmd()
		}
		return m, nil

	// Parpadeo sutil del kaomoji
	case kaomojiBlinkMsg:
		m.currentKao = m.buildKao(false)
		return m, m.kaomojiResetTickCmd()
	case kaomojiResetMsg:
		m.currentKao = m.buildKao(true)
		// Si está marcado un doble parpadeo, disparamos un segundo parpadeo pronto.
		if m.kaoDoDouble {
			m.kaoDoDouble = false
			return m, m.kaomojiSecondBlinkTickCmd()
		}
		return m, m.kaomojiIdleTickCmd()

	case errorMsg:
		m.err = msg
		// Si detectamos cuota o 429, abrir menú MODELOS con sugerencia
		errStr := strings.ToLower(msg.err.Error())
		if strings.Contains(errStr, "429") || strings.Contains(errStr, "quota") {
			m.showModels = true
			m.modelsLastOutput = "Cuota de API excedida. Usa 'activate <proveedor> <modelo>' o 'setkey <proveedor> <clave>' aquí."
		}
		return m, nil

	case tea.KeyMsg:
		// Si está abierta la ayuda, solo permite cerrar con F1 o salir con Quit
		if m.showHelp {
			if key.Matches(msg, m.keyMap.Help) {
				m.showHelp = false
				return m, nil
			}
			if key.Matches(msg, m.keyMap.Quit) {
				return m, tea.Quit
			}
			return m, nil
		}

		// Toggle menú MODELOS (F3)
		if key.Matches(msg, m.keyMap.Models) {
			m.showModels = !m.showModels
			if m.showModels {
				m.textarea.Blur()
			} else {
				m.modelsCaptureInput = false
				m.modelsInput.Reset()
				m.textarea.Focus()
			}
			m.updateViewport()
			return m, nil
		}
		// Toggle menú SESIONES (F4)
		if key.Matches(msg, m.keyMap.Sessions) {
			m.showSessions = !m.showSessions
			if m.showSessions {
				m.textarea.Blur()
				if names, err := session.ListSessions(); err == nil {
					m.sessions = names
					if m.sessionsSelectedIdx >= len(m.sessions) {
						m.sessionsSelectedIdx = 0
					}
				}
			} else {
				m.textarea.Focus()
			}
			m.updateViewport()
			return m, nil
		}

		// Confirmaciones de herramientas
		if m.isConfirming {
			switch strings.ToLower(msg.String()) {
			case "s", "y":
				// Manejar doble confirmación para bash:execute
				if m.pendingCall != nil && m.pendingCall.Name == "bash:execute" && m.awaitingDoubleConfirm {
					m.confirmPrompt = fmt.Sprintf("Comando crítico detectado: `%s`. Confirmar otra vez?", m.pendingCall.Arguments["command"])
					m.awaitingDoubleConfirm = false
					m.isConfirming = true
					m.updateViewport()
					return m, nil
				}
				m.isConfirming = false
				if m.pendingCall != nil {
					if m.pendingCall.Name == "ask_user_confirmation" {
						m.messages = append(m.messages, &genai.Content{Parts: []genai.Part{genai.Text("CONFIRMATION: yes")}, Role: "model"})
						m.updateViewport()
						m.pendingCall = nil
					} else {
						m.executeAndLog(*m.pendingCall)
						m.pendingCall = nil
					}
				}
			case "n":
				m.isConfirming = false
				if m.pendingCall != nil && m.pendingCall.Name == "ask_user_confirmation" {
					m.messages = append(m.messages, &genai.Content{Parts: []genai.Part{genai.Text("CONFIRMATION: no")}, Role: "model"})
					m.updateViewport()
					m.pendingCall = nil
				} else {
					m.pendingCall = nil
					m.messages = append(m.messages, &genai.Content{Parts: []genai.Part{genai.Text("Acción cancelada por el usuario.")}, Role: "model"})
					m.updateViewport()
				}
			case "a":
				if m.confirmationType == "S/N/A" {
					m.isConfirming = false
					if m.pendingCall != nil {
						m.allowedTools[m.pendingCall.Name] = true
						m.executeAndLog(*m.pendingCall)
						m.pendingCall = nil
					}
				}
			}
			return m, nil
		}

		// Si algún menú está activo, capturamos las teclas aquí para evitar que el chat reciba input
		if m.showModels {
			// Cerrar menú con Esc
			if msg.Type == tea.KeyEsc {
				m.modelsCaptureInput = false
				m.modelsInput.Reset()
				m.showModels = false
				m.textarea.Focus()
				m.updateViewport()
				return m, nil
			}
			// Captura de API key para el modelo seleccionado
			if m.modelsCaptureInput {
				var miCmd tea.Cmd
				if msg.Type == tea.KeyEnter {
					if len(m.modelsFlat) == 0 || m.modelsSelectedIdx < 0 || m.modelsSelectedIdx >= len(m.modelsFlat) {
						m.modelsCaptureInput = false
						m.updateViewport()
						return m, nil
					}
					sel := m.modelsFlat[m.modelsSelectedIdx]
					keyVal := strings.TrimSpace(m.modelsInput.Value())
					if keyVal == "" {
						m.modelsLastOutput = "Clave vacía; ingresa la API key."
						m.updateViewport()
						return m, nil
					}
					out, err := m.cmdSetKey(sel.ProviderID, keyVal, "")
					if err != nil {
						m.modelsLastOutput = fmt.Sprintf("Error guardando clave: %v", err)
					} else {
						if err2 := m.cmdActivate(sel.ProviderID, sel.ModelID); err2 != nil {
							m.modelsLastOutput = fmt.Sprintf("Clave actualizada, pero error activando: %v", err2)
						} else {
							m.modelsLastOutput = fmt.Sprintf("%s activado: %s / %s. %s", strings.Title(sel.ProviderID), sel.ProviderName, sel.ModelID, out)
						}
					}
					m.modelsCaptureInput = false
					m.modelsInput.Reset()
					m.updateViewport()
					return m, nil
				}
				m.modelsInput, miCmd = m.modelsInput.Update(msg)
				return m, miCmd
			}
			// Navegación y activación
			switch msg.Type {
			case tea.KeyUp:
				if m.modelsSelectedIdx > 0 { m.modelsSelectedIdx-- }
				m.ensureModelsSelectionVisible()
				m.updateViewport(); return m, nil
			case tea.KeyDown:
				if m.modelsSelectedIdx < len(m.modelsFlat)-1 { m.modelsSelectedIdx++ }
				m.ensureModelsSelectionVisible()
				m.updateViewport(); return m, nil
			case tea.KeyEnter:
				if len(m.modelsFlat) == 0 { return m, nil }
				sel := m.modelsFlat[m.modelsSelectedIdx]
				env := sel.EnvVar
				if env == "" {
					if p, ok := m.config.Providers[sel.ProviderID]; ok && p.APIKeyEnv != "" {
						env = p.APIKeyEnv
					} else {
						env = envVarForProvider(sel.ProviderID)
					}
				}
				keyPresent := false
				if env != "" {
					if v := os.Getenv(env); v != "" { keyPresent = true } else {
						if v2, err := config.LoadEnvVar(env); err == nil && v2 != "" { keyPresent = true }
					}
				}
				if keyPresent {
					if err := m.cmdActivate(sel.ProviderID, sel.ModelID); err != nil {
						m.modelsLastOutput = fmt.Sprintf("Error activando: %v", err)
					} else {
						m.modelsLastOutput = fmt.Sprintf("Activado %s / %s", sel.ProviderName, sel.ModelID)
					}
					m.updateViewport(); return m, nil
				}
				// pedir clave
				m.modelsCaptureInput = true
				m.modelsInput.Placeholder = "Escribe API key y presiona Enter"
				m.modelsInput.Focus(); m.updateViewport(); return m, nil
			}
			return m, nil
		}

		if m.showSessions {
			// Cerrar menú con Esc
			if msg.Type == tea.KeyEsc {
				m.sessionsCaptureInput = false
				m.sessionsInput.Reset()
				m.showSessions = false
				m.textarea.Focus()
				m.updateViewport()
				return m, nil
			}
			// Modo edición: renombrar o borrar
			if m.sessionsCaptureInput {
				var siCmd tea.Cmd
				if msg.Type == tea.KeyEnter {
					if len(m.sessions) == 0 { m.sessionsCaptureInput = false; m.updateViewport(); return m, nil }
					name := m.sessions[m.sessionsSelectedIdx]
					v := strings.TrimSpace(m.sessionsInput.Value())
					if strings.EqualFold(v, "del") || strings.EqualFold(v, "delete") || strings.EqualFold(v, "rm") {
						if err := session.DeleteSession(name); err != nil {
							m.messages = append(m.messages, &genai.Content{Parts: []genai.Part{genai.Text("Error al borrar sesión: "+err.Error())}, Role: "model"})
						} else {
							m.messages = append(m.messages, &genai.Content{Parts: []genai.Part{genai.Text("Sesión eliminada: "+name)}, Role: "model"})
							if names, err := session.ListSessions(); err == nil { m.sessions = names } else { m.sessions = nil }
							if m.sessionsSelectedIdx >= len(m.sessions) { m.sessionsSelectedIdx = 0 }
						}
						m.sessionsCaptureInput = false
						m.sessionsInput.Reset()
						m.updateViewport(); return m, nil
					}
					if v == "" {
						m.messages = append(m.messages, &genai.Content{Parts: []genai.Part{genai.Text("Nombre vacío; escribe uno nuevo o 'DEL'.")}, Role: "model"})
						m.updateViewport(); return m, nil
					}
					if err := session.RenameSession(name, v); err != nil {
						m.messages = append(m.messages, &genai.Content{Parts: []genai.Part{genai.Text("Error al renombrar: "+err.Error())}, Role: "model"})
					} else {
						m.messages = append(m.messages, &genai.Content{Parts: []genai.Part{genai.Text(fmt.Sprintf("Sesión '%s' renombrada a '%s'.", name, v))}, Role: "model"})
						if names, err := session.ListSessions(); err == nil { m.sessions = names } else { m.sessions = nil }
						for i, nm := range m.sessions { if nm == v { m.sessionsSelectedIdx = i; break } }
					}
					m.sessionsCaptureInput = false
					m.sessionsInput.Reset()
					m.updateViewport(); return m, nil
				}
				m.sessionsInput, siCmd = m.sessionsInput.Update(msg)
				return m, siCmd
			}
			// Navegación / carga
			switch msg.Type {
			case tea.KeyUp:
				if m.sessionsSelectedIdx > 0 { m.sessionsSelectedIdx-- }
				m.ensureSessionsSelectionVisible()
				m.updateViewport(); return m, nil
			case tea.KeyDown:
				if m.sessionsSelectedIdx < len(m.sessions)-1 { m.sessionsSelectedIdx++ }
				m.ensureSessionsSelectionVisible()
				m.updateViewport(); return m, nil
			case tea.KeyEnter:
				if len(m.sessions) == 0 { return m, nil }
				name := m.sessions[m.sessionsSelectedIdx]
				loaded, err := session.LoadSession(name)
				if err != nil {
					m.messages = append(m.messages, &genai.Content{Parts: []genai.Part{genai.Text("Error al cargar sesión: "+err.Error())}, Role: "model"})
					m.updateViewport(); return m, nil
				}
				m.messages = loaded
				m.sessionName = name
				m.showSessions = false
				m.textarea.Focus()
				m.updateViewport(); return m, nil
			default:
				// Atajo: tecla 'e' para editar (renombrar/borrar)
				if strings.ToLower(msg.String()) == "e" {
					m.sessionsCaptureInput = true
					m.sessionsInput.Placeholder = "Renombrar o escribir 'DEL' para borrar"
					m.sessionsInput.Focus(); m.updateViewport(); return m, nil
				}
			}
			return m, nil
		}

		switch {
		case key.Matches(msg, m.keyMap.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keyMap.Save):
			session.SaveSession(m.sessionName, m.messages)
			m.messages = append(m.messages, &genai.Content{Parts: []genai.Part{genai.Text("Sesión guardada.")}, Role: "model"})
			m.updateViewport()
			return m, nil
		case key.Matches(msg, m.keyMap.Help):
			m.showHelp = !m.showHelp
			return m, nil
		case key.Matches(msg, m.keyMap.Theme):
			m.cycleAccentTheme()
			m.updateViewport()
			return m, nil
		case msg.Type == tea.KeyEnter:
			userInput := m.textarea.Value()
			trimmed := strings.TrimSpace(userInput)
			if strings.HasPrefix(trimmed, "/help") {
				m.showHelp = !m.showHelp
				m.textarea.Reset()
				return m, nil
			}
			m.messages = append(m.messages, &genai.Content{Parts: []genai.Part{genai.Text(userInput)}, Role: "user"})
			if m.isSplashVisible { m.isSplashVisible = false }
			m.updateViewport()
			m.textarea.Reset()
			m.isPreloading = true
			m.preloadText = randomAlphaNum(16)
			return m, tea.Batch(m.startStreamCmd(userInput), m.preloadTickCmd(), m.loadingTickCmd(), m.spin.Tick, m.thinkingTickCmd(), m.prog.SetPercent(0.0))
		}
	}

	m.textarea, tiCmd = m.textarea.Update(msg)
	m.viewport, vpCmd = m.viewport.Update(msg)
	// Permitir scroll con mouse en menús cuando estén visibles
	var mvCmd tea.Cmd
	var svCmd tea.Cmd
	if m.showModels {
		m.modelsViewport, mvCmd = m.modelsViewport.Update(msg)
	}
	if m.showSessions {
		m.sessionsViewport, svCmd = m.sessionsViewport.Update(msg)
	}
	var spCmd tea.Cmd
	m.spin, spCmd = m.spin.Update(msg)
	// Ajuste: progress.Update devuelve tea.Model, hacemos type assert
	var prCmd tea.Cmd
	var progModel tea.Model
	progModel, prCmd = m.prog.Update(msg)
	if pm, ok := progModel.(progress.Model); ok {
		m.prog = pm
	}
	cmds = append(cmds, tiCmd, vpCmd, mvCmd, svCmd, spCmd, prCmd)

	return m, tea.Batch(cmds...)
}

func (m *appModel) updateViewport() {
	var content strings.Builder
	for i, msg := range m.messages {
		role := "🤖 Agente"
		style := m.styles.AgentRole
		if msg.Role == "user" {
			role = "👤 Usuario"
			style = m.styles.UserRole
		}
		if len(msg.Parts) > 0 {
			if txt, ok := msg.Parts[0].(genai.Text); ok {
				formattedRole := style.Render(role)

				textToRender := string(txt)
				if msg.Role != "user" {
					textToRender = sanitizeAgentText(textToRender)
					if strings.TrimSpace(textToRender) == "" {
						// Nada que mostrar tras sanitizar
						continue
					}
				}

				renderer, _ := glamour.NewTermRenderer(
					glamour.WithEnvironmentConfig(),
					glamour.WithAutoStyle(),
					glamour.WithWordWrap(m.viewport.Width),
				)
				rendered, err := renderer.Render(textToRender)
				if err != nil {
					rendered = lipgloss.NewStyle().Width(m.viewport.Width).Render(textToRender)
				}

				content.WriteString(fmt.Sprintf("%s:\n%s", formattedRole, rendered))
				if i < len(m.messages)-1 {
					content.WriteString("\n\n---\n\n")
				}
			}
		}
	}
	// Mostrar precarga alfanumérica (antes de responder)
	if m.isPreloading {
		if content.Len() > 0 {
			content.WriteString("\n\n---\n\n")
		}
		phrase := m.styles.ThinkingStyle.Render(m.thinkingPhrase)
		line := fmt.Sprintf("%s %s", m.spin.View(), phrase)
		content.WriteString(line + "\n")
		pre := m.styles.LoadingStyle.Render(m.preloadText)
		content.WriteString(pre)
	}
	// Añadir buffer de streaming en vivo
	if m.isStreaming && m.streamBuffer.Len() > 0 {
		formattedRole := m.styles.AgentRole.Render("🤖 Agente")
		raw := m.streamBuffer.String()
		clean := sanitizeAgentText(raw)
		if strings.TrimSpace(clean) != "" {
			renderer, _ := glamour.NewTermRenderer(
				glamour.WithEnvironmentConfig(),
				glamour.WithAutoStyle(),
				glamour.WithWordWrap(m.viewport.Width),
			)
			rendered, err := renderer.Render(clean)
			if err != nil {
				rendered = lipgloss.NewStyle().Width(m.viewport.Width).Render(clean)
			}
			if content.Len() > 0 {
				content.WriteString("\n\n---\n\n")
			}
			content.WriteString(fmt.Sprintf("%s:\n%s", formattedRole, rendered))
		}
	}
	// Nota: La barra de progreso se mueve al footer para evitar duplicación
	// Chips de estado (al final) cuando no está streameando
	if !m.isStreaming && m.lastResponseDuration > 0 {
		mins := int(m.lastResponseDuration.Minutes())
		secs := int(m.lastResponseDuration.Seconds()) % 60
		chips := []string{
			fmt.Sprintf("⏱ %02d:%02d", mins, secs),
		}
		// Mostrar solo el modelo (sin proveedor) para evitar duplicaciones visuales
		if m.lastResponseModel != "" {
			chips = append(chips, fmt.Sprintf("⚙️ %s", m.lastResponseModel))
		}
		if m.lastResponseTokenApprox > 0 {
			chips = append(chips, fmt.Sprintf("🔤 ~%d tok", m.lastResponseTokenApprox))
		}
		chipLine := m.styles.ChipStyle.Render(strings.Join(chips, "  "))
		animated := lipgloss.NewStyle().MarginLeft(m.chipAnimOffset).Render(chipLine)
		if content.Len() > 0 {
			content.WriteString("\n\n")
		}
		content.WriteString(animated)
	}
	m.viewport.SetContent(content.String())
	m.viewportContentLineCount = strings.Count(content.String(), "\n") + 1
	if m.viewport.Height > 0 {
		m.viewport.GotoBottom()
	}
}

func (m *appModel) headerView() string {
	if m.isSplashVisible {
		logoStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
		return lipgloss.JoinVertical(lipgloss.Center, logoStyle.Render(logo), "\nBienvenido a SikmaCode - una nueva experiencia de IA")
	}

	// Kaomoji a la izquierda; a la derecha: ID, LLM (solo modelo), hora y progreso
	kao := m.currentKao
	leftSide := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render(kao)
	session := m.sessionName
	llm := "—"
	if m.config != nil {
		llm = m.config.ActiveLLM
		if llm == "" {
			if p, ok := m.config.Providers[m.config.ActiveLLM]; ok {
				llm = p.Name
			}
		}
	}
	if strings.Contains(llm, "/") {
		parts := strings.Split(llm, "/")
		llm = parts[len(parts)-1]
	}
	clock := time.Now().Format("15:04:05")
	rightText := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render(
		fmt.Sprintf("%s | %s | %s ", session, llm, clock),
	)
	prog := m.prog.View()
	rightSide := lipgloss.JoinHorizontal(lipgloss.Top, rightText, prog)

	leftWidth := lipgloss.Width(leftSide)
	rightWidth := lipgloss.Width(rightSide)
	totalWidth := m.viewport.Width

	spacerWidth := totalWidth - leftWidth - rightWidth
	if spacerWidth < 0 {
		spacerWidth = 0
	}
	spacer := strings.Repeat(" ", spacerWidth)

	return lipgloss.JoinHorizontal(lipgloss.Top, leftSide, spacer, rightSide)
}

func (m *appModel) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v", m.err)
	}
	if !m.isReady {
		return "Inicializando..."
	}

	if m.isConfirming {
		var options string
		if m.confirmationType == "S/N/A" {
			options = "(s/n/a)"
		} else {
			options = "(s/n)"
		}
		dialogBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("228")).
			Padding(1, 2).
			Render(m.confirmPrompt + "\n\n" + options)

		return lipgloss.Place(
			m.width,
			m.height,
			lipgloss.Center,
			lipgloss.Center,
			dialogBox,
		)
	}

	if m.showHelp {
		content := strings.Join([]string{
			"Ayuda rápida",
			"",
			"- Enter: enviar mensaje",
			"- Ctrl+S: guardar sesión",
			"- Ctrl+C / Esc: salir",
			"- F1: mostrar/ocultar esta ayuda",
			"- /help: alterna ayuda desde el prompt",
			"- F2: alternar tema",
			"- F3: abrir menú MODELOS",
			"- F4: abrir menú SESIONES",
			"",
			"Confirmaciones:",
			"- s/y: sí",
			"- n: no",
			"- a: siempre permitir (solo herramientas seguras)",
			"- Nota: comandos críticos requieren doble confirmación",
			"",
			"Estado:",
			"- Header: kaomoji a la izquierda; derecha: ID, LLM, hora y progreso",
		}, "\n")
		dialogBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("63")).
			Padding(1, 2).
			Render(content)
		return lipgloss.Place(
			m.width,
			m.height,
			lipgloss.Center,
			lipgloss.Center,
			dialogBox,
		)
	}

	// Menú MODELOS (F3)
	if m.showModels {
		dialogBox := m.styles.AppStyle.Render(m.modelsMenuView())

		return lipgloss.Place(
			m.width,
			m.height,
			lipgloss.Center,
			lipgloss.Center,
			dialogBox,
		)
	}

	// Menú SESIONES (F4)
	if m.showSessions {
		dialogBox := m.styles.AppStyle.Render(m.sessionsMenuView())

		return lipgloss.Place(
			m.width,
			m.height,
			lipgloss.Center,
			lipgloss.Center,
			dialogBox,
		)
	}

	divider := m.styles.DividerStyle.Render(strings.Repeat("─", m.viewport.Width))

	// Contenido que va dentro del borde
	innerContent := lipgloss.JoinVertical(
		lipgloss.Left,
		m.viewport.View(),
		divider,
		m.textarea.View(),
	)
	borderedContent := m.styles.AppStyle.Render(innerContent)

	// Ensamblaje final
	return lipgloss.JoinVertical(
		lipgloss.Left,
		m.headerView(),
		borderedContent,
		m.footerView(),
	)
}

func (m *appModel) thinkingTickCmd() tea.Cmd {
	return tea.Tick(4000*time.Millisecond, func(t time.Time) tea.Msg {
		return thinkingTickMsg{phrase: randomThinkingPhrase()}
	})
}

func (m *appModel) chipAnimTickCmd() tea.Cmd {
	return tea.Tick(50*time.Millisecond, func(t time.Time) tea.Msg { return chipAnimTickMsg{} })
}

func (m *appModel) footerView() string {
	help := "[Ctrl+S guardar] [Ctrl+C/Esc salir] [Ayuda: F1] [F2 tema] [F3 MODELOS] [F4 SESIONES]"
	return m.styles.FooterStyle.Render(help)
}


// Construye el kaomoji con ojos abiertos/cerrados sin cambiar ancho
func (m *appModel) buildKao(open bool) string {
	if open {
		return "≧◉◡◉≦"
	}
	// Variante de ojos cerrados, manteniendo longitud de 5 runas
	return "≧˘◡˘≦"
}


func (m *appModel) kaomojiIdleTickCmd() tea.Cmd {
	// Periodo más largo y natural: 6–11s con jitter
	dur := time.Duration(6000+rand.Intn(5000)) * time.Millisecond
	return tea.Tick(dur, func(t time.Time) tea.Msg {
		// Probabilidad baja de doble parpadeo ocasional (1 de cada 10 aprox.)
		m.kaoDoDouble = rand.Intn(10) == 0
		return kaomojiBlinkMsg{}
	})
}

func (m *appModel) kaomojiResetTickCmd() tea.Cmd {
	// Cierre más lento y suave: 180–300ms
	dur := time.Duration(180+rand.Intn(120)) * time.Millisecond
	return tea.Tick(dur, func(t time.Time) tea.Msg { return kaomojiResetMsg{} })
}

// Segundo parpadeo rápido para el doble parpadeo ocasional
func (m *appModel) kaomojiSecondBlinkTickCmd() tea.Cmd {
	// Pausa breve antes del segundo parpadeo: 220–320ms
	dur := time.Duration(220+rand.Intn(100)) * time.Millisecond
	return tea.Tick(dur, func(t time.Time) tea.Msg { return kaomojiBlinkMsg{} })
}

func (m *appModel) nextStreamChunkCmd() tea.Cmd {
	return func() tea.Msg { return m.readNextStreamChunkMsg() }
}

func (m *appModel) loadingTickCmd() tea.Cmd {
	return tea.Tick(200*time.Millisecond, func(t time.Time) tea.Msg {
		return loadingTickMsg{frame: randomBar(15)}
	})
}

func randomBar(n int) string {
	chars := []rune{'█', '▓', '▒', '░', '─', '━', '╌', '╍', '▌', '▐', '▍', '▎', '▏'}
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteRune(chars[rand.Intn(len(chars))])
	}
	return b.String()
}

func randomAlphaNum(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteByte(letters[rand.Intn(len(letters))])
	}
	return b.String()
}

func randomThinkingPhrase() string {
	phrases := []string{
		"Desplegando mi arsenal tecnológico",
		"Enrutando datos para la mejor respuesta",
		"Activando mi modo desarrollador",
		"Compilando ideas en tiempo real",
		"Sintonizando con el problema",
		"Buscando patrones óptimos",
		"Analizando contexto y dependencias",
		"Refinando lógica de solución",
		"Alineando argumentos y ejemplos",
		"Calculando caminos eficientes",
		"Cargando herramientas de análisis",
		"Indexando conocimiento relevante",
		"Verificando supuestos",
		"Preparando estrategia de respuesta",
		"Trazando el flujo óptimo",
		"Modelando casos edge",
		"Puliendo claridad y precisión",
		"Validando coherencia técnica",
		"Ajustando tono y formato",
		"Estructurando pasos accionables",
		"Curando referencias útiles",
		"Midiendo impacto y riesgos",
		"Seleccionando mejores prácticas",
		"Sincronizando estilo y contenido",
		"Afinando detalles finales",
		"Checklist de calidad listo",
	}
	return phrases[rand.Intn(len(phrases))]
}

type preloadTickMsg struct{ text string }

func (m *appModel) cycleAccentTheme() {
	palette := []string{"178", "81", "135", "201", "86"}
	m.themeAccentIdx = (m.themeAccentIdx + 1) % len(palette)
	color := lipgloss.Color(palette[m.themeAccentIdx])
	m.styles.AgentRole = m.styles.AgentRole.Foreground(color).Bold(true)
	m.styles.ThinkingStyle = m.styles.ThinkingStyle.Foreground(color).Bold(true)
	m.styles.AppStyle = m.styles.AppStyle.BorderForeground(color)
	m.spin.Style = lipgloss.NewStyle().Foreground(color)
}


// ==== MODELOS: catálogo y comandos ====

type catalogProvider struct {
	ID     string
	Name   string
	EnvVar string
	Models []string
}

type providerCatalog struct {
	Providers map[string]catalogProvider
}

// Entrada aplanada de modelo para el menú F3
type modelEntry struct {
	ProviderID   string
	ProviderName string
	EnvVar       string
	ModelID      string
}

func (m *appModel) modelsMenuView() string {
	activeProv := "(ninguno)"
	activeModel := "(sin modelo)"
	if m.config != nil {
		if p, ok := m.config.Providers[m.config.ActiveLLM]; ok {
			activeProv = m.config.ActiveLLM
			activeModel = p.Model
		}
	}

	header := m.styles.ThinkingStyle.Copy().Bold(true).Render("MENÚ: MODELOS")
	info := fmt.Sprintf("Proveedor activo: %s\nModelo activo: %s\n", activeProv, activeModel)

	listTitle := m.styles.ChipStyle.Copy().Bold(true).Render("Modelos disponibles (↑/↓ para seleccionar, Enter para activar):")
	var listLines []string
	if len(m.modelsFlat) == 0 {
		listLines = []string{m.styles.LoadingStyle.Render("No hay catálogo de modelos disponible.")}
	} else {
		prevProv := ""
		for i, e := range m.modelsFlat {
			if prevProv != "" && e.ProviderName != prevProv {
				listLines = append(listLines, m.styles.DividerStyle.Render(strings.Repeat("─", 16)))
			}
			prevProv = e.ProviderName

			marker := "  "
			style := lipgloss.NewStyle()
			if i == m.modelsSelectedIdx {
				marker = "> "
				style = m.styles.ThinkingStyle.Copy().Bold(true)
			}
			activeTag := ""
			if m.config != nil {
				if p, ok := m.config.Providers[m.config.ActiveLLM]; ok {
					if m.config.ActiveLLM == e.ProviderID && p.Model == e.ModelID {
						activeTag = " (activo)"
					}
				}
			}
			line := fmt.Sprintf("%s / %s%s", e.ProviderName, e.ModelID, activeTag)
			listLines = append(listLines, style.Render(marker+line))
		}
	}

	var inputBlock string
	if m.modelsCaptureInput && len(m.modelsFlat) > 0 && m.modelsSelectedIdx >= 0 && m.modelsSelectedIdx < len(m.modelsFlat) {
		selected := m.modelsFlat[m.modelsSelectedIdx]
		prompt := m.styles.DividerStyle.Render(fmt.Sprintf("API key para %s", selected.ProviderName))
		inputView := m.styles.InputField.Render(m.modelsInput.View())
		inputBlock = strings.Join([]string{prompt, inputView}, "\n")
	}

	last := m.modelsLastOutput
	if last == "" {
		last = "Selecciona un modelo y presiona Enter. Si falta la API key, se solicitará."
	}

	m.modelsViewport.SetContent(strings.Join(listLines, "\n"))
	menuScrollbar := m.renderScrollbarForViewport(m.modelsViewport, len(listLines))

	return strings.Join([]string{
		header,
		info,
		listTitle,
		lipgloss.JoinHorizontal(lipgloss.Top, m.modelsViewport.View(), menuScrollbar),
		"",
		inputBlock,
		"",
		m.styles.LoadingStyle.Render(last),
	}, "\n")
}

func (m *appModel) sessionsMenuView() string {
	header := m.styles.ThinkingStyle.Copy().Bold(true).Render("MENÚ: SESIONES")
	instr := m.styles.ChipStyle.Render("↑/↓ para seleccionar, Enter para cargar, E para renombrar/borrar, Esc para cerrar")
	var listLines []string
	if len(m.sessions) == 0 {
		listLines = []string{m.styles.LoadingStyle.Render("No hay sesiones guardadas.")}
	} else {
		for i, name := range m.sessions {
			marker := "  "
			style := lipgloss.NewStyle()
			if i == m.sessionsSelectedIdx {
				marker = "> "
				style = m.styles.ThinkingStyle.Copy().Bold(true)
			}
			listLines = append(listLines, style.Render(marker+name))
		}
	}
	var inputBlock string
	if m.sessionsCaptureInput && len(m.sessions) > 0 {
		prompt := m.styles.DividerStyle.Render("Nuevo nombre para la sesión o escribe 'DEL' para borrar")
		inputBlock = strings.Join([]string{prompt, m.styles.InputField.Render(m.sessionsInput.View())}, "\n")
	}
	m.sessionsViewport.SetContent(strings.Join(listLines, "\n"))
	menuScrollbar := m.renderScrollbarForViewport(m.sessionsViewport, len(listLines))
	return strings.Join([]string{
		header,
		instr,
		lipgloss.JoinHorizontal(lipgloss.Top, m.sessionsViewport.View(), menuScrollbar),
		"",
		inputBlock,
	}, "\n")
}

// Asegurar que el ítem seleccionado del menú MODELOS esté visible en el viewport
func (m *appModel) ensureModelsSelectionVisible() {
	if m.modelsViewport.Height <= 0 {
		return
	}
	top := m.modelsViewport.YOffset
	bottom := top + m.modelsViewport.Height - 1
	idx := m.modelsSelectedIdx
	if idx < top {
		m.modelsViewport.SetYOffset(idx)
	} else if idx > bottom {
		m.modelsViewport.SetYOffset(idx - m.modelsViewport.Height + 1)
	}
}

// Asegurar que el ítem seleccionado del menú SESIONES esté visible en el viewport
func (m *appModel) ensureSessionsSelectionVisible() {
	if m.sessionsViewport.Height <= 0 {
		return
	}
	top := m.sessionsViewport.YOffset
	bottom := top + m.sessionsViewport.Height - 1
	idx := m.sessionsSelectedIdx
	if idx < top {
		m.sessionsViewport.SetYOffset(idx)
	} else if idx > bottom {
		m.sessionsViewport.SetYOffset(idx - m.sessionsViewport.Height + 1)
	}
}

// Renderiza una barra de scroll vertical para un viewport dado, usando total de líneas aproximado.
func (m *appModel) renderScrollbarForViewport(v viewport.Model, totalLines int) string {
	h := v.Height
	if h <= 0 {
		return ""
	}
	// Siempre reservamos columna, aunque no haya overflow
	visible := h
	if totalLines <= 0 {
		totalLines = visible
	}
	// Tamaño del pulgar proporcional
	thumbSize := int(float64(visible) / float64(totalLines) * float64(h))
	if thumbSize < 1 {
		thumbSize = 1
	}
	if thumbSize > h {
		thumbSize = h
	}
	// Posición del pulgar basada en offset
	maxTop := h - thumbSize
	thumbTop := 0
	if totalLines > visible {
		// Evitar división por cero
		maxOffset := totalLines - visible
		if maxOffset < 1 {
			maxOffset = 1
		}
		thumbTop = int(float64(v.YOffset) / float64(maxOffset) * float64(maxTop))
		if thumbTop < 0 {
			thumbTop = 0
		}
		if thumbTop > maxTop {
			thumbTop = maxTop
		}
	}
	var b strings.Builder
	for i := 0; i < h; i++ {
		if i >= thumbTop && i < thumbTop+thumbSize && totalLines > visible {
			b.WriteString(m.styles.ThinkingStyle.Render("█"))
		} else {
			b.WriteString(m.styles.DividerStyle.Render("│"))
		}
		if i < h-1 {
			b.WriteString("\n")
		}
	}
	// Asegurar ancho fijo de la columna
	return lipgloss.NewStyle().Width(m.scrollbarWidth).Render(b.String())
}

func (m *appModel) handleModelsMenuCommand(input string) string {
	if input == "" {
		return ""
	}
	fields := strings.Fields(input)
	if len(fields) == 0 {
		return ""
	}
	switch strings.ToLower(fields[0]) {
	case "help", "/help":
		return "Usa: list | catalog [prov] | activate <prov> <model> | setkey <prov> <key> [ENV] | add <prov> <model> [ENV]"
	case "list":
		return m.cmdListProviders()
	case "catalog":
		prov := ""
		if len(fields) > 1 {
			prov = fields[1]
		}
		return m.cmdCatalog(prov)
	case "activate", "switch", "use":
		if len(fields) < 3 {
			return "Uso: activate <proveedor> <modelo>"
		}
		prov := fields[1]
		model := fields[2]
		if err := m.cmdActivate(prov, model); err != nil {
			return fmt.Sprintf("Error activando: %v", err)
		}
		return fmt.Sprintf("Activado %s / %s", prov, model)
	case "setkey":
		if len(fields) < 3 {
			return "Uso: setkey <proveedor> <clave> [ENV]"
		}
		prov := fields[1]
		key := fields[2]
		env := ""
		if len(fields) >= 4 {
			env = fields[3]
		}
		if out, err := m.cmdSetKey(prov, key, env); err != nil {
			return fmt.Sprintf("Error setkey: %v", err)
		} else {
			return out
		}
	case "add", "addmodel":
		if len(fields) < 3 {
			return "Uso: add <proveedor> <modelo> [ENV]"
		}
		prov := fields[1]
		model := fields[2]
		env := ""
		if len(fields) >= 4 {
			env = fields[3]
		}
		if err := m.cmdAddProvider(prov, model, env); err != nil {
			return fmt.Sprintf("Error agregando proveedor: %v", err)
		}
		return fmt.Sprintf("Agregado proveedor '%s' con modelo '%s'", prov, model)
	default:
		return "Comando no reconocido. Usa 'help' para ver opciones."
	}
}

func (m *appModel) cmdListProviders() string {
	if m.config == nil {
		return "Config no cargada."
	}
	var lines []string
	keys := make([]string, 0, len(m.config.Providers))
	for k := range m.config.Providers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		p := m.config.Providers[k]
		active := ""
		if m.config.ActiveLLM == k {
			active = " (activo)"
		}
		lines = append(lines, fmt.Sprintf("- %s: modelo=%s, env=%s%s", k, p.Model, p.APIKeyEnv, active))
	}
	if len(lines) == 0 {
		return "No hay proveedores configurados. Usa 'add <prov> <modelo>' o 'activate'."
	}
	return strings.Join(lines, "\n")
}

func (m *appModel) cmdCatalog(providerOpt string) string {
	cat := loadProviderCatalog()
	if providerOpt == "" {
		// listar proveedores
		var provs []string
		for id, p := range cat.Providers {
			provs = append(provs, fmt.Sprintf("%s (%s) [env=%s]", id, p.Name, p.EnvVar))
		}
		sort.Strings(provs)
		return "Proveedores disponibles:\n" + strings.Join(provs, "\n")
	}
	p, ok := cat.Providers[strings.ToLower(providerOpt)]
	if !ok {
		return fmt.Sprintf("Proveedor '%s' no encontrado en catálogo.", providerOpt)
	}
	var ms []string
	for _, m := range p.Models {
		ms = append(ms, "- "+m)
	}
	return fmt.Sprintf("Modelos de %s (%s):\n%s", p.ID, p.Name, strings.Join(ms, "\n"))
}

func (m *appModel) cmdActivate(provider string, model string) error {
	provider = strings.ToLower(provider)
	if m.config == nil {
		return fmt.Errorf("config no cargada")
	}
	p, ok := m.config.Providers[provider]
	if !ok {
		// crear proveedor con env heurístico
		p = config.LLMProvider{
			Name:      strings.Title(provider),
			Model:     model,
			APIKeyEnv: envVarForProvider(provider),
		}
		m.config.Providers[provider] = p
	} else {
		p.Model = model
		m.config.Providers[provider] = p
	}
	m.config.ActiveLLM = provider
	if err := config.SaveConfig(m.config); err != nil {
		return err
	}
	// refrescar apiKey en memoria
	m.apiKey, _ = m.config.GetActiveAPIKey()
	return nil
}

func (m *appModel) cmdSetKey(provider string, keyValue string, envOverride string) (string, error) {
	provider = strings.ToLower(provider)
	if m.config == nil {
		return "", fmt.Errorf("config no cargada")
	}
	env := envOverride
	if env == "" {
		if p, ok := m.config.Providers[provider]; ok && p.APIKeyEnv != "" {
			env = p.APIKeyEnv
		} else {
			env = envVarForProvider(provider)
		}
	}
	if env == "" {
		return "", fmt.Errorf("no se pudo determinar la variable de entorno para %s; especifica como tercer parámetro")
	}
	if err := setEnvPersist(env, keyValue); err != nil {
		return "", err
	}
	// actualizar config
	p, ok := m.config.Providers[provider]
	if !ok {
		p = config.LLMProvider{Name: strings.Title(provider)}
	}
	p.APIKeyEnv = env
	m.config.Providers[provider] = p
	if err := config.SaveConfig(m.config); err != nil {
		return "", err
	}
	// refrescar apiKey si es activo
	if m.config.ActiveLLM == provider {
		m.apiKey, _ = m.config.GetActiveAPIKey()
	}
	return fmt.Sprintf("API key almacenada en '%s' y configuración actualizada para '%s'", env, provider), nil
}

func (m *appModel) cmdAddProvider(provider string, model string, env string) error {
	provider = strings.ToLower(provider)
	if m.config == nil {
		return fmt.Errorf("config no cargada")
	}
	if env == "" {
		env = envVarForProvider(provider)
	}
	m.config.Providers[provider] = config.LLMProvider{
		Name:      strings.Title(provider),
		Model:     model,
		APIKeyEnv: env,
	}
	return config.SaveConfig(m.config)
}

func envVarForProvider(provider string) string {
	switch strings.ToLower(provider) {
	case "gemini", "google-gemini", "google":
		return "GEMINI_API_KEY"
	case "openai":
		return "OPENAI_API_KEY"
	case "anthropic":
		return "ANTHROPIC_API_KEY"
	case "cohere":
		return "COHERE_API_KEY"
	default:
		return ""
	}
}

func setEnvPersist(env string, value string) error {
	// actualiza proceso actual
	_ = os.Setenv(env, value)
	// persistencia por OS
	if runtime.GOOS == "windows" {
		cmd := exec.Command("powershell", "-NoProfile", "-Command", fmt.Sprintf("setx %s \"%s\"", env, value))
		cmd.Stdout = nil
		cmd.Stderr = nil
		return cmd.Run()
	}
	// otros sistemas: persistir en archivo de configuración
	return config.SaveEnvVar(env, value)
}

func loadProviderCatalog() providerCatalog {
	// intentar en el directorio del ejecutable, luego en el cwd y padres
	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)
	candidates := []string{
		filepath.Join(exeDir, "providers.json"),
		"providers.json",
		filepath.Join("..", "providers.json"),
		filepath.Join("..", "..", "providers.json"),
	}
	for _, p := range candidates {
		if b, err := os.ReadFile(p); err == nil {
			if cat, ok := parseProvidersJSON(b); ok {
				return cat
			}
		}
	}
	return defaultProviderCatalog()
}

func parseProvidersJSON(b []byte) (providerCatalog, bool) {
	out := providerCatalog{Providers: make(map[string]catalogProvider)}

	process := func(items []interface{}) {
		for _, it := range items {
			m, ok := it.(map[string]interface{})
			if !ok {
				continue
			}
			id := getStr(m, "id")
			if id == "" {
				id = strings.ToLower(getStr(m, "provider"))
			}
			if id == "" {
				id = strings.ToLower(getStr(m, "slug"))
			}
			name := getStr(m, "name")
			// env puede venir como api_key_env, env o api_key ("$ENV")
			env := getStr(m, "api_key_env")
			if env == "" {
				env = getStr(m, "env")
			}
			if env == "" {
				apiKey := getStr(m, "api_key")
				if strings.HasPrefix(apiKey, "$") {
					env = strings.TrimPrefix(apiKey, "$")
				} else {
					env = apiKey
				}
			}
			var models []string
			if arr, ok := m["models"].([]interface{}); ok {
				for _, mm := range arr {
					switch vv := mm.(type) {
					case string:
						models = append(models, vv)
					case map[string]interface{}:
						mid := getStr(vv, "id")
						if mid == "" {
							mid = getStr(vv, "model_id")
						}
						if mid == "" {
							mid = getStr(vv, "name")
						}
						if mid != "" {
							models = append(models, mid)
						}
					}
				}
			}
			if id == "" || len(models) == 0 {
				// saltar entradas sin id o modelos
				continue
			}
			out.Providers[strings.ToLower(id)] = catalogProvider{ID: strings.ToLower(id), Name: name, EnvVar: env, Models: models}
		}
	}

	// Caso 1: array raíz
	var arr []interface{}
	if err := json.Unmarshal(b, &arr); err == nil && len(arr) > 0 {
		process(arr)
		if len(out.Providers) > 0 {
			return out, true
		}
	}
	// Caso 2: objeto raíz con clave providers/catalog
	var root map[string]interface{}
	if err := json.Unmarshal(b, &root); err == nil {
		if pvRaw, ok := root["providers"].([]interface{}); ok {
			process(pvRaw)
		}
		if pvRaw, ok := root["catalog"].([]interface{}); ok {
			process(pvRaw)
		}
		if len(out.Providers) > 0 {
			return out, true
		}
	}
	return providerCatalog{}, false
}

func getStr(m map[string]interface{}, k string) string {
	if v, ok := m[k]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func defaultProviderCatalog() providerCatalog {
	return providerCatalog{Providers: map[string]catalogProvider{
		"google-gemini": {
			ID:     "google-gemini",
			Name:   "Google Gemini",
			EnvVar: "GEMINI_API_KEY",
			Models: []string{
				"gemini-1.5-pro",
				"gemini-1.5-flash",
				"gemini-2.0-flash",
				"gemini-2.5-flash",
				"gemini-2.5-pro",
			},
		},
		"openai": {
			ID:     "openai",
			Name:   "OpenAI",
			EnvVar: "OPENAI_API_KEY",
			Models: []string{
				"gpt-4o-mini",
				"gpt-4o",
				"gpt-4.1",
				"o3-mini",
			},
		},
		"anthropic": {
			ID:     "anthropic",
			Name:   "Anthropic Claude",
			EnvVar: "ANTHROPIC_API_KEY",
			Models: []string{
				"claude-3-haiku",
				"claude-3-sonnet",
				"claude-3-opus",
				"claude-3.5-sonnet",
			},
		},
		"cohere": {
			ID:     "cohere",
			Name:   "Cohere",
			EnvVar: "COHERE_API_KEY",
			Models: []string{
				"command-r",
				"command-r-plus",
			},
		},
	}}
}
